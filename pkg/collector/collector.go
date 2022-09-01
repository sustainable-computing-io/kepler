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
	"sort"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sustainable-computing-io/kepler/pkg/attacher"
)

const (
	NODE_ENERGY_STAT_METRRIC = "node_energy_stat"
	POD_ENERGY_STAT_METRIC   = "pod_energy_stat"
	NODE_ENERGY_METRIC       = "node_hwmon_energy_joule_total"
	FREQ_METRIC              = "node_cpu_scaling_frequency_hertz"
	PACKAGE_ENERGY_METRIC    = "node_package_energy_millijoule"
	PER_CPU_STAT_METRIC      = "pod_cpu_cpu_time_us"

	POD_LABEL_PREFIX  = "pod_"
	NODE_LABEL_PREFIX = "node_"
	CURR_PREFIX       = "curr_"
	AGGR_PREFIX       = "total_"

	mJ_SUFFIX = "millijoule"
	J_SUFFIX  = "joule"
)

var (
	DYN_ENERGY_LABEL = "dynamic_energy_"
	ENERGY_LABELS    = map[string]string{
		"core":   "energy_in_core_",
		"dram":   "energy_in_dram_",
		"uncore": "energy_in_uncore_",
		"pkg":    "energy_in_pkg_",
		"gpu":    "energy_in_gpu_",
		"other":  "energy_in_other_",
	}
	ENERGY_LABEL_KEYS = sortEnergyLabelKeys()
	ALL_ENERGY_LABEL  = "energy_"

	basicPodLabels []string = []string{
		"pod_name", "pod_namespace", "command",
	}
	prometheusMetrics []string = getPrometheusMetrics()
	podEnergyLabels   []string = append(basicPodLabels, prometheusMetrics...)
	basicNodeLabels   []string = []string{
		"node_name", "cpu_architecture",
	}
)

func sortEnergyLabelKeys() (ekeys []string) {
	for ekey, _ := range ENERGY_LABELS {
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

func New() (*Collector, error) {
	c := &Collector{}
	c.setNodeDescriptionGroup()
	return c, nil
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

func (c *Collector) setNodeDescriptionGroup() {
	jsuffix := J_SUFFIX
	var nodeEnergyLabels []string
	nodeEnergyLabels = append(nodeEnergyLabels, basicNodeLabels...)
	for _, metric := range metricNames {
		nodeEnergyLabels = append(nodeEnergyLabels, NODE_LABEL_PREFIX+metric)
	}
	for _, ekey := range ENERGY_LABEL_KEYS {
		elabel := ENERGY_LABELS[ekey]
		nodeEnergyLabels = append(nodeEnergyLabels, NODE_LABEL_PREFIX+CURR_PREFIX+elabel+jsuffix)
	}
	statDesc := prometheus.NewDesc(
		NODE_ENERGY_STAT_METRRIC,
		"Node energy consumption latest stats",
		nodeEnergyLabels,
		nil,
	)
	currEnergyDesc := make(map[string]*prometheus.Desc)
	for ekey, elabel := range ENERGY_LABELS {
		currEnergyDesc[elabel] = prometheus.NewDesc(
			NODE_LABEL_PREFIX+CURR_PREFIX+elabel+jsuffix,
			fmt.Sprintf("%s current energy consumption in %s (%s)", NODE_LABEL_PREFIX, ekey, jsuffix),
			[]string{
				"instance",
			},
			nil,
		)
	}
	allCurrDesc := prometheus.NewDesc(
		NODE_LABEL_PREFIX+CURR_PREFIX+ALL_ENERGY_LABEL+jsuffix,
		fmt.Sprintf("%s current energy consumption (%s)", NODE_LABEL_PREFIX, jsuffix),
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
	jsuffix := mJ_SUFFIX
	statDesc := prometheus.NewDesc(
		POD_ENERGY_STAT_METRIC,
		"Pod energy consumption status",
		podEnergyLabels,
		nil,
	)
	currEnergyDesc := make(map[string]*prometheus.Desc)
	aggrEnergyDesc := make(map[string]*prometheus.Desc)
	for ekey, elabel := range ENERGY_LABELS {
		currEnergyDesc[elabel] = prometheus.NewDesc(
			POD_LABEL_PREFIX+CURR_PREFIX+elabel+jsuffix,
			fmt.Sprintf("%s current energy consumption in %s (%s)", POD_LABEL_PREFIX, ekey, jsuffix),
			basicPodLabels,
			nil,
		)
	}
	currEnergyDesc[DYN_ENERGY_LABEL] = prometheus.NewDesc(
		POD_LABEL_PREFIX+CURR_PREFIX+DYN_ENERGY_LABEL+jsuffix,
		fmt.Sprintf("%s current dynamic energy consumption (%s)", POD_LABEL_PREFIX, jsuffix),
		basicPodLabels,
		nil,
	)
	for ekey, elabel := range ENERGY_LABELS {
		aggrEnergyDesc[elabel] = prometheus.NewDesc(
			POD_LABEL_PREFIX+AGGR_PREFIX+elabel+jsuffix,
			fmt.Sprintf("%s total energy consumption in %s (%s)", POD_LABEL_PREFIX, ekey, jsuffix),
			basicPodLabels,
			nil,
		)
	}
	aggrEnergyDesc[DYN_ENERGY_LABEL] = prometheus.NewDesc(
		POD_LABEL_PREFIX+AGGR_PREFIX+DYN_ENERGY_LABEL+jsuffix,
		fmt.Sprintf("%s total dynamic energy consumption (%s)", POD_LABEL_PREFIX, jsuffix),
		basicPodLabels,
		nil,
	)
	allCurrDesc := prometheus.NewDesc(
		POD_LABEL_PREFIX+CURR_PREFIX+ALL_ENERGY_LABEL+jsuffix,
		fmt.Sprintf("%s current energy consumption (%s)", POD_LABEL_PREFIX, jsuffix),
		basicPodLabels,
		nil,
	)
	allAggrDesc := prometheus.NewDesc(
		POD_LABEL_PREFIX+AGGR_PREFIX+ALL_ENERGY_LABEL+jsuffix,
		fmt.Sprintf("%s total energy consumption (%s)", POD_LABEL_PREFIX, jsuffix),
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
	for ekey, elabel := range ENERGY_LABELS {
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
		for ekey, elabel := range ENERGY_LABELS {
			edesc := prometheus.MustNewConstMetric(
				podDesc.CurrEnergy[elabel],
				prometheus.GaugeValue,
				v.GetPrometheusEnergyValue(ekey, true),
				v.GetBasicValues()...,
			)
			ch <- edesc
		}
		// dyn curr pod energy in mJ
		edesc := prometheus.MustNewConstMetric(
			podDesc.CurrEnergy[DYN_ENERGY_LABEL],
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
		for ekey, elabel := range ENERGY_LABELS {
			edesc := prometheus.MustNewConstMetric(
				podDesc.AggrEnergy[elabel],
				prometheus.CounterValue,
				v.GetPrometheusEnergyValue(ekey, false),
				v.GetBasicValues()...,
			)
			ch <- edesc
		}
		// dyn aggr pod energy in mJ
		edesc = prometheus.MustNewConstMetric(
			podDesc.AggrEnergy[DYN_ENERGY_LABEL],
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

	// collect energy per package by RAPL mJ
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
