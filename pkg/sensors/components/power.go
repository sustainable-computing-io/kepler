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

package components

import (
	"k8s.io/klog/v2"

	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/components/source"
)

type powerInterface interface {
	// GetName() returns the name of the source / impl used for estimation
	GetName() string
	// GetAbsEnergyFromDram returns mJ in DRAM. Absolute energy is the sum of Idle + Dynamic energy.
	GetAbsEnergyFromDram() (uint64, error)
	// GetAbsEnergyFromCore returns mJ in CPU cores
	GetAbsEnergyFromCore() (uint64, error)
	// GetAbsEnergyFromUncore returns mJ not in CPU cores (i.e. iGPU)
	GetAbsEnergyFromUncore() (uint64, error)
	// GetAbsEnergyFromPackage returns mJ in CPU package
	GetAbsEnergyFromPackage() (uint64, error)
	// GetAbsEnergyFromNodeComponents returns set of mJ per RAPL components
	GetAbsEnergyFromNodeComponents() map[int]source.NodeComponentsEnergy
	// StopPower stops the collection
	StopPower()
	// IsSystemCollectionSupported returns if it is possible to use this collector
	IsSystemCollectionSupported() bool
}

var (
	powerImpl powerInterface = &source.PowerSysfs{}
	enabled                  = true
)

func InitPowerImpl() {
	sysfsImpl := &source.PowerSysfs{}
	if sysfsImpl.IsSystemCollectionSupported() /*&& false*/ {
		klog.V(1).Infoln("use sysfs to obtain power")
		powerImpl = sysfsImpl
		return
	}

	msrImpl := &source.PowerMSR{}
	if msrImpl.IsSystemCollectionSupported() && config.EnabledMSR {
		klog.V(1).Infoln("use MSR to obtain power")
		powerImpl = msrImpl
		return
	}

	apmXgeneSysfsImpl := &source.ApmXgeneSysfs{}
	if apmXgeneSysfsImpl.IsSystemCollectionSupported() {
		klog.V(1).Infoln("use Ampere Xgene sysfs to obtain power")
		powerImpl = apmXgeneSysfsImpl
		return
	}

	klog.V(1).Infoln("Unable to obtain power, use estimate method")
	estimateImpl := &source.PowerEstimate{}
	powerImpl = estimateImpl
}

func GetSourceName() string {
	return powerImpl.GetName()
}

func GetAbsEnergyFromDram() (uint64, error) {
	return powerImpl.GetAbsEnergyFromDram()
}

func GetAbsEnergyFromCore() (uint64, error) {
	return powerImpl.GetAbsEnergyFromCore()
}

func GetAbsEnergyFromUncore() (uint64, error) {
	return powerImpl.GetAbsEnergyFromUncore()
}

func GetAbsEnergyFromPackage() (uint64, error) {
	return powerImpl.GetAbsEnergyFromPackage()
}

func GetAbsEnergyFromNodeComponents() map[int]source.NodeComponentsEnergy {
	return powerImpl.GetAbsEnergyFromNodeComponents()
}

func IsSystemCollectionSupported() bool {
	return powerImpl.IsSystemCollectionSupported() && enabled
}

// SetIsSystemCollectionSupported is used to enable or disable the system power collection.
// This is used for testing purpose or to enable power estimation in system that has real-time power metrics.
func SetIsSystemCollectionSupported(enable bool) {
	enabled = enable
}

func StopPower() {
	powerImpl.StopPower()
}
