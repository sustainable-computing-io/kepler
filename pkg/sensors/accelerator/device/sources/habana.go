//go:build habana
// +build habana

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
package sources

import (
	"time"

	hlml "github.com/HabanaAI/gohlml"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator/device"
	"k8s.io/klog/v2"

	"github.com/sustainable-computing-io/kepler/pkg/config"
)

const (
	habanaHwType = "gpu"
)

var (
	habanaAccImpl = GPUHabana{}
	habanaDevice  device.DeviceType
)

type GPUHabana struct {
	collectionSupported bool
	devices             map[int]device.GPUDevice
}

func init() {
	if err := habanaAccImpl.InitLib(); err != nil {
		klog.Infof("Error initializing Habana: %v", err)
	}
	habanaDevice = device.HABANA
	klog.Infof("Using %s to obtain processor power", habanaDevice.String())
	device.AddDeviceInterface(habanaDevice, habanaHwType, habanaDeviceStartup)
}

func habanaDeviceStartup() (device.DeviceInterface, error) {
	a := habanaAccImpl

	if err := a.Init(); err != nil {
		klog.Errorf("failed to StartupDevice: %v", err)
		return nil, err
	}

	return &a, nil
}

func (g *GPUHabana) Name() string {
	return habanaDevice.String()
}

func (g *GPUHabana) DevType() device.DeviceType {
	return habanaDevice
}

func (g *GPUHabana) DevTypeName() string {
	return habanaDevice.String()
}

func (g *GPUHabana) HwType() string {
	return habanaHwType
}

func (g *GPUHabana) InitLib() error {
	return nil
}

// todo: refactor logic at invoking side, if gpu is not set?
func (g *GPUHabana) Init() error {
	ret := hlml.Initialize()
	if ret != nil {
		klog.Error("ERROR initializing hlml")
		g.collectionSupported = false
	} else {
		klog.Info("Initialized hlml and enabling collection support")
		g.collectionSupported = true
	}
	return ret
}

func (g *GPUHabana) Shutdown() bool {
	if ret := hlml.Shutdown(); ret != nil {
		return false
	}
	return true
}

func (g *GPUHabana) AbsEnergyFromDevice() []uint32 {
	gpuEnergy := []uint32{}

	for _, device := range g.devices {
		power, ret := device.DeviceHandler.(hlml.Device).PowerUsage()
		if ret != nil {
			klog.V(2).Infof("failed to get power usage on device %v: %v\n", device, ret)
			continue
		}
		energy := uint32(uint64(power) * config.SamplePeriodSec)
		gpuEnergy = append(gpuEnergy, energy)

		dname, _ := device.DeviceHandler.(hlml.Device).Name()
		klog.V(2).Infof("AbsEnergyFromDevice power usage on device %v: %v\n", dname, gpuEnergy)
	}

	return gpuEnergy
}

func (g *GPUHabana) DevicesByID() map[int]any {
	// Get the count of available devices
	count, ret := hlml.DeviceCount()
	if ret != nil {
		// Return nil if there's an error retrieving the device count
		return nil
	}

	// Initialize the devices map with the count of devices
	devices := make(map[int]any, count)

	// Iterate through each device index to get the device handle
	for i := 0; i < int(count); i++ {
		// Get the device handle for the current index
		if h, ret := hlml.DeviceHandleByIndex(uint(i)); ret == nil {
			devices[i] = device.GPUDevice{
				DeviceHandler: h,
			}
		}
	}
	return devices
}

func (g *GPUHabana) DevicesByName() map[string]any {
	devices := make(map[string]interface{})
	return devices
}

func (g *GPUHabana) DeviceInstances() map[int]map[int]any {
	var devices map[int]map[int]interface{}
	return devices
}

func (g *GPUHabana) DeviceUtilizationStats(dev any) (map[any]interface{}, error) {
	ds := make(map[any]interface{}) // Process Accelerator Metrics
	return ds, nil
}

func (g *GPUHabana) ProcessResourceUtilizationPerDevice(dev any, since time.Duration) (map[uint32]interface{}, error) {
	pam := make(map[uint32]interface{}) // Process Accelerator Metrics
	return pam, nil
}

func (g *GPUHabana) IsDeviceCollectionSupported() bool {
	return g.collectionSupported
}

func (g *GPUHabana) SetDeviceCollectionSupported(supported bool) {
	g.collectionSupported = supported
}
