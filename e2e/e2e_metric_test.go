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

var kMetric map[string]float64
var podlists []string
var pods *v1.PodList

var _ = Describe("metrics check should pass", Ordered, func() {
	var _ = BeforeAll(func() {
		kMetric = make(map[string]float64)
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
		for {
			et, err := p.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			switch et {
			case textparse.EntrySeries:
				m, _, v := p.Series()
				p.Metric(&res)
				if res.Has("pod_name") {
					kMetric[res.Get("__name__")+res.Get("pod_name")] = v
				} else {
					kMetric[string(m)] = v
				}
				res = res[:0]
			case textparse.EntryType:
				m, _ := p.Type()
				kMetric[string(m)] = 0
			case textparse.EntryHelp:
				m, _ := p.Help()
				kMetric[string(m)] = 0
			}
		}
	})

	var _ = DescribeTable("Check node level metrics for details",
		func(metrics string) {
			v, ok := kMetric[metrics]
			Expect(ok).To(BeTrue())
			// TODO: check value in details base on cgroup and gpu etc...
			// so far just base check as compare with zero by default
			if v == 0 {
				Skip("skip as " + metrics + " is zero")
			}
			Expect(v).To(BeNumerically(">", 0))
		},
		EntryDescription("checking %s"),
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
			for _, podname := range podlists {
				v, ok := kMetric[metrics+podname]
				Expect(ok).To(BeTrue())
				// TODO: check value in details base on cgroup and gpu etc...
				// so far just base check as compare with zero by default
				if v == 0 {
					Skip("skip as " + metrics + " for " + podname + " is zero")
				}
				Expect(v).To(BeNumerically(">", 0))
			}
		},
		EntryDescription("checking %s"),
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
