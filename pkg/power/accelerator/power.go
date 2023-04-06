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

package accelerator

import (
	"fmt"
	"time"

	"github.com/sustainable-computing-io/kepler/pkg/config"
	accelerator_source "github.com/sustainable-computing-io/kepler/pkg/power/accelerator/source"
	"k8s.io/klog/v2"
)

var (
	acceleratorImpl acceleratorInterface
	errLib          = fmt.Errorf("could not start accelerator collector")
)

type acceleratorInterface interface {
	// Init initizalize and start the GPU metric collector
	Init() error
	// Shutdown stops the GPU metric collector
	Shutdown() bool
	// GetGpus returns a map with gpu device
	GetGpus() []interface{}
	// GetGpuEnergyPerGPU returns a map with mJ in each gpu device
	GetGpuEnergyPerGPU() []uint32
	// GetProcessResourceUtilization returns a map of ProcessUtilizationSample where the key is the process pid
	GetProcessResourceUtilizationPerDevice(device interface{}, since time.Duration) (map[uint32]accelerator_source.ProcessUtilizationSample, error)
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
	return errLib
}

func Shutdown() bool {
	if acceleratorImpl != nil && config.EnabledGPU {
		return acceleratorImpl.Shutdown()
	}
	return true
}

func GetGpus() []interface{} {
	if acceleratorImpl != nil && config.EnabledGPU {
		return acceleratorImpl.GetGpus()
	}
	return []interface{}{}
}

func GetGpuEnergyPerGPU() []uint32 {
	if acceleratorImpl != nil && config.EnabledGPU {
		return acceleratorImpl.GetGpuEnergyPerGPU()
	}
	return []uint32{}
}

// GetProcessResourceUtilizationPerDevice tries to collect the GPU metrics.
// There is a known issue that some clusters the nvidia GPU can stop to respod and we need to start it again.
// See https://github.com/sustainable-computing-io/kepler/issues/610.
func GetProcessResourceUtilizationPerDevice(device interface{}, since time.Duration) (map[uint32]accelerator_source.ProcessUtilizationSample, error) {
	if acceleratorImpl != nil && config.EnabledGPU {
		processesUtilization, err := acceleratorImpl.GetProcessResourceUtilizationPerDevice(device, since)
		if err != nil {
			klog.Infof("Failed to collect GPU metrics, trying to initizalize again: %v\n", err)
			err = acceleratorImpl.Init()
			if err != nil {
				klog.Infof("Failed to init nvml: %v\n", err)
				return map[uint32]accelerator_source.ProcessUtilizationSample{}, err
			}
		}
		return processesUtilization, err
	}
	return map[uint32]accelerator_source.ProcessUtilizationSample{}, errLib
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
