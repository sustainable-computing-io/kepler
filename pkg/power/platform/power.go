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

	"github.com/sustainable-computing-io/kepler/pkg/power/platform/source"
	"k8s.io/klog/v2"
)

type powerInterface interface {
	// GetAbsEnergyFromPlatform returns mJ in DRAM. Absolute energy is the sum of Idle + Dynamic energy.
	GetAbsEnergyFromPlatform() (map[string]float64, error)
	// StopPower stops the collection
	StopPower()
	// IsSystemCollectionSupported returns if it is possible to use this collector
	IsSystemCollectionSupported() bool
}

var (
	powerImpl   powerInterface
	redfishImpl *source.RedFishClient
	hmcImpl     = &source.PowerHMC{}
	powerSource = "none"
	enabled     = true
)

func InitPowerImpl() {
	// switch the platform power collector source to hmc if the system architecture is s390x
	// TODO: add redfish or ipmi as well.
	if runtime.GOARCH == "s390x" {
		klog.V(1).Infoln("use hmc to obtain power")
		powerImpl = hmcImpl
		powerSource = "hmc"
	} else if redfishImpl = source.NewRedfishClient(); redfishImpl != nil && redfishImpl.IsSystemCollectionSupported() {
		klog.V(1).Infoln("use redfish to obtain power")
		powerImpl = redfishImpl
		powerSource = "redfish"
	} else if powerImpl = source.NewACPIPowerMeter(); powerImpl != nil && powerImpl.IsSystemCollectionSupported() {
		klog.V(1).Infoln("use acpi to obtain power")
		powerSource = "acpi"
	}
}

func GetPowerSource() string {
	return powerSource
}

// GetAbsEnergyFromPlatform returns the absolute energy, which is the sum of Idle + Dynamic energy.
func GetAbsEnergyFromPlatform() (map[string]float64, error) {
	if powerImpl != nil {
		return powerImpl.GetAbsEnergyFromPlatform()
	}
	return nil, fmt.Errorf("powerImpl is nil")
}

func IsSystemCollectionSupported() bool {
	if powerImpl != nil && enabled {
		return powerImpl.IsSystemCollectionSupported()
	}
	return false
}

// SetIsSystemCollectionSupported is used to enable or disable the system power collection.
// This is used for testing purpose or to enable power estimation in system that has real-time power metrics.
func SetIsSystemCollectionSupported(enable bool) {
	enabled = enable
}

func StopPower() {
	if powerImpl != nil {
		powerImpl.StopPower()
	}
}
