//go:build bcc || libbpf
// +build bcc libbpf

/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package platform_validation_test

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/textparse"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	kMetric    map[string][]float64
	podlistMap map[string][]string //key:namespace, value:pod list
	pods       *corev1.PodList
	cpu_arch   string
	client     api.Client
	v1api      promv1.API
	queryRange promv1.Range
)

func updateMetricMap(key string, value float64) {
	_, ok := kMetric[key]
	if !ok {
		kMetric[key] = make([]float64, 0)
	}
	kMetric[key] = append(kMetric[key], value)
}

func queryNodePower(m string, r promv1.Range, api promv1.API) float64 {
	if !strings.Contains(m, "node") {
		fmt.Printf("Invalid metric for node power: %s\n", m)
		return float64(0)
	}
	q := "sum(irate(" + m + "[1m]))"
	return query_range(m, q, r, api)
}

func queryNamespacePower(m, ns string, r promv1.Range, api promv1.API) float64 {
	if !strings.Contains(m, "container") {
		fmt.Printf("Invalid metric for namespace power: %s\n", m)
		return float64(0)
	}
	q := "sum(irate(" + m + "{container_namespace=~\"" + ns + "\"}[1m]))"
	return query_range(m, q, r, api)
}

func queryPodPower(m, p, ns string, r promv1.Range, api promv1.API) float64 {
	if !strings.Contains(m, "container") {
		fmt.Printf("Invalid metric for pod power: %s\n", m)
		return float64(0)
	}
	q := `sum by(pod_name,container_namespace)(irate(` + m + `{container_namespace=~"` + ns + `", pod_name=~"` + p + `"}[1m]))`
	return query_range(m, q, r, api)
}

func query_range(metric, queryString string, r promv1.Range, api promv1.API) float64 {
	fmt.Printf("\nQuery: \n%s\n", queryString)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, warnings, err := api.QueryRange(ctx, queryString, r, promv1.WithTimeout(5*time.Second))
	if err != nil {
		fmt.Printf("Error querying Prometheus: %v\n", err)
		return float64(0)
	}
	if len(warnings) > 0 {
		fmt.Printf("Warnings: %v\n", warnings)
	}
	tmpList := strings.Split(result.String(), "=>")
	if len(tmpList) == 0 {
		fmt.Println("Warning: no result in Prometheus for this query.")
		return float64(0)
	}
	samplePairList := strings.Split(tmpList[1], "\n")
	sampleValList := make([]float64, 0)
	var valString string
	var val float64
	for _, pair := range samplePairList {
		if pair != "" {
			valString = strings.TrimSpace(strings.Split(pair, "@")[0])
			val, _ = strconv.ParseFloat(valString, 64)
			sampleValList = append(sampleValList, val)
		}
	}
	sum := float64(0)
	for _, v := range sampleValList {
		sum += v
	}
	meanPower := sum / float64(len(sampleValList))
	fmt.Printf("\nResult: %f\n", meanPower)

	return meanPower
}

