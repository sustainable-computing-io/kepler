//go:build !linux || (linux && !libbpf)
// +build !linux linux,!libbpf

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

import "github.com/sustainable-computing-io/kepler/pkg/config"

const (
	CPUCycleLabel       = config.CPUCycle
	CPURefCycleLabel    = config.CPURefCycle
	CPUInstructionLabel = config.CPUInstruction
	CacheMissLabel      = config.CacheMiss
	TaskClockLabel      = config.TaskClock

	IRQNetTX = 2
	IRQNetRX = 3
	IRQBlock = 4

	TableProcessName = "processes"
	TableCPUFreqName = "cpu_freq_array"
	MapSize          = 10240
	CPUNumSize       = 128
)

var (
	HardwareCountersEnabled = true
	BpfPerfArrayPrefix      = "_event_reader"
	PerfEvents              = map[string][]int{}
	SoftIRQEvents           = []string{config.IRQNetTXLabel, config.IRQNetRXLabel, config.IRQBlockLabel}
)

func GetEnabledBPFHWCounters() []string {
	return []string{}
}

func GetEnabledBPFSWCounters() []string {
	return []string{}
}

func Attach() (interface{}, error) {
	return nil, nil
}

func Detach() {
}

func CollectProcesses() (processesData []ProcessBPFMetrics, err error) {
	return nil, nil
}

func CollectCPUFreq() (cpuFreqData map[int32]uint64, err error) {
	return nil, nil
}
