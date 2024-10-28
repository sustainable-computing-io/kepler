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
package devices

import (
	"errors"
	"os"
	"time"

	hlml "github.com/HabanaAI/gohlml"
	"k8s.io/klog/v2"

	"github.com/sustainable-computing-io/kepler/pkg/config"
)

const (
	habanaAccType = config.GPU
	libhlmlpath   = "/usr/lib/habanalabs/libhlml.so"
)

var (
	habanaAccImpl = gpuHabana{}
	habanaType    DeviceType
)

type gpuHabana struct {
	collectionSupported bool
	devices             map[int]interface{}
}

func habanaCheck(r *Registry) {
	if err := habanaAccImpl.InitLib(); err != nil {
		klog.V(5).Infof("Error  initializing %s: %v", habanaAccImpl.Name(), err)
		return
	}
	habanaType = HABANA
	klog.V(5).Infof("Register %s with device startup register", habanaType)
	if err := addDeviceInterface(r, habanaType, habanaAccType, habanaDeviceStartup); err == nil {
		klog.Infof("Using %s to obtain processor power", habanaAccImpl.Name())
	} else {
		klog.V(5).Infof("Error registering habana: %v", err)
	}
}

func habanaDeviceStartup() Device {
	a := habanaAccImpl

	if err := a.Init(); err != nil {
		klog.Errorf("failed to StartupDevice: %v", err)
		return nil
	}

	return &a
}

func (g *gpuHabana) Name() string {
	return habanaType.String()
}

func (g *gpuHabana) DevType() DeviceType {
	return habanaType
}

func (g *gpuHabana) HwType() string {
	return habanaAccType
}

func (g *gpuHabana) InitLib() error {
	if _, err := os.Stat(libhlmlpath); errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (g *gpuHabana) Init() error {
	ret := hlml.Initialize()
	if ret != nil {
		klog.Error("ERROR initializing hlml")
		g.collectionSupported = false
	} else {
		g.collectionSupported = true
		g.devices = g.DevicesByID()
		klog.Info("Initialized hlml and enabling collection support")
	}
	return ret
}

func (g *gpuHabana) Shutdown() bool {
	if ret := hlml.Shutdown(); ret != nil {
		return false
	}
	return true
}

func (g *gpuHabana) AbsEnergyFromDevice() []uint32 {
	gpuEnergy := []uint32{}
	for _, dev := range g.devices {
		power, ret := dev.(GPUDevice).DeviceHandler.(hlml.Device).PowerUsage()
		if ret != nil {
			klog.Errorf("failed to get power usage on device %v: %v\n", dev, ret)
			continue
		}
		energy := uint32(uint64(power) * config.SamplePeriodSec())
		gpuEnergy = append(gpuEnergy, energy)

		dname, _ := dev.(GPUDevice).DeviceHandler.(hlml.Device).Name()
		klog.V(5).Infof("AbsEnergyFromDevice power usage on device %v: %v\n", dname, gpuEnergy)
	}

	return gpuEnergy
}

func (g *gpuHabana) DevicesByID() map[int]interface{} {
	// Get the count of available devices
	count, ret := hlml.DeviceCount()
	if ret != nil {
		// Return nil if there's an error retrieving the device count
		return nil
	}

	// Initialize the devices map with the count of devices
	devices := make(map[int]interface{}, count)

	// Iterate through each device index to get the device handle
	for i := 0; i < int(count); i++ {
		// Get the device handle for the current index
		if h, ret := hlml.DeviceHandleByIndex(uint(i)); ret == nil {
			devices[i] = GPUDevice{
				DeviceHandler: h,
			}
		}
	}
	return devices
}

func (g *gpuHabana) DevicesByName() map[string]any {
	devices := make(map[string]interface{})
	return devices
}

func (g *gpuHabana) DeviceInstances() map[int]map[int]interface{} {
	var devices map[int]map[int]interface{}
	return devices
}

func (g *gpuHabana) DeviceUtilizationStats(dev any) (map[any]interface{}, error) {
	ds := make(map[any]interface{}) // Process Accelerator Metrics
	return ds, nil
}

func (g *gpuHabana) ProcessResourceUtilizationPerDevice(dev any, since time.Duration) (map[uint32]interface{}, error) {
	pam := make(map[uint32]interface{}) // Process Accelerator Metrics
	return pam, nil
}

func (g *gpuHabana) IsDeviceCollectionSupported() bool {
	return g.collectionSupported
}

func (g *gpuHabana) SetDeviceCollectionSupported(supported bool) {
	g.collectionSupported = supported
}
