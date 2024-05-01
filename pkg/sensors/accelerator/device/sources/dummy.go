//go:build gpu
// +build gpu

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

	acc "github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator/device"
)

var (
	dummyDevice = "dummy"
)

type Dummy struct {
	dummyDevice         string
	name                string
	collectionSupported bool
}

func init() {
	acc.AddDeviceInterface(dummyDevice, dummyDevice, dummyDeviceStartup)
}

func dummyDeviceStartup() (acc.AcceleratorInterface, error) {
	d := Dummy{
		dummyDevice:         dummyDevice,
		name:                dummyDevice,
		collectionSupported: false,
	}

	return &d, nil
}

func (d *Dummy) GetName() string {
	return d.name
}

func (d *Dummy) GetType() string {
	return d.dummyDevice
}

func (d *Dummy) GetHwType() string {
	return d.dummyDevice
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

func (d *Dummy) GetAbsEnergyFromDevice() []uint32 {
	return nil
}

func (d *Dummy) GetDevicesByID() map[int]any {
	return nil
}

func (d *Dummy) GetDevicesByName() map[string]any {
	return nil
}

func (d *Dummy) GetDeviceInstances() map[int]map[int]interface{} {
	return nil
}

func (d *Dummy) GetDeviceUtilizationStats(device any) (map[any]interface{}, error) {
	ds := make(map[any]interface{}) // Process Accelerator Metrics
	return ds, nil
}

func (d *Dummy) GetProcessResourceUtilizationPerDevice(device any, _ time.Duration) (map[uint32]any, error) {
	processAcceleratorMetrics := map[uint32]acc.GPUProcessUtilizationSample{}
	pam := make(map[uint32]interface{})
	processAcceleratorMetrics[0] = acc.GPUProcessUtilizationSample{
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
