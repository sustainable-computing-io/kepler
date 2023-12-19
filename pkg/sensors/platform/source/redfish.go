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
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/nodecred"

	"k8s.io/klog/v2"
)

// RedfishSystemModel is the struct for the system model
// this is generated via the following command:
// redfishtool Systems
type RedfishSystemModel struct {
	OdataContext string `json:"@odata.context"`
	OdataID      string `json:"@odata.id"`
	OdataType    string `json:"@odata.type"`
	Description  string `json:"Description"`
	Members      []struct {
		OdataID string `json:"@odata.id"`
	} `json:"Members"`
	MembersOdataCount int    `json:"Members@odata.count"`
	Name              string `json:"Name"`
}

// RedfishPowerModel is the struct for the power model
// this is generated via the following command:
// redfishtool raw GET /redfish/v1/Chassis/System.Embedded.1/Power#/PowerControl, where "System.Embedded.1" is the system ID from the RedfishSystemModel
// the output is then formatted via https://mholt.github.io/json-to-go/ based on sample output from https://www.dmtf.org/sites/default/files/standards/documents/DSP2046_2023.1.pdf
type RedfishPowerModel struct {
	OdataType     string          `json:"@odata.type,omitempty"`
	ID            string          `json:"Id,omitempty"`
	Name          string          `json:"Name,omitempty"`
	PowerControl  []PowerControl  `json:"PowerControl,omitempty"`
	Voltages      []Voltages      `json:"Voltages,omitempty"`
	PowerSupplies []PowerSupplies `json:"PowerSupplies,omitempty"`
	Actions       Actions         `json:"Actions,omitempty"`
	OdataID       string          `json:"@odata.id,omitempty"`
}
type PowerMetrics struct {
	IntervalInMin        int `json:"IntervalInMin,omitempty"`
	MinConsumedWatts     int `json:"MinConsumedWatts,omitempty"`
	MaxConsumedWatts     int `json:"MaxConsumedWatts,omitempty"`
	AverageConsumedWatts int `json:"AverageConsumedWatts,omitempty"`
}
type PowerLimit struct {
	LimitInWatts   int    `json:"LimitInWatts,omitempty"`
	LimitException string `json:"LimitException,omitempty"`
	CorrectionInMs int    `json:"CorrectionInMs,omitempty"`
}
type RelatedItem struct {
	OdataID string `json:"@odata.id,omitempty"`
}
type Status struct {
	State  string `json:"State,omitempty"`
	Health string `json:"Health,omitempty"`
}
type PowerControl struct {
	OdataID             string        `json:"@odata.id,omitempty"`
	MemberID            string        `json:"MemberId,omitempty"`
	Name                string        `json:"Name,omitempty"`
	PowerConsumedWatts  int           `json:"PowerConsumedWatts,omitempty"`
	PowerRequestedWatts int           `json:"PowerRequestedWatts,omitempty"`
	PowerAvailableWatts int           `json:"PowerAvailableWatts,omitempty"`
	PowerCapacityWatts  int           `json:"PowerCapacityWatts,omitempty"`
	PowerAllocatedWatts int           `json:"PowerAllocatedWatts,omitempty"`
	PowerMetrics        PowerMetrics  `json:"PowerMetrics,omitempty"`
	PowerLimit          PowerLimit    `json:"PowerLimit,omitempty"`
	RelatedItem         []RelatedItem `json:"RelatedItem,omitempty"`
	Status              Status        `json:"Status,omitempty"`
}
type Voltages struct {
	OdataID                   string        `json:"@odata.id,omitempty"`
	MemberID                  string        `json:"MemberId,omitempty"`
	Name                      string        `json:"Name,omitempty"`
	SensorNumber              int           `json:"SensorNumber,omitempty"`
	Status                    Status        `json:"Status,omitempty"`
	ReadingVolts              int           `json:"ReadingVolts,omitempty"`
	UpperThresholdNonCritical float64       `json:"UpperThresholdNonCritical,omitempty"`
	UpperThresholdCritical    int           `json:"UpperThresholdCritical,omitempty"`
	UpperThresholdFatal       int           `json:"UpperThresholdFatal,omitempty"`
	LowerThresholdNonCritical float64       `json:"LowerThresholdNonCritical,omitempty"`
	LowerThresholdCritical    int           `json:"LowerThresholdCritical,omitempty"`
	LowerThresholdFatal       int           `json:"LowerThresholdFatal,omitempty"`
	MinReadingRange           int           `json:"MinReadingRange,omitempty"`
	MaxReadingRange           int           `json:"MaxReadingRange,omitempty"`
	PhysicalContext           string        `json:"PhysicalContext,omitempty"`
	RelatedItem               []RelatedItem `json:"RelatedItem,omitempty"`
}
type InputRanges struct {
	InputType      string `json:"InputType,omitempty"`
	MinimumVoltage int    `json:"MinimumVoltage,omitempty"`
	MaximumVoltage int    `json:"MaximumVoltage,omitempty"`
	OutputWattage  int    `json:"OutputWattage,omitempty"`
}
type PowerSupplies struct {
	OdataID              string        `json:"@odata.id,omitempty"`
	MemberID             string        `json:"MemberId,omitempty"`
	Name                 string        `json:"Name,omitempty"`
	Status               Status        `json:"Status,omitempty"`
	PowerSupplyType      string        `json:"PowerSupplyType,omitempty"`
	LineInputVoltageType string        `json:"LineInputVoltageType,omitempty"`
	LineInputVoltage     int           `json:"LineInputVoltage,omitempty"`
	PowerCapacityWatts   int           `json:"PowerCapacityWatts,omitempty"`
	LastPowerOutputWatts int           `json:"LastPowerOutputWatts,omitempty"`
	Model                string        `json:"Model,omitempty"`
	Manufacturer         string        `json:"Manufacturer,omitempty"`
	FirmwareVersion      string        `json:"FirmwareVersion,omitempty"`
	SerialNumber         string        `json:"SerialNumber,omitempty"`
	PartNumber           string        `json:"PartNumber,omitempty"`
	SparePartNumber      string        `json:"SparePartNumber,omitempty"`
	InputRanges          []InputRanges `json:"InputRanges,omitempty"`
	RelatedItem          []RelatedItem `json:"RelatedItem,omitempty"`
}
type PowerPowerSupplyReset struct {
	Target string `json:"target,omitempty"`
}
type Actions struct {
	PowerPowerSupplyReset PowerPowerSupplyReset `json:"#Power.PowerSupplyReset,omitempty"`
}

