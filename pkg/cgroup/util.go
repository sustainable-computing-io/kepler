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

package cgroup

const (
	UNDEFINED_SOFTIRQ = "UNDEFINED"
)

var (
	// based /sys/kernel/debug/tracing/events/irq/softirq_entry/format
	irqName = []string{"SoftIRQ_HI" /* 0 */, "SoftIRQ_TIMER", "SoftIRQ_NET_TX", "SoftIRQ_NET_RX", "SoftIRQ_BLOCK",
		"SoftIRQ_IRQ_POLL", "SoftIRQ_TASKLET", "SoftIRQ_SCHED", "SoftIRQ_HRTIMER", "SoftIRQ_RCU" /* 9 */}
)

func VecToName(vec uint32) string {
	if vec < uint32(len(irqName)) {
		return irqName[vec]
	}
	return UNDEFINED_SOFTIRQ
}

func IsIRQProcess(name string) bool {
	for _, v := range irqName {
		if v == name {
			return true
		}
	}
	return false
}
