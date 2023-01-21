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
	UndefinedSoftIRQ = "UNDEFINED"
)

var (
	// based /sys/kernel/debug/tracing/events/irq/entry/format
	irqName = []string{"HI" /* 0 */, "TIMER", "NET_TX", "NET_RX", "BLOCK",
		"IRQ_POLL", "TASKLET", "SCHED", "HRTIMER", "RCU" /* 9 */}
)

func VecToName(vec uint32) string {
	if vec < uint32(len(irqName)) {
		return irqName[vec]
	}
	return UndefinedSoftIRQ
}

func IsIRQProcess(name string) bool {
	for _, v := range irqName {
		if v == name {
			return true
		}
	}
	return false
}
