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

package source

import (
	"os"
	"strconv"
 
	"k8s.io/klog/v2"

	"github.com/sustainable-computing-io/kepler/pkg/power/components/source"
	"github.com/zhmcclient/golang-zhmcclient/pkg/zhmcclient"
)

var hmcManager *zhmcclient.ZhmcManager

type PowerHMC struct{}

func (a *PowerHMC) GetName() string {
    return "hmc"
}

func (a *PowerHMC) StopPower() {
}

func (r *PowerHMC) GetHMCManager() *zhmcclient.ZhmcManager {
	if hmcManager == nil {
		endpoint := os.Getenv("HMC_ENDPOINT")
		username := os.Getenv("HMC_USERNAME")
		password := os.Getenv("HMC_PASSWORD")
		cacert := os.Getenv("CA_CERT")
		skipCert := os.Getenv("SKIP_CERT")
		isSkipCert, _ := strconv.ParseBool(skipCert)

		creds := &zhmcclient.Options{Username: username, Password: password, CaCert: cacert, SkipCert: isSkipCert, Trace: false}
		client, err := zhmcclient.NewClient(endpoint, creds, nil)
		if err != nil {
			klog.V(1).Infof("Error getting client connection %v", err.Message)
		}
		if client != nil {
			zhmcAPI := zhmcclient.NewManagerFromClient(client)
			hmcManager, _ := zhmcAPI.(*zhmcclient.ZhmcManager)
			return hmcManager
		}
	}
	return hmcManager
}

func (r *PowerHMC) GetEnergyFromLpar() (uint64, error) {
	hmcManager := r.GetHMCManager()
	lparURI := "api/logical-partitions/" + os.Getenv("LPAR_ID")
	props := &zhmcclient.EnergyRequestPayload{
		Range:      "last-day",
		Resolution: "fifteen-minutes",
	}
	energy, _, err := hmcManager.GetEnergyDetailsforLPAR(lparURI, props)
	if err != nil {
		klog.V(1).Infof("Error getting energy data: %v", err.Message)
	}
	klog.V(1).Infof("Get energy data successfully")
	return energy, err
}

func (r *PowerHMC) GetLiveEnergyFromLpar() (uint64, error) {
	hmcManager := r.GetHMCManager()
	lparURI := "/api/logical-partitions/" + os.Getenv("LPAR_ID")
	energy, _, err := hmcManager.GetLiveEnergyDetailsforLPAR(lparURI)
	if err != nil {
		klog.V(1).Infof("Error getting energy data: %v", err.Message)
	} else {
		klog.V(1).Infof("Get energy data successfully with power: %v", energy)
	}
	return energy, err
}

func (r *PowerHMC) IsSystemCollectionSupported() bool {
	return true
}

func (r *PowerHMC) StopPower() {
}

func (r *PowerHMC) GetEnergyFromDram() (uint64, error) {
	return 0, nil
}

func (r *PowerHMC) GetEnergyFromCore() (uint64, error) {
	return 0, nil
}

func (r *PowerHMC) GetEnergyFromUncore() (uint64, error) {
	return 0, nil
}

func (r *PowerHMC) GetEnergyFromPackage() (uint64, error) {
	return 0, nil
}

func (r *PowerHMC) GetNodeComponentsEnergy() map[int]source.NodeComponentsEnergy {
	pkgEnergy, _ := r.GetLiveEnergyFromLpar()
	pkgEnergy = pkgEnergy * 3
	coreEnergy := uint64(0)
	dramEnergy := uint64(0)
	uncoreEnergy := uint64(0)
	componentsEnergies := make(map[int]source.NodeComponentsEnergy)
	componentsEnergies[0] = source.NodeComponentsEnergy{
		Core:   coreEnergy,
		DRAM:   dramEnergy,
		Uncore: uncoreEnergy,
		Pkg:    pkgEnergy,
	}
	return componentsEnergies
}

func (r *PowerHMC) GetPlatformEnergy() map[string]float64 {
	pkgEnergy, _ := r.GetLiveEnergyFromLpar()
	platformEnergies := make(map[string]float64)
	platformEnergies[hmcSensorID] = float64(pkgEnergy) * 3
	return platformEnergies
}

func (r *PowerHMC) IsPlatformCollectionSupported() bool {
	return true
}

// GetEnergyFromHost returns the accumulated energy consumption
func (r *PowerHMC) GetAbsEnergyFromPlatform() (map[string]float64, error) {
	pkgEnergy, _ := r.GetLiveEnergyFromLpar()
	platformEnergies := make(map[string]float64)
	platformEnergies[hmcSensorID] = float64(pkgEnergy) * 3
	return platformEnergies, nil
}