// RedfishSystemPowerResult is the system power query result
type RedfishSystemPowerResult struct {
	system        string
	consumedWatts int
	timestamp     time.Time
}

// RedfishAccessInfo is the struct for the access model
type RedfishAccessInfo struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Host     string `json:"host"`
}

type RedFishClient struct {
	// systemEnergy is the system accumulated energy consumption in Joule
	accessInfo    RedfishAccessInfo
	systems       []*RedfishSystemPowerResult
	ticker        *time.Ticker
	probeInterval time.Duration
	mutex         sync.Mutex
}

func NewRedfishClient() *RedFishClient {
	credPath := config.GetRedfishCredFilePath()
	if credPath == "" {
		klog.Infof("failed to get redfish credential file path")
		return nil
	}
	if err := nodecred.InitNodeCredImpl(map[string]string{"redfish_cred_file_path": credPath}); err != nil {
		klog.Infof("%s", fmt.Sprintf("failed to initialize node credential: %v", err))
		return nil
	} else {
		klog.V(5).Infof("Initialized node credential")
		nodeName := os.Getenv("NODE_NAME")
		if nodeName == "" {
			nodeName = "localhost"
		}
		redfishCred, err := nodecred.GetNodeCredByNodeName(nodeName, "redfish")
		if err == nil {
			userName := redfishCred["redfish_username"]
			password := redfishCred["redfish_password"]
			host := redfishCred["redfish_host"]
			if userName != "" && password != "" && host != "" {
				klog.V(5).Infof("Initialized redfish credential")
				probeInterval := config.GetRedfishProbeIntervalInSeconds()
				interval := time.Duration(probeInterval) * time.Second
				redfish := &RedFishClient{
					accessInfo:    RedfishAccessInfo{Username: userName, Password: password, Host: host},
					systems:       []*RedfishSystemPowerResult{},
					probeInterval: interval,
					mutex:         sync.Mutex{},
				}
				return redfish
			}
		} else {
			klog.V(1).Infof("%s", fmt.Sprintf("failed to get node credential: %v", err))
			return nil
		}
	}
	return nil
}

