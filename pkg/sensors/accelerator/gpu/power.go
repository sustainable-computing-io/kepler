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
	"time"

	"github.com/sustainable-computing-io/kepler/pkg/config"
	gpu_source "github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator/gpu/source"
	"k8s.io/klog/v2"
)

var (
	acceleratorImpl acceleratorInterface
	errLib          = fmt.Errorf("could not start accelerator-gpu collector")
)

type acceleratorInterface interface {
	// GetName returns the name of the collector
	GetName() string

	// Init the external library loading, if any.
	InitLib() error
	// Init initizalize and start the GPU metric collector
	Init() error
	// Shutdown stops the GPU metric collector
	Shutdown() bool
	// GetGpus returns a map with gpu device
	GetGpus() map[int]gpu_source.Device
	// GetMIGInstances returns a map with mig instances of each GPU
	GetMIGInstances() map[int]map[int]gpu_source.Device
	// GetAbsEnergyFromGPU returns a map with mJ in each gpu device. Absolute energy is the sum of Idle + Dynamic energy.
	GetAbsEnergyFromGPU() []uint32
	// GetProcessResourceUtilization returns a map of ProcessUtilizationSample where the key is the process pid
	GetProcessResourceUtilizationPerDevice(device gpu_source.Device, since time.Duration) (map[uint32]gpu_source.ProcessUtilizationSample, error)
	// IsGPUCollectionSupported returns if it is possible to use this collector
	IsGPUCollectionSupported() bool
	// SetGPUCollectionSupported manually set if it is possible to use this collector. This is for testing purpose only.
	SetGPUCollectionSupported(bool)
}

// Init() only returns the erro regarding if the gpu collector was suceffully initialized or not
// The gpu.go file has an init function that starts and configures the gpu collector
// However this file is only included in the build if kepler is run with gpus support.
// This is necessary because nvidia libraries are not available on all systems
func Init() error {
	return acceleratorImpl.Init()
}

func Shutdown() bool {
	if acceleratorImpl != nil && config.EnabledGPU {
		return acceleratorImpl.Shutdown()
	}
	return true
}

func GetGpus() map[int]gpu_source.Device {
	if acceleratorImpl != nil && config.EnabledGPU {
		return acceleratorImpl.GetGpus()
	}
	return map[int]gpu_source.Device{}
}

func GetMIGInstances() map[int]map[int]gpu_source.Device {
	if acceleratorImpl != nil && config.EnabledGPU {
		return acceleratorImpl.GetMIGInstances()
	}
	return map[int]map[int]gpu_source.Device{}
}

func GetAbsEnergyFromGPU() []uint32 {
	if acceleratorImpl != nil && config.EnabledGPU {
		return acceleratorImpl.GetAbsEnergyFromGPU()
	}
	return []uint32{}
}

// GetProcessResourceUtilizationPerDevice tries to collect the GPU metrics.
// There is a known issue that some clusters the nvidia GPU can stop to respod and we need to start it again.
// See https://github.com/sustainable-computing-io/kepler/issues/610.
func GetProcessResourceUtilizationPerDevice(device gpu_source.Device, since time.Duration) (map[uint32]gpu_source.ProcessUtilizationSample, error) {
	if acceleratorImpl != nil && config.EnabledGPU {
		processesUtilization, err := acceleratorImpl.GetProcessResourceUtilizationPerDevice(device, since)
		if err != nil {
			klog.Infof("Failed to collect GPU metrics, trying to initizalize again: %v\n", err)
			err = acceleratorImpl.Init()
			if err != nil {
				klog.Infof("Failed to init nvml: %v\n", err)
				return map[uint32]gpu_source.ProcessUtilizationSample{}, err
			}
		}
		return processesUtilization, err
	}
	return map[uint32]gpu_source.ProcessUtilizationSample{}, errLib
}

func IsGPUCollectionSupported() bool {
	if acceleratorImpl != nil && config.EnabledGPU {
		return acceleratorImpl.IsGPUCollectionSupported()
	}
	return false
}

func SetGPUCollectionSupported(supported bool) {
	if acceleratorImpl != nil && config.EnabledGPU {
		acceleratorImpl.SetGPUCollectionSupported(supported)
	}
}
