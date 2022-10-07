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
	"sort"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sustainable-computing-io/kepler/pkg/attacher"
	"k8s.io/klog/v2"
)

const (
	nodeEnergyStatMetric    = "node_energy_stat"
	podEnergyStatMetric     = "pod_energy_stat"
	nodeEnergyMetric        = "node_hwmon_energy_joule_total"
	freqMetric              = "node_cpu_scaling_frequency_hertz"
	pkgEnergyMetric         = "node_package_energy_millijoule"
	perCPUStatMetric        = "pod_cpu_cpu_time_us"
	podCPUInstructionMetric = "pod_cpu_instructions"

	podLabelPrefix  = "pod_"
	nodeLabelPrefix = "node_"
	CurrPrefix      = "curr_"
	AggrPrefix      = "total_"

	mJSuffix = "millijoule"
	JSuffix  = "joule"
)

var (
	dynEnergyLabel = "dynamic_energy_"
	EnergyLabels   = map[string]string{
		"core":   "energy_in_core_",
		"dram":   "energy_in_dram_",
		"uncore": "energy_in_uncore_",
		"pkg":    "energy_in_pkg_",
		"gpu":    "energy_in_gpu_",
		"other":  "energy_in_other_",
	}
	EnergyLabelsKeys = sortEnergyLabelKeys()
	allEnergyLabel   = "energy_"

	basicPodLabels []string = []string{
		"pod_name", "pod_namespace", "command",
	}
	prometheusMetrics []string
	podEnergyLabels   []string
	basicNodeLabels   []string = []string{
		"node_name", "cpu_architecture",
	}
)

func sortEnergyLabelKeys() (ekeys []string) {
	for ekey := range EnergyLabels {
		ekeys = append(ekeys, ekey)
	}
	sort.Strings(ekeys)
	return
}

type DescriptionGroup struct {
	Stat          *prometheus.Desc
	CurrEnergy    map[string]*prometheus.Desc
	AggrEnergy    map[string]*prometheus.Desc
	AllCurrEnergy *prometheus.Desc
	AllAggrEnergy *prometheus.Desc
}

type Collector struct {
	NodeDesc *DescriptionGroup
	modules  *attacher.BpfModuleTables
}

func setPodStatProm() {
	prometheusMetrics = getPrometheusMetrics()
	podEnergyLabels = []string{}
	podEnergyLabels = append(podEnergyLabels, basicPodLabels...)
	podEnergyLabels = append(podEnergyLabels, prometheusMetrics...)
}

func New() (*Collector, error) {
	c := &Collector{}
	c.setNodeDescriptionGroup()
	return c, nil
}

func (c *Collector) Attach() error {
	defer func() {
		if r := recover(); r != nil {
			klog.Infoln(r)
		}
	}()
	m, err := attacher.AttachBPFAssets()
	setPodStatProm()
	if err != nil {
		return fmt.Errorf("failed to attach bpf assets: %v", err)
	}
	c.modules = m
	c.reader()
	return nil
}

func (c *Collector) setNodeDescriptionGroup() {
	jsuffix := JSuffix
	var nodeEnergyLabels []string
	nodeEnergyLabels = append(nodeEnergyLabels, basicNodeLabels...)
	for _, metric := range metricNames {
		nodeEnergyLabels = append(nodeEnergyLabels, nodeLabelPrefix+metric)
	}
	for _, ekey := range EnergyLabelsKeys {
		elabel := EnergyLabels[ekey]
		nodeEnergyLabels = append(nodeEnergyLabels, nodeLabelPrefix+CurrPrefix+elabel+jsuffix)
	}
	statDesc := prometheus.NewDesc(
		nodeEnergyStatMetric,
		"Node energy consumption latest stats",
		nodeEnergyLabels,
		nil,
	)
	currEnergyDesc := make(map[string]*prometheus.Desc)
	for ekey, elabel := range EnergyLabels {
		currEnergyDesc[elabel] = prometheus.NewDesc(
			nodeLabelPrefix+CurrPrefix+elabel+jsuffix,
			fmt.Sprintf("%s current energy consumption in %s (%s)", nodeLabelPrefix, ekey, jsuffix),
			[]string{
				"instance",
			},
			nil,
		)
	}
	allCurrDesc := prometheus.NewDesc(
		nodeLabelPrefix+CurrPrefix+allEnergyLabel+jsuffix,
		fmt.Sprintf("%s current energy consumption (%s)", nodeLabelPrefix, jsuffix),
		[]string{
			"instance",
		},
		nil,
	)
	c.NodeDesc = &DescriptionGroup{
		Stat:          statDesc,
		CurrEnergy:    currEnergyDesc,
		AllCurrEnergy: allCurrDesc,
	}
}

