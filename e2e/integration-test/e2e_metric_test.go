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

package integrationtest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/textparse"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type TestKeplerMetric struct {
	Metric   map[string][]float64
	PodLists []string
	Client   *kubernetes.Clientset
}

func NewTestKeplerMetric(kubeconfigPath string) (*TestKeplerMetric, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &TestKeplerMetric{
		Metric:   make(map[string][]float64),
		PodLists: make([]string, 0),
		Client:   clientset,
	}, nil
}

func (kmc *TestKeplerMetric) UpdateMetricMap(key string, value float64) {
	if _, ok := kmc.Metric[key]; !ok {
		kmc.Metric[key] = []float64{}
	}
	kmc.Metric[key] = append(kmc.Metric[key], value)
}

func (kmc *TestKeplerMetric) RetrivePodNames(ctx context.Context) error {
	namespaces, err := kmc.Client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for i := range namespaces.Items {
		ns := &namespaces.Items[i]
		pods, err := kmc.Client.CoreV1().Pods(ns.Name).List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		for j := range pods.Items {
			pod := &pods.Items[j]
			kmc.PodLists = append(kmc.PodLists, pod.Name)
		}
	}
	return nil
}

func getEnvOrDefault(envName, defaultValue string) string {
	value, exists := os.LookupEnv(envName)
	if !exists {
		return defaultValue
	}
	return value
}

var _ = Describe("Metrics check should pass", Ordered, func() {

	var keplerMetric *TestKeplerMetric

	_ = BeforeAll(func() {
		var err error
		kubeconfigPath := getEnvOrDefault("KUBECONFIG", "/tmp/.kube/config")
		keplerMetric, err = NewTestKeplerMetric(kubeconfigPath)
		Expect(err).NotTo(HaveOccurred())

		ctx := context.Background()
		err = keplerMetric.RetrivePodNames(ctx)
		Expect(err).NotTo(HaveOccurred())

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
					keplerMetric.UpdateMetricMap(res.Get("__name__")+" @ "+res.Get("pod_name"), v)
				} else {
					keplerMetric.UpdateMetricMap(res.Get("__name__"), v)
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
		for k, v := range keplerMetric.Metric {
			fmt.Printf("metric: %s, value: %v\n", k, v)
		}
	})

	_ = DescribeTable("Check node level metrics for details",
		func(metrics string) {
			v, ok := keplerMetric.Metric[metrics]
			Expect(ok).To(BeTrue())
			nonzeroFound := false
			for _, val := range v {
				if val > 0 {
					nonzeroFound = true
					break
				}
			}
			if !nonzeroFound {
				Skip("Skip as " + metrics + " is zero")
			}
			Expect(nonzeroFound).To(BeTrue())

			// TODO: check value in details base on cgroup and gpu etc...
			// so far just base check as compare with zero by default
		},
		EntryDescription("Checking %s"),
		Entry(nil, "kepler_exporter_build_info"),        // only one
		Entry(nil, "kepler_node_core_joules_total"),     // node level check by instance
		Entry(nil, "kepler_node_dram_joules_total"),     // node level check by instance
		Entry(nil, "kepler_node_info"),                  // node level missing labels
		Entry(nil, "kepler_node_package_joules_total"),  // node levelcheck by instance
		Entry(nil, "kepler_node_platform_joules_total"), // node levelcheck by instance
		Entry(nil, "kepler_node_uncore_joules_total"),   // node levelcheck by instance
	)

	_ = DescribeTable("Check pod level metrics for details",
		func(metrics string) {
			nonzeroFound := false
			var value float64
			for _, podname := range keplerMetric.PodLists {
				v, ok := keplerMetric.Metric[metrics+" @ "+podname]
				Expect(ok).To(BeTrue())
				for _, val := range v {
					if val > 0 {
						nonzeroFound = true
						value = val
						break
					}
				}
				if !nonzeroFound {
					fmt.Printf("Skip as %s for %s is zero\n", metrics, podname)
				} else {
					break
				}
				// TODO: check value in details base on cgroup and gpu etc...
				// so far just base check as compare with zero by default
			}
			if !nonzeroFound {
				Skip("skip as " + metrics + " for all pods are zero")
			}
			Expect(value).To(BeNumerically(">", 0))
		},
		EntryDescription("Checking %s"),
		Entry(nil, "kepler_container_core_joules_total"),    // pod level
		Entry(nil, "kepler_container_dram_joules_total"),    // pod level
		Entry(nil, "kepler_container_joules_total"),         // pod level
		Entry(nil, "kepler_container_package_joules_total"), // pod level
		Entry(nil, "kepler_container_uncore_joules_total"),  // pod level
	)
})
