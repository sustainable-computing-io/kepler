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

package consts

import (
	"github.com/sustainable-computing-io/kepler/pkg/config"
)

const (
	MetricsNamespace       = "kepler"
	EnergyMetricNameSuffix = "_joules_total"
	UsageMetricNameSuffix  = "_total"
	MiliJouleToJoule       = 1000
)

var (
	// Energy related metric labels
	ProcessEnergyLabels   = []string{"pid", "container_id", "vm_id", "command", "mode"}
	ContainerEnergyLabels = []string{"container_id", "pod_name", "container_name", "container_namespace", "mode"}
	VMEnergyLabels        = []string{"vm_id", "mode"}
	NodeEnergyLabels      = []string{"package", "instance", "mode"}

	// Resource utilization related metric labels
	ProcessResUtilLabels   = []string{"pid", "container_id", "vm_id", "command"}
	ContainerResUtilLabels = []string{"container_id", "pod_name", "container_name", "container_namespace"}
	VMResUtilLabels        = []string{"vm_id"}
	NodeResUtilLabels      = []string{"device", "instance"}
	GPUResUtilLabels       = []string{"gpu_id"}
)

var (
	EnergyMetricNames = []string{
		config.PKG,
		config.CORE,
		config.UNCORE,
		config.DRAM,
		config.OTHER,
		config.GPU,
		config.PLATFORM,
	}
	DynEnergyMetricNames = []string{
		config.DynEnergyInPkg,
		config.DynEnergyInCore,
		config.DynEnergyInUnCore,
		config.DynEnergyInDRAM,
		config.DynEnergyInOther,
		config.DynEnergyInGPU,
		config.DynEnergyInPlatform,
	}
	IdleEnergyMetricNames = []string{
		config.IdleEnergyInPkg,
		config.IdleEnergyInCore,
		config.IdleEnergyInUnCore,
		config.IdleEnergyInDRAM,
		config.IdleEnergyInOther,
		config.IdleEnergyInGPU,
		config.IdleEnergyInPlatform,
	}
	HCMetricNames = []string{
		config.CPUCycle,
		config.CPUInstruction,
		config.CacheMiss,
	}
	SCMetricNames = []string{
		config.CPUTime,
		config.TaskClock,
		config.PageCacheHit,
	}
	IRQMetricNames = []string{
		config.IRQNetTXLabel,
		config.IRQNetRXLabel,
		config.IRQBlockLabel,
	}
	CGroupMetricNames = []string{
		config.CgroupfsCPU,
		config.CgroupfsMemory,
		config.CgroupfsSystemCPU,
		config.CgroupfsUserCPU,
	}
	GPUMetricNames = []string{
		config.GPUComputeUtilization,
		config.GPUMemUtilization,
	}
)
