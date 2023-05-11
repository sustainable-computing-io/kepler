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
package source

import (
	"fmt"
	"time"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"k8s.io/klog/v2"
)

var (
	// List of GPU identifiers for the device
	devices []interface{}
)

type GPUNvml struct {
	collectionSupported bool
}

// Init initizalize and start the GPU metric collector
// the nvml only works if the container has support to GPU, e.g., it is using nvidia-docker2
// otherwise it will fail to load the libnvidia-ml.so.1
func (n *GPUNvml) Init() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("could not init nvml: %v", r)
		}
	}()
	if ret := nvml.Init(); ret != nvml.SUCCESS {
		n.collectionSupported = false
		err = fmt.Errorf("failed to init nvml: %v", nvml.ErrorString(ret))
		return err
	}

	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		nvml.Shutdown()
		n.collectionSupported = false
		err = fmt.Errorf("failed to get nvml device count: %v", nvml.ErrorString(ret))
		return err
	}
	klog.Infof("found %d gpu devices\n", count)
	devices = make([]interface{}, count)
	for i := 0; i < count; i++ {
		device, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			nvml.Shutdown()
			n.collectionSupported = false
			err = fmt.Errorf("failed to get nvml device %d: %v ", i, nvml.ErrorString(ret))
			return err
		}
		name, _ := device.GetName()
		klog.Infoln("GPU", i, name)
		devices[i] = device
	}
	n.collectionSupported = true
	return nil
}

// Shutdown stops the GPU metric collector
func (n *GPUNvml) Shutdown() bool {
	return nvml.Shutdown() == nvml.SUCCESS
}

// GetGpus returns a map with gpu device
func (n *GPUNvml) GetGpus() []interface{} {
	return devices
}

// GetGpuEnergyPerGPU returns a map with mJ in each gpu device
func (n *GPUNvml) GetGpuEnergyPerGPU() []uint32 {
	gpuEnergy := []uint32{}
	for _, device := range devices {
		power, ret := device.(nvml.Device).GetPowerUsage()
		if ret != nvml.SUCCESS {
			klog.V(2).Infof("failed to get power usage on device %v: %v\n", device, nvml.ErrorString(ret))
			continue
		}
		gpuEnergy = append(gpuEnergy, power)
	}
	return gpuEnergy
}

// GetProcessResourceUtilization returns a map of ProcessUtilizationSample where the key is the process pid
//
//	ProcessUtilizationSample.SmUtil represents the process Streaming Multiprocessors - SM (3D/Compute) utilization in percentage.
//	ProcessUtilizationSample.MemUtil represents the process Frame Buffer Memory utilization Value.
func (n *GPUNvml) GetProcessResourceUtilizationPerDevice(device interface{}, since time.Duration) (map[uint32]ProcessUtilizationSample, error) {
	processAcceleratorMetrics := map[uint32]ProcessUtilizationSample{}
	lastUtilizationTimestamp := uint64(time.Now().Add(-1*since).UnixNano() / 1000)

	processUtilizationSample, ret := device.(nvml.Device).GetProcessUtilization(lastUtilizationTimestamp)
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("failed to get processes' utilization on device %v: %v", device, nvml.ErrorString(ret))
	}

	for _, pinfo := range processUtilizationSample {
		// pid 0 means no data.
		if pinfo.Pid != 0 {
			processAcceleratorMetrics[pinfo.Pid] = ProcessUtilizationSample{
				Pid:       pinfo.Pid,
				TimeStamp: pinfo.TimeStamp,
				SmUtil:    pinfo.SmUtil,
				MemUtil:   pinfo.MemUtil,
				EncUtil:   pinfo.EncUtil,
				DecUtil:   pinfo.DecUtil,
			}
		}
	}

	return processAcceleratorMetrics, nil
}

func (n *GPUNvml) IsGPUCollectionSupported() bool {
	return n.collectionSupported
}

func (n *GPUNvml) SetGPUCollectionSupported(supported bool) {
	n.collectionSupported = supported
}
