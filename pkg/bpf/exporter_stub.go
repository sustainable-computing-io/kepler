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

package bpf

import "k8s.io/apimachinery/pkg/util/sets"

type stubAttacher struct{}

func NewExporter() (Exporter, error) {
	return &stubAttacher{}, nil
}

func (a *stubAttacher) SupportedMetrics() SupportedMetrics {
	return SupportedMetrics{
		HardwareCounters: sets.New[string](),
		SoftwareCounters: sets.New[string](),
	}
}

func (a *stubAttacher) Detach() {
}

func (a *stubAttacher) CollectProcesses() (processesData []ProcessBPFMetrics, err error) {
	return nil, nil
}

func (a *stubAttacher) CollectCPUFreq() (cpuFreqData map[int32]uint64, err error) {
	return nil, nil
}

func (a *stubAttacher) HardwareCountersEnabled() bool {
	return false
}
