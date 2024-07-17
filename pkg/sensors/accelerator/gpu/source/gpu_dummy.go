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
	"time"
)

type GPUDummy struct {
	collectionSupported bool
}

func (d *GPUDummy) GetName() string {
	return "dummy"
}

func (d *GPUDummy) InitLib() error {
	return nil
}

// todo: refactor logic at invoking side, if gpu is not set?
func (d *GPUDummy) Init() error {
	d.collectionSupported = false
	return nil
}

func (d *GPUDummy) Shutdown() bool {
	return true
}

func (d *GPUDummy) GetAbsEnergyFromGPU() []uint32 {
	return []uint32{}
}

func (d *GPUDummy) GetGpus() map[int]Device {
	var devices map[int]Device
	return devices
}

func (d *GPUDummy) GetMIGInstances() map[int]map[int]Device {
	var devices map[int]map[int]Device
	return devices
}

func (n *GPUDummy) GetProcessResourceUtilizationPerDevice(device Device, since time.Duration) (map[uint32]ProcessUtilizationSample, error) {
	processAcceleratorMetrics := map[uint32]ProcessUtilizationSample{}
	processAcceleratorMetrics[0] = ProcessUtilizationSample{
		Pid:         0,
		TimeStamp:   uint64(time.Now().UnixNano()),
		ComputeUtil: 10,
		MemUtil:     10,
		EncUtil:     10,
		DecUtil:     10,
	}
	return processAcceleratorMetrics, nil
}

func (d *GPUDummy) IsGPUCollectionSupported() bool {
	return d.collectionSupported
}

func (d *GPUDummy) SetGPUCollectionSupported(supported bool) {
	d.collectionSupported = supported
}
