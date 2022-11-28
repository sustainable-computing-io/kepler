//go:build bcc
// +build bcc

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
	"errors"
	"io"
	"net/http"

	"github.com/prometheus/prometheus/model/textparse"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var k_metric map[string]float64

var _ = Describe("metrics check should pass", Ordered, func() {
	var _ = BeforeAll(func() {
		k_metric = make(map[string]float64)
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
		p := textparse.NewPromParser(body)
		for {
			et, err := p.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			switch et {
			case textparse.EntrySeries:
				m, _, v := p.Series()
				k_metric[string(m)] = v
			case textparse.EntryType:
				m, _ := p.Type()
				k_metric[string(m)] = 0
			case textparse.EntryHelp:
				m, _ := p.Help()
				k_metric[string(m)] = 0
			}
		}
	})
	DescribeTable("Check metrics for details",
		func(metrics string) {
			v, ok := k_metric[metrics]
			Expect(ok).To(BeTrue())
			// TODO: check value in details base on cgroup and gpu etc...
			// so far just base check as compare with zero by default
			Expect(v).To(BeNumerically(">=", 0))
		},
		EntryDescription("checking %s"),
		Entry(nil, "kepler_container_core_joules_total"),
		Entry(nil, "kepler_container_dram_joules_total"),
		Entry(nil, "kepler_container_gpu_joules_total"),
		Entry(nil, "kepler_container_joules_total"),
		Entry(nil, "kepler_container_other_host_components_joules_total"),
		Entry(nil, "kepler_container_package_joules_total"),
		Entry(nil, "kepler_container_uncore_joules_total"),
		Entry(nil, "kepler_exporter_build_info"),
		Entry(nil, "kepler_node_core_joules_total"),
		//"kepler_node_cpu_scaling_frequency_hertz",
		Entry(nil, "kepler_node_dram_joules_total"),
		Entry(nil, "kepler_node_energy_stat"),
		Entry(nil, "kepler_node_nodeInfo"),
		Entry(nil, "kepler_node_other_host_components_joules_total"),
		Entry(nil, "kepler_node_package_energy_millijoule"),
		Entry(nil, "kepler_node_package_joules_total"),
		Entry(nil, "kepler_node_platform_joules_total"),
		Entry(nil, "kepler_node_uncore_joules_total"),
		Entry(nil, "kepler_pod_energy_stat"),
	)

})
