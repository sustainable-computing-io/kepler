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
	CORE      = "core"
	DRAM      = "dram"
	UNCORE    = "uncore"
	PKG       = "package"
	GPU       = "gpu"
	OTHER     = "other"
	PLATFORM  = "platform"
	FREQUENCY = "frequency"

	// counter - attacher package
	CPUCycle       = "cpu_cycles"
	CPURefCycle    = "cpu_ref_cycles"
	CPUInstruction = "cpu_instructions"
	CacheMiss      = "cache_miss"
	TaskClock      = "task_clock_ms"

	// bpf - attacher package
	CPUTime       = "bpf_cpu_time_ms"
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

	// system
	CPUFrequency = "avg_cpu_frequency"

	// NVIDIA GPU
	GPUComputeUtilization = "gpu_compute_util"
	GPUMemUtilization     = "gpu_mem_util"

	// Intel QuickAssist Technology (QAT)
	// TODO: test if different request has different energy consumption.
	QATUtilization = "qta_sample_cnt"

	// Energy Metrics
	// Absolute energy and power
	AbsEnergyInCore     = "abs_energy_in_core"
	AbsEnergyInDRAM     = "abs_energy_in_dram"
	AbsEnergyInUnCore   = "abs_energy_in_uncore"
	AbsEnergyInPkg      = "abs_energy_in_pkg"
	AbsEnergyInGPU      = "abs_energy_in_gpu"
	AbsEnergyInOther    = "abs_energy_in_other"
	AbsEnergyInPlatform = "abs_energy_in_platform"
	// Dynamic energy and power
	DynEnergyInCore     = "dyn_energy_in_core"
	DynEnergyInDRAM     = "dyn_energy_in_dram"
	DynEnergyInUnCore   = "dyn_energy_in_uncore"
	DynEnergyInPkg      = "dyn_energy_in_pkg"
	DynEnergyInGPU      = "dyn_energy_in_gpu"
	DynEnergyInOther    = "dyn_energy_in_other"
	DynEnergyInPlatform = "dyn_energy_in_platform"
	// Idle energy and power
	IdleEnergyInCore     = "idle_energy_in_core"
	IdleEnergyInDRAM     = "idle_energy_in_dram"
	IdleEnergyInUnCore   = "idle_energy_in_uncore"
	IdleEnergyInPkg      = "idle_energy_in_pkg"
	IdleEnergyInGPU      = "idle_energy_in_gpu"
	IdleEnergyInOther    = "idle_energy_in_other"
	IdleEnergyInPlatform = "idle_energy_in_platform"
)
