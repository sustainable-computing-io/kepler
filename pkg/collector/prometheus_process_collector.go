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

package collector

import (
	"strconv"
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/sustainable-computing-io/kepler/pkg/bpfassets/attacher"
	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"
	"github.com/sustainable-computing-io/kepler/pkg/config"
)

type processDesc struct {
	// Energy (counter)
	processCoreJoulesTotal            *prometheus.Desc
	processUncoreJoulesTotal          *prometheus.Desc
	processDramJoulesTotal            *prometheus.Desc
	processPackageJoulesTotal         *prometheus.Desc
	processOtherComponentsJoulesTotal *prometheus.Desc
	processGPUJoulesTotal             *prometheus.Desc
	processJoulesTotal                *prometheus.Desc

	// Hardware Counters (counter)
	processCPUCyclesTotal *prometheus.Desc
	processCPUInstrTotal  *prometheus.Desc
	processCacheMissTotal *prometheus.Desc

	// Additional metrics (gauge)
	processCPUTime *prometheus.Desc

	// IRQ metrics
	processNetTxIRQTotal *prometheus.Desc
	processNetRxIRQTotal *prometheus.Desc
	processBlockIRQTotal *prometheus.Desc
}

// describeProcess is called by Describe to implement the prometheus.Collector interface
func (p *PrometheusCollector) describeProcess(ch chan<- *prometheus.Desc) {
	// process Energy (counter)
	ch <- p.processDesc.processCoreJoulesTotal
	ch <- p.processDesc.processUncoreJoulesTotal
	ch <- p.processDesc.processDramJoulesTotal
	ch <- p.processDesc.processPackageJoulesTotal
	ch <- p.processDesc.processOtherComponentsJoulesTotal
	if config.EnabledGPU {
		ch <- p.processDesc.processGPUJoulesTotal
	}
	ch <- p.processDesc.processJoulesTotal

	// process Hardware Counters (counter)
	if collector_metric.CPUHardwareCounterEnabled {
		ch <- p.processDesc.processCPUCyclesTotal
		ch <- p.processDesc.processCPUInstrTotal
		ch <- p.processDesc.processCacheMissTotal
	}

	if config.ExposeIRQCounterMetrics {
		ch <- p.processDesc.processNetTxIRQTotal
		ch <- p.processDesc.processNetRxIRQTotal
		ch <- p.processDesc.processBlockIRQTotal
	}
}

func (p *PrometheusCollector) newprocessMetrics() {
	// Energy (counter)
	processCoreJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "process", "core_joules_total"),
		"Aggregated RAPL value in core in joules",
		[]string{"pid", "command", "mode"}, nil,
	)
	processUncoreJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "process", "uncore_joules_total"),
		"Aggregated RAPL value in uncore in joules",
		[]string{"pid", "command", "mode"}, nil,
	)
	processDramJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "process", "dram_joules_total"),
		"Aggregated RAPL value in dram in joules",
		[]string{"pid", "command", "mode"}, nil,
	)
	processPackageJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "process", "package_joules_total"),
		"Aggregated RAPL value in package (socket) in joules",
		[]string{"pid", "command", "mode"}, nil,
	)
	processOtherComponentsJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "process", "other_host_components_joules_total"),
		"Aggregated value in other host components (platform - package - dram) in joules",
		[]string{"pid", "command", "mode"}, nil,
	)
	processGPUJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "process", "gpu_joules_total"),
		"Aggregated GPU value in joules",
		[]string{"pid", "command", "mode"}, nil,
	)
	processJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "process", "joules_total"),
		"Aggregated RAPL Package + Uncore + DRAM + GPU + other host components (platform - package - dram) in joules",
		[]string{"pid", "command", "mode"}, nil,
	)

	// Hardware Counters (counter)
	processCPUCyclesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "process", "cpu_cycles_total"),
		"Aggregated CPU cycle value",
		[]string{"pid", "command"}, nil,
	)
	processCPUInstrTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "process", "cpu_instructions_total"),
		"Aggregated CPU instruction value",
		[]string{"pid", "command"}, nil,
	)
	processCacheMissTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "process", "cache_miss_total"),
		"Aggregated cache miss value",
		[]string{"pid", "command"}, nil,
	)
	// Additional metrics (gauge)
	processCPUTime := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "process", "cpu_cpu_time_us"),
		"Aggregated CPU time",
		[]string{"pid", "command"}, nil)

	// network irq metrics
	processNetTxIRQTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "process", "bpf_net_tx_irq_total"),
		"Aggregated network tx irq value obtained from BPF",
		[]string{"pid", "command"}, nil,
	)
	processNetRxIRQTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "process", "bpf_net_rx_irq_total"),
		"Aggregated network rx irq value obtained from BPF",
		[]string{"pid", "command"}, nil,
	)
	processBlockIRQTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "process", "bpf_block_irq_total"),
		"Aggregated block irq value obtained from BPF",
		[]string{"pid", "command"}, nil,
	)

	p.processDesc = &processDesc{
		processCoreJoulesTotal:            processCoreJoulesTotal,
		processUncoreJoulesTotal:          processUncoreJoulesTotal,
		processDramJoulesTotal:            processDramJoulesTotal,
		processPackageJoulesTotal:         processPackageJoulesTotal,
		processOtherComponentsJoulesTotal: processOtherComponentsJoulesTotal,
		processGPUJoulesTotal:             processGPUJoulesTotal,
		processJoulesTotal:                processJoulesTotal,
		processCPUCyclesTotal:             processCPUCyclesTotal,
		processCPUInstrTotal:              processCPUInstrTotal,
		processCacheMissTotal:             processCacheMissTotal,
		processCPUTime:                    processCPUTime,
		processNetTxIRQTotal:              processNetTxIRQTotal,
		processNetRxIRQTotal:              processNetRxIRQTotal,
		processBlockIRQTotal:              processBlockIRQTotal,
	}
}

