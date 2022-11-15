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
	"fmt"
	"io"
	"net/http"
	"regexp"

	"github.com/sustainable-computing-io/kepler/pkg/config"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	// metrics obtained from the kepler exporter:
	// curl http://localhost:9102/metrics |awk -F"\{" '{print $1}' | grep kepler_ |grep -v \# |sort  |uniq  |xargs -I {} echo \"{}\",
	//TODO: uncomment the following lines after the metrics are implemented
	keplerMetrics = []string{
		"kepler_container_core_joules_total",
		//"kepler_container_cpu_cpu_time_us",
		"kepler_container_dram_joules_total",
		"kepler_container_gpu_joules_total",
		"kepler_container_joules_total",
		"kepler_container_other_host_components_joules_total",
		"kepler_container_package_joules_total",
		"kepler_container_uncore_joules_total",
		"kepler_exporter_build_info",
		//"kepler_node_core_joules_total",
		//"kepler_node_cpu_scaling_frequency_hertz",
		//"kepler_node_dram_joules_total",
		"kepler_node_energy_stat",
		"kepler_node_nodeInfo",
		"kepler_node_other_host_components_joules_total",
		//"kepler_node_package_energy_millijoule",
		//"kepler_node_package_joules_total",
		"kepler_node_platform_joules_total",
		//"kepler_node_uncore_joules_total",
		"kepler_pod_energy_stat",
	}
)
var _ = Describe("Check metrics", func() {
	It("Check kepler metrics", func() {
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
		content := string(body)
		cgroupVer := config.GetCGroupVersion()
		for _, metric := range keplerMetrics {
			reStr := metric + "{.*} \\d*\\.?\\d*" // metric{.*}\d*\.?\d*
			re := regexp.MustCompile(reStr)
			str := re.FindString(content)
			if str == "" && cgroupVer == 2 {
				msg := fmt.Sprintf("metric %s not found; cgroup version %v; content:\n%v", metric, cgroupVer, content)
				Fail(msg)
			}
		}
	})
})
