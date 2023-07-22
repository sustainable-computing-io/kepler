//go:build bcc || libbpf
// +build bcc libbpf

/*
Copyright 2022.

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

package e2e_test

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/textparse"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var kMetric map[string][]float64
var podlists []string
var pods *v1.PodList

func updateMetricMap(key string, value float64) {
	_, ok := kMetric[key]
	if !ok {
		kMetric[key] = make([]float64, 0)
	}
	kMetric[key] = append(kMetric[key], value)
}

var _ = Describe("Metrics check should pass", Ordered, func() {
	var _ = BeforeAll(func() {
		kMetric = make(map[string][]float64)
		podlists = make([]string, 0)

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
			for _, pod := range pods.Items {
				podlists = append(podlists, pod.Name)
			}
		}
		reader := bytes.NewReader([]byte{})
		req, err := http.NewRequest("GET", "http://"+address+"/metrics", reader)
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
				fmt.Printf("error: %v\n", err)
				break
			}
			switch et {
			case textparse.EntrySeries:
				m, _, v := p.Series()
				p.Metric(&res)
				if res.Has("pod_name") {
					updateMetricMap(res.Get("__name__")+" @ "+res.Get("pod_name"), v)
				} else {
					updateMetricMap(res.Get("__name__"), v)
				}
				fmt.Printf("metric(with lables): %s\nvalue: %f\n", m, v)
				res = res[:0]
			case textparse.EntryType:
				_, t := p.Type()
				fmt.Printf("type: %s\n", t)
			case textparse.EntryHelp:
				m, h := p.Help()
				fmt.Println("\n------------------------------------------")
				fmt.Printf("metric: %s\nhelp: %s\n", m, h)
			}
		}
		fmt.Println("=============================================")
		fmt.Println("Dump saved metrics...")
		for k, v := range kMetric {
			fmt.Printf("metric: %s, value: %v\n", k, v)
		}
	})

	var _ = DescribeTable("Check node level metrics for details",
		func(metrics string) {
			v, ok := kMetric[metrics]
			Expect(ok).To(BeTrue())
			nonzero_found := false
			for _, val := range v {
				if val > 0 {
					nonzero_found = true
					break
				}
			}
			if !nonzero_found {
				Skip("Skip as " + metrics + " is zero")
			}
			Expect(nonzero_found).To(BeTrue())

			// TODO: check value in details base on cgroup and gpu etc...
			// so far just base check as compare with zero by default
		},
		EntryDescription("Checking %s"),
		Entry(nil, "kepler_exporter_build_info"),                     // only one
		Entry(nil, "kepler_node_core_joules_total"),                  // node level check by instance
		Entry(nil, "kepler_node_dram_joules_total"),                  // node level check by instance
		Entry(nil, "kepler_node_energy_stat"),                        // node level missing instance label but node_name
		Entry(nil, "kepler_node_info"),                               // node level missing labels
		Entry(nil, "kepler_node_other_host_components_joules_total"), // node level check by instance
		Entry(nil, "kepler_node_package_energy_millijoule"),          // node level missing instance label
		Entry(nil, "kepler_node_package_joules_total"),               // node levelcheck by instance
		Entry(nil, "kepler_node_platform_joules_total"),              // node levelcheck by instance
		Entry(nil, "kepler_node_uncore_joules_total"),                // node levelcheck by instance
	)

	var _ = DescribeTable("Check pod level metrics for details",
		func(metrics string) {
			nonzero_found := false
			var value float64
			for _, podname := range podlists {
				v, ok := kMetric[metrics+" @ "+podname]
				Expect(ok).To(BeTrue())
				for _, val := range v {
					if val > 0 {
						nonzero_found = true
						value = val
						break
					}
				}
				if !nonzero_found {
					fmt.Printf("Skip as %s for %s is zero\n", metrics, podname)
				} else {
					break
				}
				// TODO: check value in details base on cgroup and gpu etc...
				// so far just base check as compare with zero by default
			}
			if !nonzero_found {
				Skip("skip as " + metrics + " for all pods are zero")
			}
			Expect(value).To(BeNumerically(">", 0))
		},
		EntryDescription("Checking %s"),
		Entry(nil, "kepler_container_core_joules_total"),                  // pod level
		Entry(nil, "kepler_container_dram_joules_total"),                  // pod level
		Entry(nil, "kepler_container_joules_total"),                       // pod level
		Entry(nil, "kepler_container_other_host_components_joules_total"), // pod level
		Entry(nil, "kepler_container_package_joules_total"),               // pod level
		Entry(nil, "kepler_container_uncore_joules_total"),                // pod level
		Entry(nil, "kepler_container_kubelet_cpu_usage_total"),            // pod level
		Entry(nil, "kepler_container_kubelet_memory_bytes_total"),         // pod level
		Entry(nil, "kepler_pod_energy_stat"),                              // pod level
	)
})