var _ = Describe("Kepler exporter side metrics check", Ordered, func() {
	_ = BeforeAll(func() {
		kMetric = make(map[string][]float64)
		podlistMap = make(map[string][]string)
		// Step 1: get K8S topo
		kubeconfig := flag.String("kubeconfig", "/tmp/.kube/config", "location to your kubeconfig file")
		config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
		Expect(err).NotTo(HaveOccurred())
		clientset, err := kubernetes.NewForConfig(config)
		Expect(err).NotTo(HaveOccurred())
		ctx := context.Background()
		namespaces, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())
		for _, ns := range namespaces.Items {
			pods, err = clientset.CoreV1().Pods(ns.Name).List(ctx, metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			podlistMap[ns.Name] = make([]string, 0)
			for _, pod := range pods.Items {
				podlistMap[ns.Name] = append(podlistMap[ns.Name], pod.Name)
			}
		}
		fmt.Println("=============================================")
		fmt.Println("Dump pod list...")
		for ns, pl := range podlistMap {
			fmt.Printf("\n--- namespace: %s ---\n", ns)
			for _, p := range pl {
				fmt.Println(p)
			}
		}
		// Step 2: parse Kepler metrics, generate keplerMetrics map
		reader := bytes.NewReader([]byte{})
		req, err := http.NewRequest("GET", "http://"+keplerAddr+"/metrics", reader)
		Expect(err).NotTo(HaveOccurred())
		req.Header.Set("Accept", "application/json")
		resp, err := http.DefaultClient.Do(req)
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		body, err := io.ReadAll(resp.Body)
		Expect(err).NotTo(HaveOccurred())
		// ref https://github.com/prometheus/prometheus/blob/main/model/textparse/promparse_test.go
		var res labels.Labels
		p := textparse.NewPromParser(body)
		fmt.Println("=============================================")
		fmt.Println("Parsing Metrics...")
		for {
			et, err := p.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			switch et {
			case textparse.EntrySeries:
				_, _, v := p.Series()
				p.Metric(&res)
				if res.Has("pod_name") {
					updateMetricMap(res.Get("__name__")+" @ "+res.Get("pod_name"), v)
				} else {
					if res.Has("cpu_architecture") {
						cpu_arch = res.Get("cpu_architecture")
					}
					updateMetricMap(res.Get("__name__"), v)
				}
				res = res[:0]
			case textparse.EntryType:
			case textparse.EntryHelp:
			}
		}
		fmt.Println("=============================================")
		fmt.Println("Dump saved metrics...")
		for k, v := range kMetric {
			fmt.Printf("metric: %s, value: %v\n", k, v)
		}
		// Step 3: Initialize Prometheus client and default query range.
		client, err = api.NewClient(api.Config{
			Address: "http://" + promAddr,
		})
		Expect(err).NotTo(HaveOccurred())

		v1api = promv1.NewAPI(client)

		queryRange = promv1.Range{
			Start: time.Now().Add(-5 * time.Minute),
			End:   time.Now(),
			Step:  15 * time.Second,
		}
	})

	_ = Describe("Case 1: CPU architecture check.", func() {
		It("Kepler can export correct cpu_architecture", func() {
			Expect(cpu_arch).To(Equal(cpuArch))
		})
	})

	_ = DescribeTable("Case 2-1: Check node level metrics for platform specific power source component",
		func(metrics string, enable *bool) {
			v, ok := kMetric[metrics]
			Expect(ok).To(BeTrue())
			nonzero_found := false
			for _, val := range v {
				if val > 0 {
					nonzero_found = true
					break
				}
			}
			if *enable {
				Expect(nonzero_found).To(BeTrue())
			} else {
				Expect(nonzero_found).To(BeFalse())
			}
		},
		EntryDescription("Checking %s"),
		Entry(nil, "kepler_node_core_joules_total", &raplCoreEnable),
		Entry(nil, "kepler_node_dram_joules_total", &raplDramEnable),
		Entry(nil, "kepler_node_package_joules_total", &raplPkgEnable),
		Entry(nil, "kepler_node_uncore_joules_total", &raplUncoreEnable),
	)

	_ = DescribeTable("Case 2-2: Check pod level metrics for platform specific power source component",
		func(metrics string, enable *bool) {
			nonzero_found := false
		outer:
			for _, podlist := range podlistMap {
				for _, pod := range podlist {
					v, ok := kMetric[metrics+" @ "+pod]
					if !ok {
						continue
					}
					for _, val := range v {
						if val > 0 {
							nonzero_found = true
							break outer
						}
					}
				}
			}
			if *enable {
				Expect(nonzero_found).To(BeTrue())
			} else {
				Expect(nonzero_found).To(BeFalse())
			}
		},
		EntryDescription("Checking %s"),
		Entry(nil, "kepler_container_core_joules_total", &raplCoreEnable),
		Entry(nil, "kepler_container_dram_joules_total", &raplDramEnable),
		Entry(nil, "kepler_container_package_joules_total", &raplPkgEnable),
		Entry(nil, "kepler_container_uncore_joules_total", &raplUncoreEnable),
	)

	_ = DescribeTable("Case 3: Compare Prometheus side node level components power stats with validator measured data",
		func(metrics string, enable *bool) {
			if !*enable {
				Skip("Skip as " + metrics + " should not be supported in this platform")
			}
			component := strings.Split(metrics[12:], "_")[0]
			var validatorData float64
			switch component {
			case "core":
				validatorData = testPowerData[2].corePower
			case "dram":
				validatorData = testPowerData[2].dramPower
			case "package":
				validatorData = testPowerData[2].pkgPower
			case "uncore":
				validatorData = testPowerData[2].uncorePower
			default:
				Skip("Skip as " + metrics + " is not supported in this test case")
			}
			promData := queryNodePower(metrics, queryRange, v1api)
			fmt.Printf("For metric: %s\n", metrics)
			fmt.Printf("Validator measured node power is: %f\n", validatorData)
			fmt.Printf("Prometheus queried node power is: %f\n", promData)
		},
		EntryDescription("Checking %s"),
		Entry(nil, "kepler_node_core_joules_total", &raplCoreEnable),
		Entry(nil, "kepler_node_dram_joules_total", &raplDramEnable),
		Entry(nil, "kepler_node_package_joules_total", &raplPkgEnable),
		Entry(nil, "kepler_node_uncore_joules_total", &raplUncoreEnable),
	)

	_ = DescribeTable("Case 4: Compare Prometheus side namespace level components power stats with validator measured data",
		func(metrics string, enable *bool) {
			if !*enable {
				Skip("Skip as " + metrics + " should not be supported in this platform")
			}
			component := strings.Split(metrics[17:], "_")[0]
			kind_namespaces := []string{
				"kube-system",
				"monitoring",
				"local-path-storage",
			}
			kepler_namespace := "kepler"
			// d1: validator measured power for Kind
			// d2: validator measured power for Kepler
			var d1, d2 float64
			// p1: prometheus queried power for Kind
			// p2: prometheus queried power for Kepler
			var p1, p2 float64

			switch component {
			case "core":
				d1 = testPowerData[1].corePower - testPowerData[0].corePower
				d2 = testPowerData[2].corePower - testPowerData[1].corePower
			case "dram":
				d1 = testPowerData[1].dramPower - testPowerData[0].dramPower
				d2 = testPowerData[2].dramPower - testPowerData[1].dramPower
			case "package":
				d1 = testPowerData[1].pkgPower - testPowerData[0].pkgPower
				d2 = testPowerData[2].pkgPower - testPowerData[1].pkgPower
			case "uncore":
				d1 = testPowerData[1].uncorePower - testPowerData[0].uncorePower
				d2 = testPowerData[2].uncorePower - testPowerData[1].uncorePower
			default:
				Skip("Skip as " + metrics + " is not supported in this test case")
			}
			for _, n := range kind_namespaces {
				nsPower := queryNamespacePower(metrics, n, queryRange, v1api)
				p1 += nsPower
			}
			p2 = queryNamespacePower(metrics, kepler_namespace, queryRange, v1api)

			fmt.Printf("For metric: %s\n", metrics)
			fmt.Printf("Validator measured kind power(postDeploy - preDeploy) is: %f\n", d1)
			fmt.Printf("Prometheus queried kind power(related namespaces power sum) is: %f\n", p1)
			fmt.Printf("Validator measured kepler power(postDeploy - preDeploy) is: %f\n", d2)
			fmt.Printf("Prometheus queried kepler power(related namespaces power sum) is: %f\n", p2)
		},
		EntryDescription("Checking %s"),
		Entry(nil, "kepler_container_core_joules_total", &raplCoreEnable),
		Entry(nil, "kepler_container_dram_joules_total", &raplDramEnable),
		Entry(nil, "kepler_container_package_joules_total", &raplPkgEnable),
		Entry(nil, "kepler_container_uncore_joules_total", &raplUncoreEnable),
	)
})
