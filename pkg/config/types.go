/*
Copyright 2022.

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

package config

const (
	// counter - attacher package
	CPUCycle       = "cpu_cycles"
	CPURefCycle    = "cpu_ref_cycles"
	CPUInstruction = "cpu_instructions"
	CacheMiss      = "cache_miss"

	// bpf - attacher package
	CPUTime       = "bpf_cpu_time_us"
	PageCacheHit  = "bpf_page_cache_hit"
	IRQNetTXLabel = "bpf_net_tx_irq"
	IRQNetRXLabel = "bpf_net_rx_irq"
	IRQBlockLabel = "bpf_block_irq"

	// cgroup - cgroup package
	CgroupfsMemory       = "cgroupfs_memory_usage_bytes"
	CgroupfsKernelMemory = "cgroupfs_kernel_memory_usage_bytes"
	CgroupfsTCPMemory    = "cgroupfs_tcp_memory_usage_bytes"
	CgroupfsCPU          = "cgroupfs_cpu_usage_us"
	CgroupfsSystemCPU    = "cgroupfs_system_cpu_usage_us"
	CgroupfsUserCPU      = "cgroupfs_user_cpu_usage_us"
	CgroupfsReadIO       = "cgroupfs_ioread_bytes"
	CgroupfsWriteIO      = "cgroupfs_iowrite_bytes"
	BytesReadIO          = "bytes_read"
	BytesWriteIO         = "bytes_writes"
	BlockDevicesIO       = "block_devices_used"
	// kubelet - package
	KubeletCPUUsage        = "kubelet_cpu_usage"
	KubeletMemoryUsage     = "kubelet_memory_bytes"
	KubeletContainerCPU    = "container_cpu_usage_seconds_total"
	KubeletContainerMemory = "container_memory_working_set_bytes"
	KubeletNodeCPU         = "node_cpu_usage_seconds_total"
	KubeletNodeMemory      = "node_memory_working_set_bytes"

	// system
	CPUFrequency = "avg_cpu_frequency"

	// GPU
	GPUSMUtilization  = "gpu_sm_util"
	GPUMemUtilization = "gpu_mem_util"

	// Metric suffix
	AggregatedUsageSuffix  = "total"
	AggregatedEnergySuffix = "joules_total"
)
