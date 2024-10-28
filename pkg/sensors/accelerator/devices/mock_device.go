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
	"time"

	"k8s.io/klog/v2"
)

var (
	mockDevice = MOCK
)

type MockDevice struct {
	mockDevice          DeviceType
	name                string
	collectionSupported bool
}

func RegisterMockDevice() {
	r := GetRegistry()
	if err := addDeviceInterface(r, mockDevice, mockDevice.String(), MockDeviceDeviceStartup); err != nil {
		klog.Errorf("couldn't register mock device %v", err)
	}
}

func MockDeviceDeviceStartup() Device {
	d := MockDevice{
		mockDevice:          mockDevice,
		name:                mockDevice.String(),
		collectionSupported: true,
	}

	return &d
}

func (d *MockDevice) Name() string {
	return d.name
}

func (d *MockDevice) DevType() DeviceType {
	return d.mockDevice
}

func (d *MockDevice) HwType() string {
	return d.mockDevice.String()
}

func (d *MockDevice) InitLib() error {
	return nil
}

func (d *MockDevice) Init() error {
	return nil
}

func (d *MockDevice) Shutdown() bool {
	GetRegistry().Unregister(d.DevType())
	return true
}

func (d *MockDevice) AbsEnergyFromDevice() []uint32 {
	return nil
}

func (d *MockDevice) DevicesByID() map[int]any {
	return nil
}

func (d *MockDevice) DevicesByName() map[string]any {
	return nil
}

func (d *MockDevice) DeviceInstances() map[int]map[int]any {
	return nil
}

func (d *MockDevice) DeviceUtilizationStats(dev any) (map[any]interface{}, error) {
	ds := make(map[any]interface{}) // Process Accelerator Metrics
	return ds, nil
}

func (d *MockDevice) ProcessResourceUtilizationPerDevice(dev any, _ time.Duration) (map[uint32]any, error) {
	processAcceleratorMetrics := map[uint32]GPUProcessUtilizationSample{}
	pam := make(map[uint32]interface{})
	processAcceleratorMetrics[0] = GPUProcessUtilizationSample{
		Pid:         0,
		TimeStamp:   uint64(time.Now().UnixNano()),
		ComputeUtil: 10,
		MemUtil:     10,
		EncUtil:     10,
		DecUtil:     10,
	}
	for k, v := range processAcceleratorMetrics {
		pam[k] = v
	}
	return pam, nil
}

func (d *MockDevice) IsDeviceCollectionSupported() bool {
	return d.collectionSupported
}

func (d *MockDevice) SetDeviceCollectionSupported(supported bool) {
	d.collectionSupported = supported
}