func (c *Collector) getPodDescriptionGroup() *DescriptionGroup {
	jsuffix := mJSuffix
	statDesc := prometheus.NewDesc(
		podEnergyStatMetric,
		"Pod energy consumption status",
		podEnergyLabels,
		nil,
	)
	currEnergyDesc := make(map[string]*prometheus.Desc)
	aggrEnergyDesc := make(map[string]*prometheus.Desc)
	for ekey, elabel := range EnergyLabels {
		currEnergyDesc[elabel] = prometheus.NewDesc(
			podLabelPrefix+CurrPrefix+elabel+jsuffix,
			fmt.Sprintf("%s current energy consumption in %s (%s)", podLabelPrefix, ekey, jsuffix),
			basicPodLabels,
			nil,
		)
	}
	currEnergyDesc[dynEnergyLabel] = prometheus.NewDesc(
		podLabelPrefix+CurrPrefix+dynEnergyLabel+jsuffix,
		fmt.Sprintf("%s current dynamic energy consumption (%s)", podLabelPrefix, jsuffix),
		basicPodLabels,
		nil,
	)
	for ekey, elabel := range EnergyLabels {
		aggrEnergyDesc[elabel] = prometheus.NewDesc(
			podLabelPrefix+AggrPrefix+elabel+jsuffix,
			fmt.Sprintf("%s total energy consumption in %s (%s)", podLabelPrefix, ekey, jsuffix),
			basicPodLabels,
			nil,
		)
	}
	aggrEnergyDesc[dynEnergyLabel] = prometheus.NewDesc(
		podLabelPrefix+AggrPrefix+dynEnergyLabel+jsuffix,
		fmt.Sprintf("%s total dynamic energy consumption (%s)", podLabelPrefix, jsuffix),
		basicPodLabels,
		nil,
	)
	allCurrDesc := prometheus.NewDesc(
		podLabelPrefix+CurrPrefix+allEnergyLabel+jsuffix,
		fmt.Sprintf("%s current energy consumption (%s)", podLabelPrefix, jsuffix),
		basicPodLabels,
		nil,
	)
	allAggrDesc := prometheus.NewDesc(
		podLabelPrefix+AggrPrefix+allEnergyLabel+jsuffix,
		fmt.Sprintf("%s total energy consumption (%s)", podLabelPrefix, jsuffix),
		basicPodLabels,
		nil,
	)
	return &DescriptionGroup{
		Stat:          statDesc,
		CurrEnergy:    currEnergyDesc,
		AggrEnergy:    aggrEnergyDesc,
		AllCurrEnergy: allCurrDesc,
		AllAggrEnergy: allAggrDesc,
	}
}

func (c *Collector) getSensorDescription() *prometheus.Desc {
	return prometheus.NewDesc(
		nodeEnergyMetric,
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
		freqMetric,
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
		pkgEnergyMetric,
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
		perCPUStatMetric,
		"Current CPU time per CPU.",
		[]string{
			"pod_name",
			"pod_namespace",
			"cpu",
		},
		nil,
	)
}

