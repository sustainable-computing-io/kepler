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
	"strings"

	"github.com/sustainable-computing-io/kepler/pkg/attacher"
	"github.com/sustainable-computing-io/kepler/pkg/cgroup"
	"github.com/prometheus/client_golang/prometheus"
)

var basicStatLabels []string = []string{
	"pod_name", "pod_namespace", "command",
	"total_cpu_time", "curr_cpu_time",
	"total_cpu_cycles", "curr_cpu_cycles",
	"total_cpu_instructions", "curr_cpu_instructions",
	"total_cache_misses", "curr_cache_misses",
	"total_energy_in_core", "curr_energy_in_core",
	"total_energy_in_dram", "curr_energy_in_dram",
	"total_energy_in_gpu", "curr_energy_in_gpu",
	"total_energy_in_other", "curr_energy_in_other",
	"avg_cpu_frequency",
	"block_devices_used",
	"total_bytes_read", "curr_bytes_read",
	"total_bytes_writes","curr_bytes_writes",
}
var cgroupStatLabels []string = cgroup.GetAvailableCgroupMetrics()

const (
	NODE_ENERGY_STAT_METRRIC = "node_energy_stat"
	POD_ENERGY_STAT_METRIC = "pod_energy_stat"
	NODE_ENERGY_METRIC = "node_hwmon_energy_joule_total"
	FREQ_METRIC = "node_cpu_scaling_frequency_hertz"
	PACKAGE_ENERGY_METRIC = "node_package_energy_joule"
	PER_CPU_STAT_METRIC = "pod_cpu_cpu_time_us"
)

type PodMetric struct {
	FullStat  *prometheus.Desc
	AllCurr   *prometheus.Desc
	AllAggr   *prometheus.Desc
	CPUCurr   *prometheus.Desc
	CPUAggr   *prometheus.Desc
	DRAMCurr  *prometheus.Desc
	DRAMAggr  *prometheus.Desc 
	GPUCurr   *prometheus.Desc
	GPUAggr   *prometheus.Desc
	OtherCurr *prometheus.Desc
	OtherAggr *prometheus.Desc
}


type Collector struct {
	NodeEnergyStatMetric *prometheus.Desc
	modules *attacher.BpfModuleTables
}

