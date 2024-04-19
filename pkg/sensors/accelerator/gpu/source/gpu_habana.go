//go:build gpu && habana
// +build gpu,habana

/*
Copyright 2024.

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
	"time"

	hlml "github.com/HabanaAI/gohlml"
	"k8s.io/klog/v2"

	"github.com/sustainable-computing-io/kepler/pkg/config"
)

type GPUHabana struct {
	collectionSupported bool
	devices             map[int]Device
}

func (d *GPUHabana) GetName() string {
	return "habana"
}

func (d *GPUHabana) InitLib() error {
	return nil
}

// todo: refactor logic at invoking side, if gpu is not set?
func (d *GPUHabana) Init() error {
	ret := hlml.Initialize()
	if ret != nil {
		d.collectionSupported = false
	} else {
		d.collectionSupported = true
	}
	return ret
}

func (d *GPUHabana) Shutdown() bool {
	ret := hlml.Shutdown()
	if ret != nil {
		return false
	}
	return true
}

func (d *GPUHabana) GetAbsEnergyFromGPU() []uint32 {
	gpuEnergy := []uint32{}
	for _, device := range d.devices {
		power, ret := device.HabanaDeviceHandler.(hlml.Device).PowerUsage()
		if ret != nil {
			klog.V(2).Infof("failed to get power usage on device %v: %v\n", device, ret)
			continue
		}
		energy := uint32(uint64(power) * config.SamplePeriodSec)
		gpuEnergy = append(gpuEnergy, energy)

	}
	return gpuEnergy
}

func (d *GPUHabana) GetGpus() map[int]Device {
	count, ret := hlml.DeviceCount()
	if ret != nil {
		return nil
	}
	d.devices = make(map[int]Device, count)
	for i := 0; i < int(count); i++ {
		device, ret := hlml.DeviceHandleByIndex(uint(i))
		if ret == nil {
			d.devices[i] = Device{
				HabanaDeviceHandler: device,
				NVMLDeviceHandler:   nil,
			}
		}
	}
	return d.devices
}

func (d *GPUHabana) GetMIGInstances() map[int]map[int]Device {
	var devices map[int]map[int]Device
	return devices
}

func (n *GPUHabana) GetProcessResourceUtilizationPerDevice(device Device, since time.Duration) (map[uint32]ProcessUtilizationSample, error) {
	processAcceleratorMetrics := map[uint32]ProcessUtilizationSample{}
	return processAcceleratorMetrics, nil
}

func (d *GPUHabana) IsGPUCollectionSupported() bool {
	return d.collectionSupported
}

func (d *GPUHabana) SetGPUCollectionSupported(supported bool) {
	d.collectionSupported = supported
}
