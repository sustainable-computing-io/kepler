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
//nolint:dupl // should be refactor with process collector
package collector

import (
	"strconv"
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/sustainable-computing-io/kepler/pkg/bpfassets/attacher"
	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"
	"github.com/sustainable-computing-io/kepler/pkg/config"
)

type vmDesc struct {
	// Energy (counter)
	vmCoreJoulesTotal            *prometheus.Desc
	vmUncoreJoulesTotal          *prometheus.Desc
	vmDramJoulesTotal            *prometheus.Desc
	vmPackageJoulesTotal         *prometheus.Desc
	vmOtherComponentsJoulesTotal *prometheus.Desc
	vmGPUJoulesTotal             *prometheus.Desc
	vmJoulesTotal                *prometheus.Desc

	// Hardware Counters (counter)
	vmCPUCyclesTotal *prometheus.Desc
	vmCPUInstrTotal  *prometheus.Desc
	vmCacheMissTotal *prometheus.Desc

	// Additional metrics (gauge)
	vmCPUTime *prometheus.Desc

	// IRQ metrics
	vmNetTxIRQTotal *prometheus.Desc
	vmNetRxIRQTotal *prometheus.Desc
	vmBlockIRQTotal *prometheus.Desc
}

// describevm is called by Describe to implement the prometheus.Collector interface
func (p *PrometheusCollector) describeVM(ch chan<- *prometheus.Desc) {
	// vm Energy (counter)
	ch <- p.vmDesc.vmCoreJoulesTotal
	ch <- p.vmDesc.vmUncoreJoulesTotal
	ch <- p.vmDesc.vmDramJoulesTotal
	ch <- p.vmDesc.vmPackageJoulesTotal
	ch <- p.vmDesc.vmOtherComponentsJoulesTotal
	if config.EnabledGPU {
		ch <- p.vmDesc.vmGPUJoulesTotal
	}
	ch <- p.vmDesc.vmJoulesTotal

	// vm Hardware Counters (counter)
	if collector_metric.CPUHardwareCounterEnabled {
		ch <- p.vmDesc.vmCPUCyclesTotal
		ch <- p.vmDesc.vmCPUInstrTotal
		ch <- p.vmDesc.vmCacheMissTotal
	}

	if config.ExposeIRQCounterMetrics {
		ch <- p.vmDesc.vmNetTxIRQTotal
		ch <- p.vmDesc.vmNetRxIRQTotal
		ch <- p.vmDesc.vmBlockIRQTotal
	}
}

func (p *PrometheusCollector) newVMMetrics() {
	// Energy (counter)
	vmCoreJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "vm", "core_joules_total"),
		"Aggregated RAPL value in core in joules",
		[]string{"pid", "name", "mode"}, nil,
	)
	vmUncoreJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "vm", "uncore_joules_total"),
		"Aggregated RAPL value in uncore in joules",
		[]string{"pid", "name", "mode"}, nil,
	)
	vmDramJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "vm", "dram_joules_total"),
		"Aggregated RAPL value in dram in joules",
		[]string{"pid", "name", "mode"}, nil,
	)
	vmPackageJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "vm", "package_joules_total"),
		"Aggregated RAPL value in package (socket) in joules",
		[]string{"pid", "name", "mode"}, nil,
	)
	vmOtherComponentsJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "vm", "other_host_components_joules_total"),
		"Aggregated value in other host components (platform - package - dram) in joules",
		[]string{"pid", "name", "mode"}, nil,
	)
	vmGPUJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "vm", "gpu_joules_total"),
		"Aggregated GPU value in joules",
		[]string{"pid", "name", "mode"}, nil,
	)
	vmJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "vm", "joules_total"),
		"Aggregated RAPL Package + Uncore + DRAM + GPU + other host components (platform - package - dram) in joules",
		[]string{"pid", "name", "mode"}, nil,
	)

	// Hardware Counters (counter)
	vmCPUCyclesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "vm", "cpu_cycles_total"),
		"Aggregated CPU cycle value",
		[]string{"pid", "name"}, nil,
	)
	vmCPUInstrTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "vm", "cpu_instructions_total"),
		"Aggregated CPU instruction value",
		[]string{"pid", "name"}, nil,
	)
	vmCacheMissTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "vm", "cache_miss_total"),
		"Aggregated cache miss value",
		[]string{"pid", "name"}, nil,
	)
	// Additional metrics (gauge)
	vmCPUTime := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "vm", "cpu_cpu_time_us"),
		"Aggregated CPU time",
		[]string{"pid", "name"}, nil)

	// network irq metrics
	vmNetTxIRQTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "vm", "bpf_net_tx_irq_total"),
		"Aggregated network tx irq value obtained from BPF",
		[]string{"pid", "name"}, nil,
	)
	vmNetRxIRQTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "vm", "bpf_net_rx_irq_total"),
		"Aggregated network rx irq value obtained from BPF",
		[]string{"pid", "name"}, nil,
	)
	vmBlockIRQTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "vm", "bpf_block_irq_total"),
		"Aggregated block irq value obtained from BPF",
		[]string{"pid", "name"}, nil,
	)

	p.vmDesc = &vmDesc{
		vmCoreJoulesTotal:            vmCoreJoulesTotal,
		vmUncoreJoulesTotal:          vmUncoreJoulesTotal,
		vmDramJoulesTotal:            vmDramJoulesTotal,
		vmPackageJoulesTotal:         vmPackageJoulesTotal,
		vmOtherComponentsJoulesTotal: vmOtherComponentsJoulesTotal,
		vmGPUJoulesTotal:             vmGPUJoulesTotal,
		vmJoulesTotal:                vmJoulesTotal,
		vmCPUCyclesTotal:             vmCPUCyclesTotal,
		vmCPUInstrTotal:              vmCPUInstrTotal,
		vmCacheMissTotal:             vmCacheMissTotal,
		vmCPUTime:                    vmCPUTime,
		vmNetTxIRQTotal:              vmNetTxIRQTotal,
		vmNetRxIRQTotal:              vmNetRxIRQTotal,
		vmBlockIRQTotal:              vmBlockIRQTotal,
	}
}