func (c *Collector) getPodInstructionDescription() *prometheus.Desc {
	return prometheus.NewDesc(
		podCPUInstructionMetric,
		"Recorded CPU instructions during the period.",
		[]string{
			"pod_name",
			"pod_namespace",
		},
		nil,
	)
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	lock.Lock()
	defer lock.Unlock()
	ch <- c.NodeDesc.Stat
	for _, desc := range c.NodeDesc.CurrEnergy {
		ch <- desc
	}
	podDesc := c.getPodDescriptionGroup()
	ch <- podDesc.Stat
	for _, desc := range podDesc.CurrEnergy {
		ch <- desc
	}
	for _, desc := range podDesc.AggrEnergy {
		ch <- desc
	}
	ch <- c.getSensorDescription()
	ch <- c.getFreqDescription()
	ch <- c.getPackageEnergyDescription()
	ch <- c.getPodDetailedCPUTimeDescription()
	if cpuInstrCounterEnabled {
		ch <- c.getPodInstructionDescription()
	}
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	lock.Lock()
	defer lock.Unlock()

	// general node-level stat with current sensor value
	desc := prometheus.MustNewConstMetric(
		c.NodeDesc.Stat,
		prometheus.GaugeValue,
		float64(nodeEnergy.SensorEnergy.Curr())/1000.0, /*miliJoule to Joule*/
		nodeEnergy.ToPrometheusValues()...,
	)
	ch <- desc
	// sum curr node energy in J (for each energy unit)
	for ekey, elabel := range EnergyLabels {
		edesc := prometheus.MustNewConstMetric(
			c.NodeDesc.CurrEnergy[elabel],
			prometheus.GaugeValue,
			float64(nodeEnergy.GetPrometheusEnergyValue(ekey))/1000.0, /*miliJoule to Joule*/
			nodeName,
		)
		ch <- edesc
	}
	// all curr node energy in J
	edesc := prometheus.MustNewConstMetric(
		c.NodeDesc.AllCurrEnergy,
		prometheus.GaugeValue,
		float64(nodeEnergy.Curr())/1000.0, /*miliJoule to Joule*/
		nodeName,
	)
	ch <- edesc

	for _, v := range podEnergy {
		// general pod-level stat with current sum value of pkg+gpu+other
		podDesc := c.getPodDescriptionGroup()
		desc := prometheus.MustNewConstMetric(
			podDesc.Stat,
			prometheus.GaugeValue,
			float64(v.Curr()),
			v.ToPrometheusValues()...,
		)
		ch <- desc
		// ratio curr pod energy in mJ (for each energy unit)
		for ekey, elabel := range EnergyLabels {
			edesc = prometheus.MustNewConstMetric(
				podDesc.CurrEnergy[elabel],
				prometheus.GaugeValue,
				v.GetPrometheusEnergyValue(ekey, true),
				v.GetBasicValues()...,
			)
			ch <- edesc
		}
		// dyn curr pod energy in mJ
		edesc = prometheus.MustNewConstMetric(
			podDesc.CurrEnergy[dynEnergyLabel],
			prometheus.GaugeValue,
			float64(v.DynEnergy.Curr),
			v.GetBasicValues()...,
		)
		ch <- edesc
		// all curr pod energy in mJ
		edesc = prometheus.MustNewConstMetric(
			podDesc.AllCurrEnergy,
			prometheus.GaugeValue,
			float64(v.Curr()),
			v.GetBasicValues()...,
		)
		ch <- edesc
		// ratio aggr pod energy in mJ
		for ekey, elabel := range EnergyLabels {
			edesc = prometheus.MustNewConstMetric(
				podDesc.AggrEnergy[elabel],
				prometheus.CounterValue,
				v.GetPrometheusEnergyValue(ekey, false),
				v.GetBasicValues()...,
			)
			ch <- edesc
		}
		// dyn aggr pod energy in mJ
		edesc = prometheus.MustNewConstMetric(
			podDesc.AggrEnergy[dynEnergyLabel],
			prometheus.CounterValue,
			float64(v.DynEnergy.Aggr),
			v.GetBasicValues()...,
		)
		ch <- edesc
		// all curr pod energy in mJ
		edesc = prometheus.MustNewConstMetric(
			podDesc.AllAggrEnergy,
			prometheus.CounterValue,
			float64(v.Aggr()),
			v.GetBasicValues()...,
		)
		ch <- edesc
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
		if cpuInstrCounterEnabled &&
			v.CounterStats[attacher.CPUInstructionLabel] != nil {
			// all curr pod cpu instructions
			edesc = prometheus.MustNewConstMetric(
				c.getPodInstructionDescription(),
				prometheus.GaugeValue,
				float64(v.CounterStats[attacher.CPUInstructionLabel].Curr),
				v.PodName, v.Namespace)
			ch <- edesc
		}
	}

	// de_node_energy and desc_node_energy give indexable values for total energy consumptions of a node
	for sensorID, energy := range sensorEnergy {
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

	// collect energy per RAPL components in mJ
	for pkgID, energy := range pkgEnergy {
		desc := c.getPackageEnergyDescription()
		coreEnergy := strconv.FormatUint(energy.Core, 10)
		dramEnergy := strconv.FormatUint(energy.DRAM, 10)
		uncoreEnergy := strconv.FormatUint(energy.Uncore, 10)

		metric := prometheus.MustNewConstMetric(
			desc,
			prometheus.GaugeValue,
			float64(energy.Pkg),
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
