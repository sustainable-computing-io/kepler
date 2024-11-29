//go:build !darwin
// +build !darwin

/*
Copyright 2021-2024

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
package devices

import (
	"errors"
	"fmt"
	"time"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"k8s.io/klog/v2"

	"github.com/sustainable-computing-io/kepler/pkg/config"
)

const (
	nvmlHwType = config.GPU
)

var (
	nvmlAccImpl = gpuNvml{}
	nvmlType    DeviceType
)

type gpuNvml struct {
	libInited                   bool
	collectionSupported         bool
	devices                     map[int]GPUDevice // List of GPU identifiers for the device
	processUtilizationSupported bool              // bool to check if the process utilization collection is supported
}

func nvmlCheck(r *Registry) {
	if err := nvml.Init(); err != nvml.SUCCESS {
		klog.V(5).Infof("Error initializing nvml: %v", nvmlErrorString(err))
		return
	}
	klog.Info("Initializing nvml Successful")
	nvmlType = NVML
	if err := addDeviceInterface(r, nvmlType, nvmlHwType, nvmlDeviceStartup); err == nil {
		klog.Infof("Using %s to obtain processor power", nvmlAccImpl.Name())
	} else {
		klog.V(5).Infof("Error registering nvml: %v", err)
	}
}

func nvmlDeviceStartup() Device {
	a := nvmlAccImpl
	if err := a.InitLib(); err != nil {
		klog.Errorf("Error initializing %s: %v", nvmlType.String(), err)
		return nil
	}
	if err := a.Init(); err != nil {
		klog.Errorf("failed to Init device: %v", err)
		return nil
	}
	klog.Infof("Using %s to obtain gpu power", nvmlType.String())
	return &a
}

func (n *gpuNvml) Name() string {
	return nvmlType.String()
}

func (n *gpuNvml) DevType() DeviceType {
	return nvmlType
}

func (n *gpuNvml) HwType() string {
	return nvmlHwType
}

func (n *gpuNvml) IsDeviceCollectionSupported() bool {
	return n.collectionSupported
}

func (n *gpuNvml) SetDeviceCollectionSupported(supported bool) {
	n.collectionSupported = supported
}

// Init initizalize and start the GPU metric collector
// the nvml only works if the container has support to GPU, e.g., it is using nvidia-docker2
// otherwise it will fail to load the libnvidia-ml.so.1
func (n *gpuNvml) InitLib() (err error) {
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

func (n *gpuNvml) Init() (err error) {
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
	n.devices = make(map[int]GPUDevice, count)
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
		dev := GPUDevice{
			DeviceHandler: nvmlDeviceHandler,
			ID:            gpuID,
			IsSubdevice:   false,
		}
		n.devices[gpuID] = dev
	}
	n.collectionSupported = true
	return nil
}

// Shutdown stops the GPU metric collector
func (n *gpuNvml) Shutdown() bool {
	n.libInited = false
	return nvml.Shutdown() == nvml.SUCCESS
}

// GetGpus returns a map with gpu device
func (n *gpuNvml) DevicesByID() map[int]interface{} {
	devices := make(map[int]interface{})
	for id, dev := range n.devices {
		devices[id] = dev
	}
	return devices
}

func (n *gpuNvml) DevicesByName() map[string]any {
	devices := make(map[string]interface{})
	return devices
}

func (n *gpuNvml) DeviceInstances() map[int]map[int]interface{} {
	var devices map[int]map[int]interface{}
	return devices
}

// GetAbsEnergyFromGPU returns a map with mJ in each gpu device
func (n *gpuNvml) AbsEnergyFromDevice() []uint32 {
	gpuEnergy := []uint32{}
	for _, dev := range n.devices {
		power, ret := dev.DeviceHandler.(nvml.Device).GetPowerUsage()
		if ret != nvml.SUCCESS {
			klog.Errorf("failed to get power usage on device %v: %v\n", dev, nvml.ErrorString(ret))
			continue
		}
		// since Kepler collects metrics at intervals of SamplePeriodSec, which is greater than 1 second, it is
		// necessary to calculate the energy consumption for the entire waiting period
		energy := uint32(uint64(power) * config.SamplePeriodSec())
		gpuEnergy = append(gpuEnergy, energy)
	}
	return gpuEnergy
}

func (n *gpuNvml) DeviceUtilizationStats(dev any) (map[any]interface{}, error) {
	ds := make(map[any]interface{}) // Process Accelerator Metrics
	return ds, nil
}

// GetProcessResourceUtilization returns a map of GPUProcessUtilizationSample where the key is the process pid
// GPUProcessUtilizationSample.SmUtil represents the process Streaming Multiprocessors - SM (3D/Compute) utilization in percentage.
// GPUProcessUtilizationSample.MemUtil represents the process Frame Buffer Memory utilization Value.
func (n *gpuNvml) ProcessResourceUtilizationPerDevice(dev any, since time.Duration) (map[uint32]any, error) {
	processAcceleratorMetrics := map[uint32]GPUProcessUtilizationSample{}
	pam := make(map[uint32]interface{})
	lastUtilizationTimestamp := uint64(time.Now().Add(-1*since).UnixNano() / 1000)

	switch d := dev.(type) {
	case GPUDevice:
		if d.DeviceHandler == nil {
			return pam, nil
		}
		if n.processUtilizationSupported {
			gpuProcessUtilizationSample, ret := d.DeviceHandler.(nvml.Device).GetProcessUtilization(lastUtilizationTimestamp)
			if ret != nvml.SUCCESS {
				if ret == nvml.ERROR_NOT_FOUND {
					// Ignore the error if there is no process running in the GPU
					return nil, nil
				}
				n.processUtilizationSupported = false
			} else {
				for _, pinfo := range gpuProcessUtilizationSample {
					// pid 0 means no data
					if pinfo.Pid != 0 {
						processAcceleratorMetrics[pinfo.Pid] = GPUProcessUtilizationSample{
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

		if !n.processUtilizationSupported { // If processUtilizationSupported is false, try deviceGetMPSComputeRunningProcesses_v3 to use memory usage to ratio power usage
			config.SetGPUUsageMetric(config.GPUMemUtilization)
			processInfo, ret := d.DeviceHandler.(nvml.Device).GetComputeRunningProcesses()
			if ret != nvml.SUCCESS {
				if ret == nvml.ERROR_NOT_FOUND {
					// Ignore the error if there is no process running in the GPU
					return nil, nil
				}
				return nil, fmt.Errorf("failed to get processes' utilization on device %v: %v", d.ID, nvml.ErrorString(ret))
			}
			memoryInfo, ret := d.DeviceHandler.(nvml.Device).GetMemoryInfo()
			if ret != nvml.SUCCESS {
				return nil, fmt.Errorf("failed to get memory info on device %v: %v", d, nvml.ErrorString(ret))
			}
			// Convert processInfo to GPUProcessUtilizationSample
			for _, pinfo := range processInfo {
				// pid 0 means no data
				if pinfo.Pid != 0 {
					processAcceleratorMetrics[pinfo.Pid] = GPUProcessUtilizationSample{
						Pid:     pinfo.Pid,
						MemUtil: uint32(pinfo.UsedGpuMemory * 100 / memoryInfo.Total),
					}
					klog.V(5).Infof("pid: %d, memUtil: %d gpu instance %d compute instance %d\n", pinfo.Pid, processAcceleratorMetrics[pinfo.Pid].MemUtil, pinfo.GpuInstanceId, pinfo.ComputeInstanceId)
				}
			}
		}

		for k, v := range processAcceleratorMetrics {
			pam[k] = v
		}

		return pam, nil
	default:
		klog.Error("expected GPUDevice but got come other type")
		return pam, errors.New("invalid device type")
	}
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