// updatevmMetrics send vm metrics to prometheus
func (p *PrometheusCollector) updateVMMetrics(wg *sync.WaitGroup, ch chan<- prometheus.Metric) {
	// "Instance Name" in openstack are 17 characters strings
	const nameLenLimit = 17
	for pid, vm := range *p.VMMetrics {
		wg.Add(1)
		go func(pid uint64, vm *collector_metric.VMMetrics) {
			defer wg.Done()
			vmName := vm.Name
			if len(vmName) > nameLenLimit {
				vmName = vm.Name[:nameLenLimit]
			}
			pidStr := strconv.FormatUint(pid, 10)
			if vm.BPFStats[config.CPUTime] != nil {
				ch <- prometheus.MustNewConstMetric(
					p.vmDesc.vmCPUTime,
					prometheus.CounterValue,
					float64(vm.BPFStats[config.CPUTime].Aggr),
					pidStr, vmName,
				)
			}
			ch <- prometheus.MustNewConstMetric(
				p.vmDesc.vmCoreJoulesTotal,
				prometheus.CounterValue,
				float64(vm.DynEnergyInCore.Aggr)/miliJouleToJoule,
				pidStr, vmName, "dynamic",
			)
			ch <- prometheus.MustNewConstMetric(
				p.vmDesc.vmCoreJoulesTotal,
				prometheus.CounterValue,
				float64(vm.IdleEnergyInCore.Aggr)/miliJouleToJoule,
				pidStr, vmName, "idle",
			)
			ch <- prometheus.MustNewConstMetric(
				p.vmDesc.vmUncoreJoulesTotal,
				prometheus.CounterValue,
				float64(vm.DynEnergyInUncore.Aggr)/miliJouleToJoule,
				pidStr, vmName, "dynamic",
			)
			ch <- prometheus.MustNewConstMetric(
				p.vmDesc.vmUncoreJoulesTotal,
				prometheus.CounterValue,
				float64(vm.IdleEnergyInUncore.Aggr)/miliJouleToJoule,
				pidStr, vmName, "idle",
			)
			ch <- prometheus.MustNewConstMetric(
				p.vmDesc.vmDramJoulesTotal,
				prometheus.CounterValue,
				float64(vm.DynEnergyInDRAM.Aggr)/miliJouleToJoule,
				pidStr, vmName, "dynamic",
			)
			ch <- prometheus.MustNewConstMetric(
				p.vmDesc.vmDramJoulesTotal,
				prometheus.CounterValue,
				float64(vm.IdleEnergyInDRAM.Aggr)/miliJouleToJoule,
				pidStr, vmName, "idle",
			)
			ch <- prometheus.MustNewConstMetric(
				p.vmDesc.vmPackageJoulesTotal,
				prometheus.CounterValue,
				float64(vm.DynEnergyInPkg.Aggr)/miliJouleToJoule,
				pidStr, vmName, "dynamic",
			)
			ch <- prometheus.MustNewConstMetric(
				p.vmDesc.vmPackageJoulesTotal,
				prometheus.CounterValue,
				float64(vm.IdleEnergyInPkg.Aggr)/miliJouleToJoule,
				pidStr, vmName, "idle",
			)
			ch <- prometheus.MustNewConstMetric(
				p.vmDesc.vmOtherComponentsJoulesTotal,
				prometheus.CounterValue,
				float64(vm.DynEnergyInOther.Aggr)/miliJouleToJoule,
				pidStr, vmName, "dynamic",
			)
			ch <- prometheus.MustNewConstMetric(
				p.vmDesc.vmOtherComponentsJoulesTotal,
				prometheus.CounterValue,
				float64(vm.IdleEnergyInOther.Aggr)/miliJouleToJoule,
				pidStr, vmName, "idle",
			)
			if config.EnabledGPU {
				ch <- prometheus.MustNewConstMetric(
					p.vmDesc.vmGPUJoulesTotal,
					prometheus.CounterValue,
					float64(vm.DynEnergyInGPU.Aggr)/miliJouleToJoule,
					pidStr, vmName, "dynamic",
				)
				ch <- prometheus.MustNewConstMetric(
					p.vmDesc.vmGPUJoulesTotal,
					prometheus.CounterValue,
					float64(vm.IdleEnergyInGPU.Aggr)/miliJouleToJoule,
					pidStr, vmName, "idle",
				)
			}
			ch <- prometheus.MustNewConstMetric(
				p.vmDesc.vmJoulesTotal,
				prometheus.CounterValue,
				(float64(vm.DynEnergyInPkg.Aggr)/miliJouleToJoule +
					float64(vm.DynEnergyInUncore.Aggr)/miliJouleToJoule +
					float64(vm.DynEnergyInDRAM.Aggr)/miliJouleToJoule +
					float64(vm.DynEnergyInGPU.Aggr)/miliJouleToJoule +
					float64(vm.DynEnergyInOther.Aggr)/miliJouleToJoule),
				pidStr, vmName, "dynamic",
			)
			ch <- prometheus.MustNewConstMetric(
				p.vmDesc.vmJoulesTotal,
				prometheus.CounterValue,
				(float64(vm.IdleEnergyInPkg.Aggr)/miliJouleToJoule +
					float64(vm.IdleEnergyInUncore.Aggr)/miliJouleToJoule +
					float64(vm.IdleEnergyInDRAM.Aggr)/miliJouleToJoule +
					float64(vm.IdleEnergyInGPU.Aggr)/miliJouleToJoule +
					float64(vm.IdleEnergyInOther.Aggr)/miliJouleToJoule),
				pidStr, vmName, "idle",
			)
			if collector_metric.CPUHardwareCounterEnabled {
				if vm.BPFStats[attacher.CPUCycleLabel] != nil {
					ch <- prometheus.MustNewConstMetric(
						p.vmDesc.vmCPUCyclesTotal,
						prometheus.CounterValue,
						float64(vm.BPFStats[attacher.CPUCycleLabel].Aggr),
						pidStr, vmName,
					)
				}
				if vm.BPFStats[attacher.CPUInstructionLabel] != nil {
					ch <- prometheus.MustNewConstMetric(
						p.vmDesc.vmCPUInstrTotal,
						prometheus.CounterValue,
						float64(vm.BPFStats[attacher.CPUInstructionLabel].Aggr),
						pidStr, vmName,
					)
				}
				if vm.BPFStats[attacher.CacheMissLabel] != nil {
					ch <- prometheus.MustNewConstMetric(
						p.vmDesc.vmCacheMissTotal,
						prometheus.CounterValue,
						float64(vm.BPFStats[attacher.CacheMissLabel].Aggr),
						pidStr, vmName,
					)
				}
			}
			if config.ExposeIRQCounterMetrics {
				if vm.BPFStats[config.IRQNetTXLabel] != nil {
					ch <- prometheus.MustNewConstMetric(
						p.vmDesc.vmNetTxIRQTotal,
						prometheus.CounterValue,
						float64(vm.BPFStats[config.IRQNetTXLabel].Aggr),
						pidStr, vmName,
					)
				}
				if vm.BPFStats[config.IRQNetRXLabel] != nil {
					ch <- prometheus.MustNewConstMetric(
						p.vmDesc.vmNetRxIRQTotal,
						prometheus.CounterValue,
						float64(vm.BPFStats[config.IRQNetRXLabel].Aggr),
						pidStr, vmName,
					)
				}
				if vm.BPFStats[config.IRQBlockLabel] != nil {
					ch <- prometheus.MustNewConstMetric(
						p.vmDesc.vmBlockIRQTotal,
						prometheus.CounterValue,
						float64(vm.BPFStats[config.IRQBlockLabel].Aggr),
						pidStr, vmName,
					)
				}
			}
		}(pid, vm)
	}
}
