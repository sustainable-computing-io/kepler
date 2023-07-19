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

package attacher

/*
 temporary placeholder till PR resolved
 https://github.com/iovisor/gobpf/pull/310
*/

import (
	"encoding/binary"

	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/utils"

	"k8s.io/klog/v2"
)

type perfCounter struct {
	EvType   int
	EvConfig int
	enabled  bool
}

const (
	TableProcessName = "processes"
	TableCPUFreqName = "cpu_freq_array"
	MapSize          = 10240
	CPUNumSize       = 128
)

var (
	Counters                = getCounters()
	HardwareCountersEnabled = true
	BpfPerfArrayPrefix      = "_hc_reader"

	PerfEvents = map[string][]int{}
	ByteOrder  binary.ByteOrder
)

// must be in sync with bpf program
type ProcessBPFMetrics struct {
	CGroupID       uint64
	PID            uint64
	ProcessRunTime uint64
	CPUCycles      uint64
	CPUInstr       uint64
	CacheMisses    uint64
	VecNR          [config.MaxIRQ]uint16 // irq counter, 10 is the max number of irq vectors
	Command        [16]byte
}

func init() {
	ByteOrder = utils.DetermineHostByteOrder()
}

func getCounters() map[string]perfCounter {
	if config.UseLibBPFAttacher {
		return bccCounters
	} else {
		return libbpfCounters
	}
}

func GetEnabledHWCounters() []string {
	var metrics []string
	klog.V(5).Infof("hardeware counter metrics config %t", config.ExposeHardwareCounterMetrics)
	if !config.ExposeHardwareCounterMetrics {
		klog.V(5).Info("hardeware counter metrics not enabled")
		return metrics
	}

	for metric, counter := range Counters {
		if counter.enabled {
			metrics = append(metrics, metric)
		}
	}
	return metrics
}

func GetEnabledBPFCounters() []string {
	var metrics []string
	metrics = append(metrics, config.CPUTime)

	klog.V(5).Infof("irq counter metrics config %t", config.ExposeIRQCounterMetrics)
	if !config.ExposeIRQCounterMetrics {
		klog.V(5).Info("irq counter metrics not enabled")
		return metrics
	}
	metrics = append(metrics, []string{config.IRQNetTXLabel, config.IRQNetRXLabel, config.IRQBlockLabel}...)
	return metrics
}

func CollectProcesses() (processesData []ProcessBPFMetrics, err error) {
	if config.UseLibBPFAttacher {
		return libbpfCollectProcess()
	}
	return bccCollectProcess()
}

func CollectCPUFreq() (cpuFreqData map[int32]uint64, err error) {
	if config.UseLibBPFAttacher {
		return libbpfCollectFreq()
	}
	return bccCollectFreq()
}

func Attach() (interface{}, error) {
	klog.Infof("LibbpfBuilt: %v, BccBuilt: %v", LibbpfBuilt, BccBuilt)
	if !BccBuilt && LibbpfBuilt {
		config.UseLibBPFAttacher = true
	}
	if config.UseLibBPFAttacher && LibbpfBuilt {
		m, err := attachLibbpfModule()
		if err == nil {
			return m, nil
		}
		// err != nil, disable and try using bcc
		detachLibbpfModule()
		config.UseLibBPFAttacher = false
		klog.Infof("failed to attach bpf with libbpf: %v", err)
	}
	return attachBccModule()
}

func Detach() {
	if config.UseLibBPFAttacher {
		detachLibbpfModule()
	}
	detachBccModule()
}
