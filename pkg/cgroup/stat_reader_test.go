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

package cgroup

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Test Read Stat Converter", func() {

	It("Test converter cgroupfs_memory_usage_bytes with memory.current", func() {
		imap := make(map[string]interface{})
		imap["memory.current"] = 100
		out := convertToStandard(imap)
		Expect(len(out)).To(Equal(1))
		v, ok := out["cgroupfs_memory_usage_bytes"]
		Expect(ok).To(Equal(true))
		Expect(v).To(Equal(100))
	})
	It("Test converter cgroupfs_memory_usage_bytes with memory.usage_in_bytes", func() {
		imap := make(map[string]interface{})
		imap["memory.usage_in_bytes"] = 100
		out := convertToStandard(imap)
		Expect(len(out)).To(Equal(1))
		v, ok := out["cgroupfs_memory_usage_bytes"]
		Expect(ok).To(Equal(true))
		Expect(v).To(Equal(100))
	})
	It("Test converter cgroupfs_kernel_memory_usage_bytes", func() {
		imap := make(map[string]interface{})
		imap["memory.kmem.usage_in_bytes"] = 100
		out := convertToStandard(imap)
		Expect(len(out)).To(Equal(1))
		v, ok := out["cgroupfs_kernel_memory_usage_bytes"]
		Expect(ok).To(Equal(true))
		Expect(v).To(Equal(100))
	})
	It("Test converter cgroupfs_tcp_memory_usage_bytes", func() {
		imap := make(map[string]interface{})
		imap["memory.kmem.tcp.usage_in_bytes"] = 100
		out := convertToStandard(imap)
		Expect(len(out)).To(Equal(1))
		v, ok := out["cgroupfs_tcp_memory_usage_bytes"]
		Expect(ok).To(Equal(true))
		Expect(v).To(Equal(100))
	})
	It("Test converter cgroupfs_cpu_usage_us with cpuacct.usage", func() {
		imap := make(map[string]interface{})
		imap["cpuacct.usage"] = uint64(100000)
		out := convertToStandard(imap)
		Expect(len(out)).To(Equal(1))
		v, ok := out["cgroupfs_cpu_usage_us"]
		Expect(ok).To(Equal(true))
		Expect(v).To(Equal(uint64(100)))
	})
	It("Test converter cgroupfs_cpu_usage_us with usage_usec", func() {
		imap := make(map[string]interface{})
		imap["usage_usec"] = 100
		out := convertToStandard(imap)
		Expect(len(out)).To(Equal(1))
		v, ok := out["cgroupfs_cpu_usage_us"]
		Expect(ok).To(Equal(true))
		Expect(v).To(Equal(100))
	})
	It("Test converter cgroupfs_system_cpu_usage_us with cpuacct.usage_sys", func() {
		imap := make(map[string]interface{})
		imap["cpuacct.usage_sys"] = uint64(100000)
		out := convertToStandard(imap)
		Expect(len(out)).To(Equal(1))
		v, ok := out["cgroupfs_system_cpu_usage_us"]
		Expect(ok).To(Equal(true))
		Expect(v).To(Equal(uint64(100)))
	})
	It("Test converter cgroupfs_system_cpu_usage_us with system_usec", func() {
		imap := make(map[string]interface{})
		imap["system_usec"] = 100
		out := convertToStandard(imap)
		Expect(len(out)).To(Equal(1))
		v, ok := out["cgroupfs_system_cpu_usage_us"]
		Expect(ok).To(Equal(true))
		Expect(v).To(Equal(100))
	})
	It("Test converter cgroupfs_user_cpu_usage_us with cpuacct.usage_user", func() {
		imap := make(map[string]interface{})
		imap["cpuacct.usage_user"] = uint64(100000)
		out := convertToStandard(imap)
		Expect(len(out)).To(Equal(1))
		v, ok := out["cgroupfs_user_cpu_usage_us"]
		Expect(ok).To(Equal(true))
		Expect(v).To(Equal(uint64(100)))
	})
	It("Test converter cgroupfs_user_cpu_usage_us with user_usec", func() {
		imap := make(map[string]interface{})
		imap["user_usec"] = 100
		out := convertToStandard(imap)
		Expect(len(out)).To(Equal(1))
		v, ok := out["cgroupfs_user_cpu_usage_us"]
		Expect(ok).To(Equal(true))
		Expect(v).To(Equal(100))
	})
	It("Test converter cgroupfs_ioread_bytes", func() {
		imap := make(map[string]interface{})
		imap["rbytes"] = 100
		out := convertToStandard(imap)
		Expect(len(out)).To(Equal(1))
		v, ok := out["cgroupfs_ioread_bytes"]
		Expect(ok).To(Equal(true))
		Expect(v).To(Equal(100))
	})
	It("Test converter cgroupfs_iowrite_bytes", func() {
		imap := make(map[string]interface{})
		imap["wbytes"] = 100
		out := convertToStandard(imap)
		Expect(len(out)).To(Equal(1))
		v, ok := out["cgroupfs_iowrite_bytes"]
		Expect(ok).To(Equal(true))
		Expect(v).To(Equal(100))
	})
	It("Test converter multiple", func() {
		imap := make(map[string]interface{})
		imap["rbytes"] = 100
		imap["wbytes"] = 100
		out := convertToStandard(imap)
		Expect(len(out)).To(Equal(2))
		v, ok := out["cgroupfs_ioread_bytes"]
		Expect(ok).To(Equal(true))
		Expect(v).To(Equal(100))
		v, ok = out["cgroupfs_iowrite_bytes"]
		Expect(ok).To(Equal(true))
		Expect(v).To(Equal(100))
	})
})
