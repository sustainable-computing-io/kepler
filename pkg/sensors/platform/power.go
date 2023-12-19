/*
Copyright 2023.

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

package platform

import (
	"fmt"
	"runtime"

	"github.com/sustainable-computing-io/kepler/pkg/sensors/platform/source"
	"k8s.io/klog/v2"
)

type powerInterface interface {
	// GetName() returns the name of the platform power source
	GetName() string
	// GetAbsEnergyFromPlatform returns mJ in DRAM. Absolute energy is the sum of Idle + Dynamic energy.
	GetAbsEnergyFromPlatform() (map[string]float64, error)
	// StopPower stops the collection
	StopPower()
	// IsSystemCollectionSupported returns if it is possible to use this collector
	IsSystemCollectionSupported() bool
}

// dummy satisfies the powerInterface and can be used as the default NOP source
type dummy struct {
}

func (dummy) GetName() string {
	return "none"
}

func (dummy) IsSystemCollectionSupported() bool {
	return false
}
func (dummy) StopPower() {
}

func (dummy) GetAbsEnergyFromPlatform() (map[string]float64, error) {
	return nil, fmt.Errorf("dummy power source")
}

var (
	powerImpl powerInterface = &dummy{}
	enabled                  = true
)

func InitPowerImpl() {
	// switch the platform power collector source to hmc if the system architecture is s390x
	// TODO: add redfish or ipmi as well.
	if runtime.GOARCH == "s390x" {
		powerImpl = &source.PowerHMC{}
	} else if redfish := source.NewRedfishClient(); redfish != nil && redfish.IsSystemCollectionSupported() {
		powerImpl = redfish
	} else if acpi := source.NewACPIPowerMeter(); acpi != nil && acpi.CollectEnergy {
		powerImpl = acpi
	}

	klog.V(1).Infof("using %s to obtain power", powerImpl.GetName())
}

func GetSourceName() string {
	return powerImpl.GetName()
}

// GetAbsEnergyFromPlatform returns the absolute energy, which is the sum of Idle + Dynamic energy.
func GetAbsEnergyFromPlatform() (map[string]float64, error) {
	return powerImpl.GetAbsEnergyFromPlatform()
}

func IsSystemCollectionSupported() bool {
	if !enabled {
		return false
	}
	return powerImpl.IsSystemCollectionSupported()
}

// SetIsSystemCollectionSupported is used to enable or disable the system power collection.
// This is used for testing purpose or to enable power estimation in system that has real-time power metrics.
func SetIsSystemCollectionSupported(enable bool) {
	enabled = enable
}

func StopPower() {
	powerImpl.StopPower()
}
