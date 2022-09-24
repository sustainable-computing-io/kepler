/*
Copyright 2022.

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

package acpi

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"k8s.io/klog/v2"
)

const (
	freqPathDir     = "/sys/devices/system/cpu/cpufreq/"
	freqPath        = "/sys/devices/system/cpu/cpufreq/policy%d/scaling_cur_freq"
	powerPath       = "/sys/class/hwmon/hwmon2/device/power%d_average"
	poolingInterval = 3000 * time.Millisecond // in seconds
	sensorIDPrefix  = "energy"
)

var (
	numCPUS int32 = int32(runtime.NumCPU())
)

// Advanced Configuration and Power Interface (APCI) makes the system hardware sensor status
// information available to the operating system via hwmon in sysfs.
type ACPI struct {
	// systemEnergy is the system accumulated energy consumption in Joule
	systemEnergy     map[string]float64 /*sensorID:value*/
	collectEnergy    bool
	cpuCoreFrequency map[int32]uint64 /*cpuID:value*/
	stopChannel      chan bool

	mu sync.Mutex
}

func NewACPIPowerMeter() *ACPI {
	acpi := &ACPI{
		systemEnergy:     map[string]float64{},
		cpuCoreFrequency: map[int32]uint64{},
		stopChannel:      make(chan bool),
	}
	if acpi.IsPowerSupported() {
		acpi.collectEnergy = true
	}
	return acpi
}

func (a *ACPI) Run() {
	go func() {
		for {
			select {
			case <-a.stopChannel:
				return
			default:
				cpuCoreFrequency := getCPUCoreFrequency()
				a.mu.Lock()
				for cpu, freq := range cpuCoreFrequency {
					// average cpu frequency
					a.cpuCoreFrequency[cpu] = (cpuCoreFrequency[cpu] + freq) / 2
				}
				a.mu.Unlock()

				if a.collectEnergy {
					if sensorPower, err := getPowerFromSensor(); err == nil {
						a.mu.Lock()
						for sensorID, power := range sensorPower {
							// energy (mJ) is equal to miliwatts*time(second)
							a.systemEnergy[sensorID] += power * float64(poolingInterval/time.Second)
						}
						a.mu.Unlock()
					} else {
						klog.Fatal(err)
					}
				}

				time.Sleep(poolingInterval)
			}
		}
	}()
}

func (a *ACPI) Stop() {
	close(a.stopChannel)
}

func (a *ACPI) GetCPUCoreFrequency() map[int32]uint64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	shallowClone := make(map[int32]uint64)
	for k, v := range a.cpuCoreFrequency {
		shallowClone[k] = v
	}
	return shallowClone
}

func getCPUCoreFrequency() map[int32]uint64 {
	files, err := os.ReadDir(freqPathDir)
	if err != nil {
		klog.Fatal(err)
	}

	ch := make(chan []uint64)
	for i := 0; i < len(files); i++ {
		go func(i uint64) {
			path := fmt.Sprintf(freqPath, i)
			data, err := os.ReadFile(path)
			if err != nil {
				return
			}
			if freq, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64); err == nil {
				ch <- []uint64{i, freq}
				return
			}
			ch <- []uint64{i, 0}
		}(uint64(i))
	}

	cpuCoreFrequency := map[int32]uint64{}
	for i := 0; i < len(files); i++ {
		select {
		case val, ok := <-ch:
			if ok {
				cpuCoreFrequency[int32(val[0])] = val[1]
			}
		case <-time.After(1 * time.Minute):
			klog.V(1).Infoln("timeout reading cpu core frequency files")
		}
	}

	return cpuCoreFrequency
}

func (a *ACPI) IsPowerSupported() bool {
	file := fmt.Sprintf(powerPath, 1)
	_, err := os.ReadFile(file)
	return err == nil
}

// GetEnergyFromHost returns the accumulated energy consumption and reset the counter
func (a *ACPI) GetEnergyFromHost() (map[string]float64, error) {
	power := map[string]float64{}
	a.mu.Lock()
	// reset counter when readed to prevent overflow
	for sensorID := range a.systemEnergy {
		power[sensorID] = a.systemEnergy[sensorID]
		a.systemEnergy[sensorID] = 0
	}
	a.mu.Unlock()
	return power, nil
}

func getPowerFromSensor() (map[string]float64, error) {
	power := map[string]float64{}

	for i := int32(1); i <= numCPUS; i++ {
		path := fmt.Sprintf(powerPath, i)
		data, err := os.ReadFile(path)
		if err != nil {
			break
		}
		// currPower is in microWatt
		currPower, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
		if err == nil {
			power[fmt.Sprintf("%s%d", sensorIDPrefix, i)] = float64(currPower) / 1000 /*miliWatts*/
		} else {
			return power, err
		}
	}

	return power, nil
}
