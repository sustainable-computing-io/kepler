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
	"runtime"

	"github.com/sustainable-computing-io/kepler/pkg/power/platform/source"
	"k8s.io/klog/v2"
)

type powerInterface interface {
	// GetEnergyFromPlatform returns mJ in DRAM
	GetEnergyFromPlatform() (map[string]float64, error)
	// StopPower stops the collection
	StopPower()
	// IsSystemCollectionSupported returns if it is possible to use this collector
	IsSystemCollectionSupported() bool
}

var (
	powerImpl powerInterface = source.NewACPIPowerMeter()
	hmcImpl                  = &source.PowerHMC{}
)

func InitPowerImpl() {
	// switch the platform power collector source to hmc if the system architecture is s390x
	// TODO: add redfish or ipmi as well.
	if runtime.GOARCH == "s390x" {
		klog.V(1).Infoln("use hmc to obtain power")
		powerImpl = hmcImpl
	}
}

func GetEnergyFromPlatform() (map[string]float64, error) {
	return powerImpl.GetEnergyFromPlatform()
}

func IsSystemCollectionSupported() bool {
	return powerImpl.IsSystemCollectionSupported()
}

func StopPower() {
	powerImpl.StopPower()
}