func New() (*Collector, error) {
	return &Collector{
		NodeEnergyStatMetric: prometheus.NewDesc(
			NODE_ENERGY_STAT_METRRIC,
			"Node energy consumption stats",
			[]string{
				"node_name",
				"cpu_architecture",
				"curr_cpu_time",
				"curr_cpu_cycles",
				"curr_cpu_instructions",
				"curr_resident_memory",
				"curr_cache_misses",
				"curr_energy_in_core",
				"curr_energy_in_dram",
				"curr_energy_in_gpu",
				"curr_energy_in_other",
			},
			nil,
		),
	}, nil
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

func (c *Collector) getPodDescription() *PodMetric {
	fullStat := prometheus.NewDesc(
		POD_ENERGY_STAT_METRIC,
		"Pod energy consumption status",
		append(basicStatLabels, cgroupStatLabels...),
		nil,
	)
	allCurr := prometheus.NewDesc(
		"pod_energy_current",
		"Pod current energy consumption",
		[]string{
			"pod_name",
			"pod_namespace",
		},
		nil,
	)
	allAggr := prometheus.NewDesc(
		"pod_energy_total",
		"Pod total energy consumption",
		[]string{
			"pod_name",
			"pod_namespace",
		},
		nil,
	)
	cpuCurr := prometheus.NewDesc(
		"pod_cpu_energy_current",
		"Pod CPU current energy consumption",
		[]string{
			"pod_name",
			"pod_namespace",
		},
		nil,
	)
	cpuAggr := prometheus.NewDesc(
		"pod_cpu_energy_total",
		"Pod CPU total energy consumption",
		[]string{
			"pod_name",
			"pod_namespace",
		},
		nil,
	)
	dramCurr := prometheus.NewDesc(
		"pod_dram_energy_current",
		"Pod DRAM current energy consumption",
		[]string{
			"pod_name",
			"pod_namespace",
		},
		nil,
	)
	dramAggr := prometheus.NewDesc(
		"pod_dram_energy_total",
		"Pod DRAM total energy consumption",
		[]string{
			"pod_name",
			"pod_namespace",
		},
		nil,
	)
	gpuCurr := prometheus.NewDesc(
		"pod_gpu_energy_current",
		"Pod GPU current energy consumption",
		[]string{
			"pod_name",
			"pod_namespace",
		},
		nil,
	)
	gpuAggr := prometheus.NewDesc(
		"pod_gpu_energy_total",
		"Pod GPU total energy consumption",
		[]string{
			"pod_name",
			"pod_namespace",
		},
		nil,
	)
	otherCurr := prometheus.NewDesc(
		"pod_other_energy_joule",
		"Pod OTHER current energy consumption besides CPU and memory",
		[]string{
			"pod_name",
			"pod_namespace",
		},
		nil,
	)
	otherAggr := prometheus.NewDesc(
		"pod_other_energy_joule_total",
		"Pod OTHER total energy consumption besides CPU and memory",
		[]string{
			"pod_name",
			"pod_namespace",
		},
		nil,
	)
	return &PodMetric{
		FullStat: fullStat,
		AllCurr: allCurr,
		AllAggr: allAggr,
		CPUCurr: cpuCurr,
		CPUAggr: cpuAggr,
		DRAMCurr: dramCurr,
		DRAMAggr: dramAggr,
		GPUCurr: gpuCurr,
		GPUAggr: gpuAggr,
		OtherCurr: otherCurr,
		OtherAggr: otherAggr,
	}
}

func (c *Collector) getSensorDescription() *prometheus.Desc {
	return prometheus.NewDesc(
		NODE_ENERGY_METRIC,
		"Hardware monitor for energy consumed in joules in the node.",
		[]string{
			"instance",
			"chip",
			"sensor",
		},
		nil,
	)
}

func (c *Collector) getFreqDescription() *prometheus.Desc {
	return prometheus.NewDesc(
		FREQ_METRIC,
		"Current scaled cpu thread frequency in hertz.",
		[]string{
			"instance",
			"cpu",
		},
		nil,
	)
}

func (c *Collector) getPackageEnergyDescription() *prometheus.Desc {
	return prometheus.NewDesc(
		PACKAGE_ENERGY_METRIC,
		"Current package energy by RAPL in joules.",
		[]string{
			"instance",
			"pkg_id",
			"core",
			"dram",
			"uncore",
		},
		nil,
	)
}

func (c *Collector) getPodDetailedCPUTimeDescription() *prometheus.Desc {
	return prometheus.NewDesc(
		PER_CPU_STAT_METRIC,
		"Current CPU time per CPU.",
		[]string{
			"pod_name",
			"pod_namespace",
			"cpu",
		},
		nil,
	)
}


func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	lock.Lock()
	defer lock.Unlock()
	ch <- c.NodeEnergyStatMetric
	podDesc := c.getPodDescription()
	ch <- podDesc.FullStat
	ch <- podDesc.AllCurr
	ch <- podDesc.AllAggr
	ch <- podDesc.CPUCurr
	ch <- podDesc.CPUAggr
	ch <- podDesc.DRAMCurr
	ch <- podDesc.DRAMAggr
	ch <- podDesc.GPUCurr
	ch <- podDesc.GPUAggr
	ch <- podDesc.OtherCurr
	ch <- podDesc.OtherAggr
	ch <- c.getSensorDescription()
	ch <- c.getFreqDescription()
	ch <- c.getPackageEnergyDescription()
	ch <- c.getPodDetailedCPUTimeDescription()
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	lock.Lock()
	defer lock.Unlock()

	cpuTime := fmt.Sprintf("%f", currNodeEnergy.CPUTime)
	energyInCore := fmt.Sprintf("%f", currNodeEnergy.EnergyInCore)
	energyInDram := fmt.Sprintf("%f", currNodeEnergy.EnergyInDram)
	energyInOther := fmt.Sprintf("%f", currNodeEnergy.EnergyInOther)
	energyInGpu := fmt.Sprintf("%f", currNodeEnergy.EnergyInGPU)
	resMem := fmt.Sprintf("%f", currNodeEnergy.NodeMem)
	desc := prometheus.MustNewConstMetric(
		c.NodeEnergyStatMetric,
		prometheus.CounterValue,
		currNodeEnergy.EnergyInCore+currNodeEnergy.EnergyInDram+currNodeEnergy.EnergyInOther+currNodeEnergy.EnergyInGPU,
		nodeName, cpuArch,
		cpuTime,
		strconv.FormatUint(currNodeEnergy.CPUCycles, 10),
		strconv.FormatUint(currNodeEnergy.CPUInstr, 10),
		resMem,
		strconv.FormatUint(currNodeEnergy.CacheMisses, 10),
		energyInCore, energyInDram, energyInGpu, energyInOther,
	)
	ch <- desc

	for _, v := range podEnergy {
		podDesc := c.getPodDescription()
		aggCPU := fmt.Sprintf("%f", v.AggCPUTime)
		currCPU := fmt.Sprintf("%f", v.CurrCPUTime)
		avgFreq := fmt.Sprintf("%f", float64(v.AvgCPUFreq))
		disks := fmt.Sprintf("%d", v.Disks)

		basicStatValues := []string{
			v.PodName, v.Namespace, v.Command,
			aggCPU, currCPU,
			strconv.FormatUint(v.AggCPUCycles, 10), strconv.FormatUint(v.CurrCPUCycles, 10),
			strconv.FormatUint(v.AggCPUInstr, 10), strconv.FormatUint(v.CurrCPUInstr, 10),
			strconv.FormatUint(v.AggCacheMisses, 10), strconv.FormatUint(v.CurrCacheMisses, 10),
			strconv.FormatUint(v.AggEnergyInCore, 10), strconv.FormatUint(v.CurrEnergyInCore, 10),
			strconv.FormatUint(v.AggEnergyInDram, 10), strconv.FormatUint(v.CurrEnergyInDram, 10),
			strconv.FormatUint(v.AggEnergyInGPU, 10), strconv.FormatUint(v.CurrEnergyInGPU, 10),
			strconv.FormatUint(v.AggEnergyInOther, 10), strconv.FormatUint(v.CurrEnergyInOther, 10),
			avgFreq, 
			disks,
			strconv.FormatUint(v.AggBytesRead, 10), strconv.FormatUint(v.CurrBytesRead, 10),
			strconv.FormatUint(v.AggBytesWrite, 10), strconv.FormatUint(v.CurrBytesWrite, 10),
		}
	
		var podEnergyVals []string
		podEnergyVals = append(podEnergyVals, basicStatValues...)
		
		for _, fullStatLabel := range cgroupStatLabels {
			splitIndex := strings.Index(fullStatLabel, "_")
			valType := fullStatLabel[0:splitIndex]
			statLabel := fullStatLabel[splitIndex+1 : len(fullStatLabel)]
			statValue, exist := v.CgroupFSStats[statLabel]
			if exist {
				switch valType {
				case "curr":
					podEnergyVals = append(podEnergyVals, strconv.FormatUint(statValue.Curr, 10))
				case "total":
					podEnergyVals = append(podEnergyVals, strconv.FormatUint(statValue.Aggr, 10))
				}
			} else {
				podEnergyVals = append(podEnergyVals, "-1")
			}
		}

		desc := prometheus.MustNewConstMetric(
			podDesc.FullStat,
			prometheus.CounterValue,
			float64(v.CurrEnergyInCore+v.CurrEnergyInDram+v.CurrEnergyInGPU+v.CurrEnergyInOther),
			podEnergyVals...,
		)
		ch <- desc

		// de_current and desc_current give indexable values for current energy consumptions (in 3 seconds) for all pods
		desc_current := prometheus.MustNewConstMetric(
			podDesc.AllCurr,
			prometheus.GaugeValue,
			float64(v.CurrEnergyInCore+v.CurrEnergyInDram+v.CurrEnergyInGPU+v.CurrEnergyInOther),
			v.PodName, v.Namespace,
		)
		ch <- desc_current

		// de_total and desc_total give indexable values for total energy consumptions for all pods
		desc_total := prometheus.MustNewConstMetric(
			podDesc.AllAggr,
			prometheus.CounterValue,
			float64(v.AggEnergyInCore+v.AggEnergyInDram+v.AggEnergyInOther),
			v.PodName, v.Namespace,
		)
		ch <- desc_total

		// de_cpu_current and desc_cpu_current give indexable values for current CPU energy consumptions (in 3 seconds) for all pods
		desc_cpu_current := prometheus.MustNewConstMetric(
			podDesc.CPUCurr,
			prometheus.GaugeValue,
			float64(v.CurrEnergyInCore),
			v.PodName, v.Namespace,
		)
		ch <- desc_cpu_current

		// de_cpu_total and desc_cpu_total give indexable values for total CPU energy consumptions for all pods
		desc_cpu_total := prometheus.MustNewConstMetric(
			podDesc.CPUAggr,
			prometheus.CounterValue,
			float64(v.AggEnergyInCore),
			v.PodName, v.Namespace,
		)
		ch <- desc_cpu_total

		// de_dram_current and desc_dram_current give indexable values for current DRAM energy consumptions (in 3 seconds) for all pods
		desc_dram_current := prometheus.MustNewConstMetric(
			podDesc.DRAMCurr,
			prometheus.GaugeValue,
			float64(v.CurrEnergyInDram),
			v.PodName, v.Namespace,
		)
		ch <- desc_dram_current

		// de_dram_total and desc_dram_total give indexable values for total DRAM energy consumptions for all pods
		desc_dram_total := prometheus.MustNewConstMetric(
			podDesc.DRAMAggr,
			prometheus.CounterValue,
			float64(v.AggEnergyInDram),
			v.PodName, v.Namespace,
		)
		ch <- desc_dram_total

		// de_gpu_current and desc_gpu_current give indexable values for current GPU energy consumptions (in 3 seconds) for all pods
		desc_gpu_current := prometheus.MustNewConstMetric(
			podDesc.GPUCurr,
			prometheus.GaugeValue,
			float64(v.CurrEnergyInGPU),
			v.PodName, v.Namespace,
		)
		ch <- desc_gpu_current

		// de_gpu_total and desc_gpu_total give indexable values for total GPU energy consumptions for all pods
		desc_gpu_total := prometheus.MustNewConstMetric(
			podDesc.GPUAggr,
			prometheus.CounterValue,
			float64(v.AggEnergyInGPU),
			v.PodName, v.Namespace,
		)
		ch <- desc_gpu_total

		// de_other_current and desc_other_current give indexable values for current DRAM energy consumptions (in 3 seconds) for all pods
		desc_other_current := prometheus.MustNewConstMetric(
			podDesc.OtherCurr,
			prometheus.GaugeValue,
			float64(v.CurrEnergyInOther),
			v.PodName, v.Namespace,
		)
		ch <- desc_other_current

		// de_other_total and desc_other_total give indexable values for total DRAM energy consumptions for all pods
		desc_other_total := prometheus.MustNewConstMetric(
			podDesc.OtherAggr,
			prometheus.CounterValue,
			float64(v.AggEnergyInOther),
			v.PodName, v.Namespace,
		)
		ch <- desc_other_total

		// collect CPU time per CPU for finer granularity
		for cpu, cpuTime := range v.CurrCPUTimePerCPU {
			detailedDesc := c.getPodDetailedCPUTimeDescription()
			metric := prometheus.MustNewConstMetric(
				detailedDesc,
				prometheus.GaugeValue,
				float64(cpuTime),
				v.PodName, v.Namespace, strconv.Itoa(int(cpu)),
			)
			ch <- metric
		}
	}

	// de_node_energy and desc_node_energy give indexable values for total energy consumptions of a node
	for sensorID, energy := range nodeEnergy {
		desc := c.getSensorDescription()
		metric := prometheus.MustNewConstMetric(
			desc,
			prometheus.CounterValue,
			energy/1000.0, /*miliJoule to Joule*/
			nodeName,
			sensorID,
			"power_meter",
		)
		ch <- metric
	}

	// de_core_freq and desc_core_freq give indexable values for each cpu core freq of a node
	for cpuID, freq := range cpuFrequency {
		desc := c.getFreqDescription()
		metric := prometheus.MustNewConstMetric(
			desc,
			prometheus.GaugeValue,
			float64(freq),
			nodeName,
			fmt.Sprintf("%d", cpuID),
		)
		ch <- metric
	}

	// collect energy per package by RAPL
	for pkgID, energy := range currPackageEnergy {
		desc := c.getPackageEnergyDescription()
		coreEnergy := fmt.Sprintf("%f", float64(energy.Core)/1000.0)
		dramEnergy := fmt.Sprintf("%f", float64(energy.DRAM)/1000.0)
		uncoreEnergy := fmt.Sprintf("%f", float64(energy.Uncore)/1000.0)

		metric := prometheus.MustNewConstMetric(
			desc,
			prometheus.GaugeValue,
			float64(energy.Pkg)/1000.0,
			nodeName,
			strconv.Itoa(pkgID),
			coreEnergy,
			dramEnergy,
			uncoreEnergy,
		)
		ch <- metric
	}
}

func (c *Collector) Destroy() {
	if c.modules != nil {
		attacher.DetachBPFModules(c.modules)
	}
}
