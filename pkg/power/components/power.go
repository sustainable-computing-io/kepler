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
	"runtime"

	"k8s.io/klog/v2"

	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/power/components/source"
)

type powerInterface interface {
	// GetEnergyFromDram returns mJ in DRAM
	GetEnergyFromDram() (uint64, error)
	// GetEnergyFromCore returns mJ in CPU cores
	GetEnergyFromCore() (uint64, error)
	// GetEnergyFromUncore returns mJ not in CPU cores (i.e. iGPU)
	GetEnergyFromUncore() (uint64, error)
	// GetEnergyFromPackage returns mJ in CPU package
	GetEnergyFromPackage() (uint64, error)
	// GetNodeComponentsEnergy returns set of mJ per RAPL components
	GetNodeComponentsEnergy() map[int]source.NodeComponentsEnergy
	// StopPower stops the collection
	StopPower()
	// IsSystemCollectionSupported returns if it is possible to use this collector
	IsSystemCollectionSupported() bool
}

var (
	estimateImpl                     = &source.PowerEstimate{}
	sysfsImpl                        = &source.PowerSysfs{}
	msrImpl                          = &source.PowerMSR{}
	apmXgeneSysfsImpl                = &source.ApmXgeneSysfs{}
	hmcImpl                          = &source.PowerHMC{}
	powerImpl         powerInterface = sysfsImpl
)

func initPowerImpls390x() {
	if hmcImpl.IsSystemCollectionSupported() {
		klog.V(1).Infoln("use hmc to obtain power")
		powerImpl = hmcImpl
	} else {
		klog.V(1).Infoln("Not able to obtain power, use estimate method")
		powerImpl = estimateImpl
	}
}

func initPowerImpl() {
	if sysfsImpl.IsSystemCollectionSupported() /*&& false*/ {
		klog.V(1).Infoln("use sysfs to obtain power")
		powerImpl = sysfsImpl
	} else {
		if msrImpl.IsSystemCollectionSupported() && config.EnabledMSR {
			klog.V(1).Infoln("use MSR to obtain power")
			powerImpl = msrImpl
		} else {
			if apmXgeneSysfsImpl.IsSystemCollectionSupported() {
				klog.V(1).Infoln("use Ampere Xgene sysfs to obtain power")
				powerImpl = apmXgeneSysfsImpl
			} else {
				klog.V(1).Infoln("Not able to obtain power, use estimate method")
				powerImpl = estimateImpl
			}
		}
	}
}

func InitPowerImpl() {
	if runtime.GOARCH == "s390x" {
		initPowerImpls390x()
	} else {
		initPowerImpl()
	}
}

func GetEnergyFromDram() (uint64, error) {
	return powerImpl.GetEnergyFromDram()
}

func GetEnergyFromCore() (uint64, error) {
	return powerImpl.GetEnergyFromCore()
}

func GetEnergyFromUncore() (uint64, error) {
	return powerImpl.GetEnergyFromUncore()
}

func GetEnergyFromPackage() (uint64, error) {
	return powerImpl.GetEnergyFromPackage()
}

func GetNodeComponentsEnergy() map[int]source.NodeComponentsEnergy {
	return powerImpl.GetNodeComponentsEnergy()
}

func IsSystemCollectionSupported() bool {
	return powerImpl.IsSystemCollectionSupported()
}

func StopPower() {
	powerImpl.StopPower()
}
