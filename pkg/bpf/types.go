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

import (
	"k8s.io/apimachinery/pkg/util/sets"
)

type Exporter interface {
	SupportedMetrics() SupportedMetrics
	Detach()
	CollectProcesses() (ProcessMetricsCollection, error)
	Start(<-chan struct{}) error
}

type ProcessMetrics struct {
	CGroupID        uint64
	Pid             uint64
	ProcessRunTime  uint64
	CPUCyles        uint64
	CPUInstructions uint64
	CacheMiss       uint64
	PageCacheHit    uint64
	NetTxIRQ        uint64
	NetRxIRQ        uint64
	NetBlockIRQ     uint64
}

type ProcessMetricsCollection struct {
	Metrics   []ProcessMetrics
	FreedPIDs []int
}

type SupportedMetrics struct {
	HardwareCounters sets.Set[string]
	SoftwareCounters sets.Set[string]
}
