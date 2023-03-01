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
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"k8s.io/klog/v2"
)

const (
	freqPathDir         = "/sys/devices/system/cpu/cpufreq/"
	freqPath            = "/sys/devices/system/cpu/cpufreq/policy%d/scaling_cur_freq"
	hwmonPowerPath      = "/sys/class/hwmon/hwmon2/device/"
	acpiPowerPath       = "/sys/devices/LNXSYSTM:00"
	acpiPowerFilePrefix = "power"
	acpiPowerFileSuffix = "_average"
	poolingInterval     = 3000 * time.Millisecond // in seconds
	sensorIDPrefix      = "energy"
)

var (
	numCPUS   int32 = int32(runtime.NumCPU())
	powerPath       = hwmonPowerPath
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
		klog.V(5).Infof("Using the HWMON power meter path: %s\n", powerPath)
	} else {
		// if the acpi power_average file is not in the hwmon path, try to find the acpi path
		powerPath = findACPIPowerPath()
		if powerPath != "" {
			acpi.collectEnergy = true
			klog.V(5).Infof("Using the ACPI power meter path: %s\n", powerPath)
		} else {
			klog.Infoln("Could not find any ACPI power meter path. Is it a VM?")
		}
	}
	return acpi
}

func findACPIPowerPath() string {
	err := filepath.WalkDir(acpiPowerPath, func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && (info.Name() == "power" ||
			strings.Contains(info.Name(), "INTL") ||
			strings.Contains(info.Name(), "PNP") ||
			strings.Contains(info.Name(), "input") ||
			strings.Contains(info.Name(), "device:") ||
			strings.Contains(info.Name(), "wakeup")) {
			return filepath.SkipDir
		}
		if !info.IsDir() && strings.Contains(info.Name(), "_average") {
			powerPath = path[:(len(path) - len(info.Name()))]
		}
		return nil
	})
	if err != nil {
		klog.V(3).Infof("Could not find any ACPI power meter path: %v\n", err)
		return ""
	}
	return powerPath
}

func (a *ACPI) Run(isEBPFEnabled bool) {
	go func() {
		for {
			select {
			case <-a.stopChannel:
				return
			default:
				// in the case we cannot collect the cpu frequency using EBPF
				if !isEBPFEnabled {
					cpuCoreFrequency := getCPUCoreFrequency()
					a.mu.Lock()
					for cpu, freq := range cpuCoreFrequency {
						// average cpu frequency
						a.cpuCoreFrequency[cpu] = (cpuCoreFrequency[cpu] + freq) / 2
					}
					a.mu.Unlock()
				}

				if a.collectEnergy {
					if sensorPower, err := getPowerFromSensor(); err == nil {
						a.mu.Lock()
						for sensorID, power := range sensorPower {
							// energy (mJ) is equal to miliwatts*time(second)
							a.systemEnergy[sensorID] += power * float64(poolingInterval/time.Second)
						}
						a.mu.Unlock()
					} else {
						// There is a kernel bug that does not allow us to collect metrics in /sys/devices/LNXSYSTM:00/device:00/ACPI000D:00/power1_average
						// More info is here: https://www.suse.com/support/kb/doc/?id=000017865
						// Therefore, when we cannot read the powerPath, we stop the collection.
						klog.Infof("Disabling the ACPI power meter collection. This might be related to a kernel bug.\n")
						a.collectEnergy = false
					}
				}

				// stop the gorotime if there is nothing to do
				if (isEBPFEnabled) && (!a.collectEnergy) {
					return
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
	cpuCoreFrequency := map[int32]uint64{}
	if err != nil {
		klog.Warning(err)
		return cpuCoreFrequency
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
	// we do not use fmt.Sprintf because it is expensive in the performance standpoint
	file := powerPath + acpiPowerFilePrefix + "1" + acpiPowerFileSuffix
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
		path := powerPath + acpiPowerFilePrefix + strconv.Itoa(int(i)) + acpiPowerFileSuffix
		data, err := os.ReadFile(path)
		if err != nil {
			break
		}
		// currPower is in microWatt
		currPower, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
		if err == nil {
			power[sensorIDPrefix+strconv.Itoa(int(i))] = float64(currPower) / 1000 /*miliWatts*/
		} else {
			return power, err
		}
	}

	return power, nil
}
