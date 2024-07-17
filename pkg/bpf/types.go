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

const (
	// Per /sys/kernel/debug/tracing/events/irq/softirq_entry/format
	// { 0, "HI" }, { 1, "TIMER" }, { 2, "NET_TX" }, { 3, "NET_RX" }, { 4, "BLOCK" }, { 5, "IRQ_POLL" }, { 6, "TASKLET" }, { 7, "SCHED" }, { 8, "HRTIMER" }, { 9, "RCU" }

	// IRQ vector to IRQ number
	IRQNetTX = 2
	IRQNetRX = 3
	IRQBlock = 4
)

type ProcessMetrics = keplerProcessMetricsT

type Exporter interface {
	SupportedMetrics() SupportedMetrics
	Detach()
	CollectProcesses() ([]ProcessMetrics, error)
}

type SupportedMetrics struct {
	HardwareCounters sets.Set[string]
	SoftwareCounters sets.Set[string]
}
