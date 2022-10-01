//go:build gpu
// +build gpu

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

package gpu

import (
	"fmt"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"k8s.io/klog/v2"
)

var (
	devices []nvml.Device
)

type pidMem struct {
	pid uint32
	mem uint64
}

func Init() error {
	if ret := nvml.Init(); ret != nvml.SUCCESS {
		return fmt.Errorf("failed to init nvml: %v", nvml.ErrorString(ret))
	}
	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		nvml.Shutdown()
		return fmt.Errorf("failed to get nvml device count: %v", nvml.ErrorString(ret))
	}
	klog.V(1).Infof("found %d gpu devices\n", count)
	devices = make([]nvml.Device, count)
	for i := 0; i < count; i++ {
		device, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			nvml.Shutdown()
			return fmt.Errorf("failed to get nvml device %d: %v ", i, nvml.ErrorString(ret))
		}
		devices[i] = device
	}
	return nil
}

func Shutdown() bool {
	return nvml.Shutdown() == nvml.SUCCESS
}

func GetGpuEnergy() []uint32 {
	e := make([]uint32, len(devices))
	for i, device := range devices {
		power, ret := device.GetPowerUsage()
		if ret != nvml.SUCCESS {
			klog.V(2).Infof("failed to get power usage on device %v: %v\n", device, nvml.ErrorString(ret))
			continue
		}
		e[i] = power
	}
	return e
}

func GetCurrGpuEnergyPerPid() (map[uint32]float64, error) {
	m := make(map[uint32]float64)

	for _, device := range devices {
		power, ret := device.GetPowerUsage()
		if ret != nvml.SUCCESS {
			klog.V(2).Infof("failed to get power usage on device %v: %v\n", device, nvml.ErrorString(ret))
			continue
		}
		pids, ret := device.GetComputeRunningProcesses()
		if ret != nvml.SUCCESS {
			klog.V(2).Infof("failed to get compute processes on device %v: %v", device, nvml.ErrorString(ret))
			continue
		}
		totalMem := uint64(0)
		pm := make([]pidMem, len(pids))
		// get used memory of each pid
		for i, pid := range pids {
			pm[i].pid = pid.Pid
			pm[i].mem = pid.UsedGpuMemory
			totalMem += pm[i].mem
		}
		// use per pid used memory/total used memory to estimate per pid energy
		for _, p := range pm {
			klog.V(5).Infof("pid %v power %v total mem %v mem %v\n", p.pid, power, totalMem, p.mem)
			m[p.pid] = float64(uint64(power) * p.mem / totalMem)
		}
	}
	return m, nil
}