func (*RedFishClient) GetName() string {
	return "redfish"
}

func (rf *RedFishClient) IsSystemCollectionSupported() bool {
	// goroutine for collecting power info from Redfish already exists
	if rf.ticker != nil {
		return true
	}

	system, err := getRedfishSystem(rf.accessInfo)

	if err != nil {
		klog.Infof("failed to get redfish system info: %v\n", err)
		return false
	}

	intervalInMin := 0
	// iterate each "Members" in the system and get the power info
	for index, member := range system.Members {
		// split the OdataID by delimiter "/" and get the system ID
		split := strings.Split(member.OdataID, "/")
		if len(split) < 2 {
			continue
		}
		id := split[len(split)-1]
		res := RedfishSystemPowerResult{}
		power, err := getRedfishPower(rf.accessInfo, id)
		if err == nil && len(power.PowerControl) > 0 {
			if index < len(rf.systems) {
				rf.systems[index].consumedWatts = power.PowerControl[0].PowerConsumedWatts
			} else {
				res.system = id
				res.consumedWatts = power.PowerControl[0].PowerConsumedWatts
				res.timestamp = time.Now()
				rf.systems = append(rf.systems, &res)
			}
			klog.V(5).Infof("power info: %+v\n", power)
			if power.PowerControl[0].PowerMetrics.IntervalInMin > intervalInMin {
				intervalInMin = power.PowerControl[0].PowerMetrics.IntervalInMin
			}
		} else {
			klog.V(5).Infof("failed to get power info: %v\n", err)
		}
	}

	// set a timer to check the power info every probeInterval seconds
	if rf.ticker == nil {
		rf.ticker = time.NewTicker(rf.probeInterval)
	}
	go func() {
		for {
			<-rf.ticker.C
			for _, system := range rf.systems {
				power, err := getRedfishPower(rf.accessInfo, system.system)
				if err == nil && len(power.PowerControl) > 0 {
					// mutex
					rf.mutex.Lock()
					klog.V(5).Infof("power info: %+v\n", power)
					system.consumedWatts = power.PowerControl[0].PowerConsumedWatts
					rf.mutex.Unlock()
				} else {
					klog.V(5).Infof("failed to get power info: %v\n", err)
				}
			}
		}
	}()
	return rf.systems != nil && len(rf.systems) > 0
}

// GetAbsEnergyFromPlatform returns the power consumption in Watt
func (rf *RedFishClient) GetAbsEnergyFromPlatform() (map[string]float64, error) {
	if rf.systems != nil {
		power := make(map[string]float64)
		for _, system := range rf.systems {
			rf.mutex.Lock()
			now := time.Now()
			// calculate the elapsed time since the last power query in seconds
			elapsed := now.Sub(system.timestamp).Seconds()
			system.timestamp = time.Now()
			klog.V(5).Infof("power info: %+v\n", system)
			power[system.system] = float64(system.consumedWatts*1000) * elapsed // convert to mW
			rf.mutex.Unlock()
		}
		return power, nil
	}
	return nil, nil
}

// StopPower stops the power collection timer
func (rf *RedFishClient) StopPower() {
	if rf != nil && rf.ticker != nil {
		rf.ticker.Stop()
	}
}
