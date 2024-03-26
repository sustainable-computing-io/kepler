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

	"github.com/sustainable-computing-io/kepler/pkg/config"
)

var (
	// List of GPU identifiers for the device
	devices map[int]Device
	// bool to check if the process utilization collection is supported
	processUtilizationSupported bool = true
)

type GPUNvml struct {
	libInited           bool
	collectionSupported bool
}

func (GPUNvml) GetName() string {
	return "nvidia-nvml"
}

// Init initizalize and start the GPU metric collector
// the nvml only works if the container has support to GPU, e.g., it is using nvidia-docker2
// otherwise it will fail to load the libnvidia-ml.so.1
func (n *GPUNvml) InitLib() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("could not init nvml: %v", r)
		}
	}()
	if ret := nvml.Init(); ret != nvml.SUCCESS {
		n.collectionSupported = false
		err = fmt.Errorf("failed to init nvml. %s", nvmlErrorString(ret))
		return err
	}
	n.libInited = true
	return nil
}

func (n *GPUNvml) Init() (err error) {
	if !n.libInited {
		if err := n.InitLib(); err != nil {
			return err
		}
	}

	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		nvml.Shutdown()
		n.collectionSupported = false
		err = fmt.Errorf("failed to get nvml device count: %v", nvml.ErrorString(ret))
		return err
	}
	klog.Infof("found %d gpu devices\n", count)
	devices = make(map[int]Device, count)
	for gpuID := 0; gpuID < count; gpuID++ {
		nvmlDeviceHandler, ret := nvml.DeviceGetHandleByIndex(gpuID)
		if ret != nvml.SUCCESS {
			nvml.Shutdown()
			n.collectionSupported = false
			err = fmt.Errorf("failed to get nvml device %d: %v ", gpuID, nvml.ErrorString(ret))
			return err
		}
		name, _ := nvmlDeviceHandler.GetName()
		uuid, _ := nvmlDeviceHandler.GetUUID()
		klog.Infof("GPU %v %q %q", gpuID, name, uuid)
		device := Device{
			NVMLDeviceHandler: nvmlDeviceHandler,
			GPUID:             gpuID,
			IsMig:             false,
		}
		devices[gpuID] = device
	}
	n.collectionSupported = true
	return nil
}

// Shutdown stops the GPU metric collector
func (n *GPUNvml) Shutdown() bool {
	n.libInited = false
	return nvml.Shutdown() == nvml.SUCCESS
}

// GetGpus returns a map with gpu device
func (n *GPUNvml) GetGpus() map[int]Device {
	return devices
}

func (d *GPUNvml) GetMIGInstances() map[int]map[int]Device {
	var devices map[int]map[int]Device
	return devices
}

// GetAbsEnergyFromGPU returns a map with mJ in each gpu device
func (n *GPUNvml) GetAbsEnergyFromGPU() []uint32 {
	gpuEnergy := []uint32{}
	for _, device := range devices {
		power, ret := device.NVMLDeviceHandler.(nvml.Device).GetPowerUsage()
		if ret != nvml.SUCCESS {
			klog.V(2).Infof("failed to get power usage on device %v: %v\n", device, nvml.ErrorString(ret))
			continue
		}
		// since Kepler collects metrics at intervals of SamplePeriodSec, which is greater than 1 second, it is
		// necessary to calculate the energy consumption for the entire waiting period
		energy := uint32(uint64(power) * config.SamplePeriodSec)
		gpuEnergy = append(gpuEnergy, energy)
	}
	return gpuEnergy
}

// GetProcessResourceUtilization returns a map of ProcessUtilizationSample where the key is the process pid
//
//	ProcessUtilizationSample.SmUtil represents the process Streaming Multiprocessors - SM (3D/Compute) utilization in percentage.
//	ProcessUtilizationSample.MemUtil represents the process Frame Buffer Memory utilization Value.
func (n *GPUNvml) GetProcessResourceUtilizationPerDevice(device Device, since time.Duration) (map[uint32]ProcessUtilizationSample, error) {
	processAcceleratorMetrics := map[uint32]ProcessUtilizationSample{}
	lastUtilizationTimestamp := uint64(time.Now().Add(-1*since).UnixNano() / 1000)

	if processUtilizationSupported {
		processUtilizationSample, ret := device.NVMLDeviceHandler.(nvml.Device).GetProcessUtilization(lastUtilizationTimestamp)
		if ret != nvml.SUCCESS {
			if ret == nvml.ERROR_NOT_FOUND {
				// ignore the error if there is no process running in the GPU
				return nil, nil
			}
			processUtilizationSupported = false
		} else {
			for _, pinfo := range processUtilizationSample {
				// pid 0 means no data.
				if pinfo.Pid != 0 {
					processAcceleratorMetrics[pinfo.Pid] = ProcessUtilizationSample{
						Pid:         pinfo.Pid,
						TimeStamp:   pinfo.TimeStamp,
						ComputeUtil: pinfo.SmUtil,
						MemUtil:     pinfo.MemUtil,
						EncUtil:     pinfo.EncUtil,
						DecUtil:     pinfo.DecUtil,
					}
				}
			}
		}
	}
	if !processUtilizationSupported { // if processUtilizationSupported is false, try deviceGetMPSComputeRunningProcesses_v3 to use memory usage to ratio power usage
		config.GpuUsageMetric = config.GPUMemUtilization
		processInfo, ret := device.NVMLDeviceHandler.(nvml.Device).GetComputeRunningProcesses()
		if ret != nvml.SUCCESS {
			if ret == nvml.ERROR_NOT_FOUND {
				// ignore the error if there is no process running in the GPU
				return nil, nil
			}
			return nil, fmt.Errorf("failed to get processes' utilization on device %v: %v", device.GPUID, nvml.ErrorString(ret))
		}
		memoryInfo, ret := device.NVMLDeviceHandler.(nvml.Device).GetMemoryInfo()
		if ret != nvml.SUCCESS {
			return nil, fmt.Errorf("failed to get memory info on device %v: %v", device, nvml.ErrorString(ret))
		}
		// convert processInfo to processUtilizationSample
		for _, pinfo := range processInfo {
			// pid 0 means no data.
			if pinfo.Pid != 0 {
				processAcceleratorMetrics[pinfo.Pid] = ProcessUtilizationSample{
					Pid:     pinfo.Pid,
					MemUtil: uint32(pinfo.UsedGpuMemory * 100 / memoryInfo.Total),
				}
				klog.V(5).Infof("pid: %d, memUtil: %d gpu instance %d compute instance %d\n", pinfo.Pid, processAcceleratorMetrics[pinfo.Pid].MemUtil, pinfo.GpuInstanceId, pinfo.ComputeInstanceId)
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

func nvmlErrorString(errno nvml.Return) string {
	switch errno {
	case nvml.SUCCESS:
		return "SUCCESS"
	case nvml.ERROR_LIBRARY_NOT_FOUND:
		return "ERROR_LIBRARY_NOT_FOUND"
	}
	return fmt.Sprintf("Error %d", errno)
}
