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

	"github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator/device"
)

var (
	dummyDevice device.DeviceType
)

type Dummy struct {
	dummyDevice         device.DeviceType
	name                string
	collectionSupported bool
}

func init() {
	dummyDevice = device.DUMMY
	device.AddDeviceInterface(dummyDevice, dummyDevice.String(), DummyDeviceStartup)
}

func DummyDeviceStartup() device.DeviceInterface {
	d := Dummy{
		dummyDevice:         dummyDevice,
		name:                dummyDevice.String(),
		collectionSupported: true,
	}

	return &d
}

func (d *Dummy) Name() string {
	return d.name
}

func (d *Dummy) DevType() device.DeviceType {
	return d.dummyDevice
}
func (d *Dummy) DevTypeName() string {
	return d.name
}

func (d *Dummy) HwType() string {
	return d.dummyDevice.String()
}

func (d *Dummy) InitLib() error {
	return nil
}

func (d *Dummy) Init() error {
	return nil
}

func (d *Dummy) Shutdown() bool {
	return true
}

func (d *Dummy) AbsEnergyFromDevice() []uint32 {
	return nil
}

func (d *Dummy) DevicesByID() map[int]any {
	return nil
}

func (d *Dummy) DevicesByName() map[string]any {
	return nil
}

func (d *Dummy) DeviceInstances() map[int]map[int]any {
	return nil
}

func (d *Dummy) DeviceUtilizationStats(dev any) (map[any]interface{}, error) {
	ds := make(map[any]interface{}) // Process Accelerator Metrics
	return ds, nil
}

func (d *Dummy) ProcessResourceUtilizationPerDevice(dev any, _ time.Duration) (map[uint32]any, error) {
	processAcceleratorMetrics := map[uint32]device.GPUProcessUtilizationSample{}
	pam := make(map[uint32]interface{})
	processAcceleratorMetrics[0] = device.GPUProcessUtilizationSample{
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

func (d *Dummy) IsDeviceCollectionSupported() bool {
	return d.collectionSupported
}

func (d *Dummy) SetDeviceCollectionSupported(supported bool) {
	d.collectionSupported = supported
}