// updateProcessMetrics send process metrics to prometheus
func (p *PrometheusCollector) updateProcessMetrics(wg *sync.WaitGroup, ch chan<- prometheus.Metric) {
	const commandLenLimit = 10
	for pid, process := range *p.ProcessMetrics {
		wg.Add(1)
		go func(pid uint64, process *collector_metric.ProcessMetrics) {
			defer wg.Done()
			processCommand := process.Command
			if len(processCommand) > commandLenLimit {
				processCommand = process.Command[:commandLenLimit]
			}
			pidStr := strconv.FormatUint(pid, 10)
			ch <- prometheus.MustNewConstMetric(
				p.processDesc.processCPUTime,
				prometheus.CounterValue,
				float64(process.CPUTime.Aggr),
				pidStr, processCommand,
			)
			ch <- prometheus.MustNewConstMetric(
				p.processDesc.processCoreJoulesTotal,
				prometheus.CounterValue,
				float64(process.DynEnergyInCore.Aggr)/miliJouleToJoule,
				pidStr, processCommand, "dynamic",
			)
			ch <- prometheus.MustNewConstMetric(
				p.processDesc.processCoreJoulesTotal,
				prometheus.CounterValue,
				float64(process.IdleEnergyInCore.Aggr)/miliJouleToJoule,
				pidStr, processCommand, "idle",
			)
			ch <- prometheus.MustNewConstMetric(
				p.processDesc.processUncoreJoulesTotal,
				prometheus.CounterValue,
				float64(process.DynEnergyInUncore.Aggr)/miliJouleToJoule,
				pidStr, processCommand, "dynamic",
			)
			ch <- prometheus.MustNewConstMetric(
				p.processDesc.processUncoreJoulesTotal,
				prometheus.CounterValue,
				float64(process.IdleEnergyInUncore.Aggr)/miliJouleToJoule,
				pidStr, processCommand, "idle",
			)
			ch <- prometheus.MustNewConstMetric(
				p.processDesc.processDramJoulesTotal,
				prometheus.CounterValue,
				float64(process.DynEnergyInDRAM.Aggr)/miliJouleToJoule,
				pidStr, processCommand, "dynamic",
			)
			ch <- prometheus.MustNewConstMetric(
				p.processDesc.processDramJoulesTotal,
				prometheus.CounterValue,
				float64(process.IdleEnergyInDRAM.Aggr)/miliJouleToJoule,
				pidStr, processCommand, "idle",
			)
			ch <- prometheus.MustNewConstMetric(
				p.processDesc.processPackageJoulesTotal,
				prometheus.CounterValue,
				float64(process.DynEnergyInPkg.Aggr)/miliJouleToJoule,
				pidStr, processCommand, "dynamic",
			)
			ch <- prometheus.MustNewConstMetric(
				p.processDesc.processPackageJoulesTotal,
				prometheus.CounterValue,
				float64(process.IdleEnergyInPkg.Aggr)/miliJouleToJoule,
				pidStr, processCommand, "idle",
			)
			ch <- prometheus.MustNewConstMetric(
				p.processDesc.processOtherComponentsJoulesTotal,
				prometheus.CounterValue,
				float64(process.DynEnergyInOther.Aggr)/miliJouleToJoule,
				pidStr, processCommand, "dynamic",
			)
			ch <- prometheus.MustNewConstMetric(
				p.processDesc.processOtherComponentsJoulesTotal,
				prometheus.CounterValue,
				float64(process.IdleEnergyInOther.Aggr)/miliJouleToJoule,
				pidStr, processCommand, "idle",
			)
			if config.EnabledGPU {
				ch <- prometheus.MustNewConstMetric(
					p.processDesc.processGPUJoulesTotal,
					prometheus.CounterValue,
					float64(process.DynEnergyInGPU.Aggr)/miliJouleToJoule,
					pidStr, processCommand, "dynamic",
				)
				ch <- prometheus.MustNewConstMetric(
					p.processDesc.processGPUJoulesTotal,
					prometheus.CounterValue,
					float64(process.IdleEnergyInGPU.Aggr)/miliJouleToJoule,
					pidStr, processCommand, "idle",
				)
			}
			ch <- prometheus.MustNewConstMetric(
				p.processDesc.processJoulesTotal,
				prometheus.CounterValue,
				(float64(process.DynEnergyInPkg.Aggr)/miliJouleToJoule +
					float64(process.DynEnergyInUncore.Aggr)/miliJouleToJoule +
					float64(process.DynEnergyInDRAM.Aggr)/miliJouleToJoule +
					float64(process.DynEnergyInGPU.Aggr)/miliJouleToJoule +
					float64(process.DynEnergyInOther.Aggr)/miliJouleToJoule),
				pidStr, processCommand, "dynamic",
			)
			ch <- prometheus.MustNewConstMetric(
				p.processDesc.processJoulesTotal,
				prometheus.CounterValue,
				(float64(process.IdleEnergyInPkg.Aggr)/miliJouleToJoule +
					float64(process.IdleEnergyInUncore.Aggr)/miliJouleToJoule +
					float64(process.IdleEnergyInDRAM.Aggr)/miliJouleToJoule +
					float64(process.IdleEnergyInGPU.Aggr)/miliJouleToJoule +
					float64(process.IdleEnergyInOther.Aggr)/miliJouleToJoule),
				pidStr, processCommand, "idle",
			)
			if collector_metric.CPUHardwareCounterEnabled {
				if process.CounterStats[attacher.CPUCycleLabel] != nil {
					ch <- prometheus.MustNewConstMetric(
						p.processDesc.processCPUCyclesTotal,
						prometheus.CounterValue,
						float64(process.CounterStats[attacher.CPUCycleLabel].Aggr),
						pidStr, processCommand,
					)
				}
				if process.CounterStats[attacher.CPUInstructionLabel] != nil {
					ch <- prometheus.MustNewConstMetric(
						p.processDesc.processCPUInstrTotal,
						prometheus.CounterValue,
						float64(process.CounterStats[attacher.CPUInstructionLabel].Aggr),
						pidStr, processCommand,
					)
				}
				if process.CounterStats[attacher.CacheMissLabel] != nil {
					ch <- prometheus.MustNewConstMetric(
						p.processDesc.processCacheMissTotal,
						prometheus.CounterValue,
						float64(process.CounterStats[attacher.CacheMissLabel].Aggr),
						pidStr, processCommand,
					)
				}
			}
			if config.ExposeIRQCounterMetrics {
				ch <- prometheus.MustNewConstMetric(
					p.processDesc.processNetTxIRQTotal,
					prometheus.CounterValue,
					float64(process.SoftIRQCount[attacher.IRQNetTX].Aggr),
					pidStr, processCommand,
				)
				ch <- prometheus.MustNewConstMetric(
					p.processDesc.processNetRxIRQTotal,
					prometheus.CounterValue,
					float64(process.SoftIRQCount[attacher.IRQNetRX].Aggr),
					pidStr, processCommand,
				)
				ch <- prometheus.MustNewConstMetric(
					p.processDesc.processBlockIRQTotal,
					prometheus.CounterValue,
					float64(process.SoftIRQCount[attacher.IRQBlock].Aggr),
					pidStr, processCommand,
				)
			}
		}(pid, process)
	}
}
