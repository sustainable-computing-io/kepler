/*
Copyright 2021.

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

package collector

import (
	"fmt"
	"strconv"

	"github.com/sustainable-computing-io/kepler/pkg/attacher"

	"github.com/prometheus/client_golang/prometheus"
)

type Collector struct {
	modules *attacher.BpfModuleTables
}

func New() (*Collector, error) {
	return &Collector{}, nil
}

func (c *Collector) Attach() error {
	m, err := attacher.AttachBPFAssets()
	if err != nil {
		return fmt.Errorf("failed to attach bpf assets: %v", err)
	}
	c.modules = m
	c.reader()
	return nil
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	lock.Lock()
	defer lock.Unlock()
	for k, _ := range podEnergy {
		desc := prometheus.NewDesc(
			"pod_stat_"+k,
			"Pod energy consumption status",
			[]string{
				"pod_name",
				"pod_namespace",
				"command",
				"last_cpu_time",
				"curr_cpu_time",
				"last_cpu_cycles",
				"curr_cpu_cycles",
				"last_cpu_instructions",
				"curr_cpu_instructions",
				"last_cache_misses",
				"curr_cache_misses",
				"last_energy_in_core",
				"curr_energy_in_core",
				"last_energy_in_dram",
				"curr_energy_in_dram",
			},
			nil,
		)
		ch <- desc
	}
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	lock.Lock()
	defer lock.Unlock()
	for _, v := range podEnergy {
		de := prometheus.NewDesc(
			"pod_energy_stat",
			"Pod energy consumption stats",
			[]string{
				"pod_name",
				"pod_namespace",
				"command",
				"last_cpu_time",
				"curr_cpu_time",
				"last_cpu_cycles",
				"curr_cpu_cycles",
				"last_cpu_instructions",
				"curr_cpu_instructions",
				"last_cache_misses",
				"curr_cache_misses",
				"last_energy_in_core",
				"curr_energy_in_core",
				"last_energy_in_dram",
				"curr_energy_in_dram",
			},
			nil,
		)
		desc := prometheus.MustNewConstMetric(
			de,
			prometheus.CounterValue,
			float64(v.LastEnergyInCore+v.LastEnergyInDram),
			v.Pod, v.Namespace, v.Command,
			strconv.FormatUint(v.LastCPUTime, 10), strconv.FormatUint(v.CPUTime, 10),
			strconv.FormatUint(v.LastCPUCycles, 10), strconv.FormatUint(v.CPUCycles, 10),
			strconv.FormatUint(v.LastCPUInstr, 10), strconv.FormatUint(v.CPUInstr, 10),
			strconv.FormatUint(v.LastCacheMisses, 10), strconv.FormatUint(v.CacheMisses, 10),
			strconv.FormatUint(v.LastEnergyInCore, 10), strconv.FormatUint(v.EnergyInCore, 10),
			strconv.FormatUint(v.LastEnergyInDram, 10), strconv.FormatUint(v.EnergyInDram, 10),
		)
		ch <- desc

		// de_total and desc_total give indexable values for total energy consumptions for all pods
		de_total := prometheus.NewDesc(
			"pod_energy_total",
			"Pod current energy consumption",
			[]string{
				"pod_name",
				"pod_namespace",
				"command",
			},
			nil,
		)
		desc_total := prometheus.MustNewConstMetric(
			de_total,
			prometheus.CounterValue,
			float64(v.EnergyInCore+v.EnergyInDram),
			v.Pod, v.Namespace, v.Command,
		)
		ch <- desc_total

		// de_current and desc_current give indexable values for current energy consumptions (in 3 seconds) for all pods
		de_current := prometheus.NewDesc(
			"pod_energy_current",
			"Pod current energy consumption",
			[]string{
				"pod_name",
				"pod_namespace",
				"command",
			},
			nil,
		)
		desc_current := prometheus.MustNewConstMetric(
			de_current,
			prometheus.CounterValue,
			float64(v.EnergyInCore+v.EnergyInDram),
			v.Pod, v.Namespace, v.Command,
		)
		ch <- desc_current

		// de_cpu_current and desc_cpu_current give indexable values for current CPU energy consumptions (in 3 seconds) for all pods
		de_cpu_current := prometheus.NewDesc(
			"pod_cpu_energy_current",
			"Pod CPU energy consumption",
			[]string{
				"pod_name",
				"pod_namespace",
				"command",
			},
			nil,
		)
		desc_cpu_current := prometheus.MustNewConstMetric(
			de_cpu_current,
			prometheus.CounterValue,
			float64(v.EnergyInCore),
			v.Pod, v.Namespace, v.Command,
		)
		ch <- desc_cpu_current

		// de_cpu_total and desc_cpu_total give indexable values for total CPU energy consumptions for all pods
		de_cpu_total := prometheus.NewDesc(
			"pod_cpu_energy_total",
			"Pod CPU total energy consumption",
			[]string{
				"pod_name",
				"pod_namespace",
				"command",
			},
			nil,
		)
		desc_cpu_total := prometheus.MustNewConstMetric(
			de_cpu_total,
			prometheus.CounterValue,
			float64(v.LastEnergyInCore),
			v.Pod, v.Namespace, v.Command,
		)
		ch <- desc_cpu_total

		// de_dram_current and desc_dram_current give indexable values for current DRAM energy consumptions (in 3 seconds) for all pods
		de_dram_current := prometheus.NewDesc(
			"pod_dram_energy_current",
			"Pod DRAM energy consumption",
			[]string{
				"pod_name",
				"pod_namespace",
				"command",
			},
			nil,
		)
		desc_dram_current := prometheus.MustNewConstMetric(
			de_dram_current,
			prometheus.CounterValue,
			float64(v.EnergyInDram),
			v.Pod, v.Namespace, v.Command,
		)
		ch <- desc_dram_current

		// de_dram_total and desc_dram_total give indexable values for total DRAM energy consumptions for all pods
		de_dram_total := prometheus.NewDesc(
			"pod_dram_energy_total",
			"Pod DRAM total energy consumption",
			[]string{
				"pod_name",
				"pod_namespace",
				"command",
			},
			nil,
		)
		desc_dram_total := prometheus.MustNewConstMetric(
			de_dram_total,
			prometheus.CounterValue,
			float64(v.LastEnergyInDram),
			v.Pod, v.Namespace, v.Command,
		)
		ch <- desc_dram_total
	}
}

func (c *Collector) Destroy() {
	if c.modules != nil {
		attacher.DetachBPFModules(c.modules)
	}
}
