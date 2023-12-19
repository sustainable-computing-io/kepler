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

package source

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/sustainable-computing-io/kepler/pkg/config"
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
	numCPUS int32 = int32(runtime.NumCPU())
)

// Advanced Configuration and Power Interface (APCI) makes the system hardware sensor status
// information available to the operating system via hwmon in sysfs.
type ACPI struct {
	CollectEnergy bool
	powerPath     string
}

func NewACPIPowerMeter() *ACPI {
	acpi := &ACPI{powerPath: hwmonPowerPath}
	if acpi.IsHWMONCollectionSupported() {
		acpi.CollectEnergy = true
		klog.V(5).Infof("Using the HWMON power meter path: %s\n", acpi.powerPath)
	} else {
		// if the acpi power_average file is not in the hwmon path, try to find the acpi path
		acpi.powerPath = findACPIPowerPath()
		if acpi.powerPath != "" {
			acpi.CollectEnergy = true
			klog.V(5).Infof("Using the ACPI power meter path: %s\n", acpi.powerPath)
		} else {
			klog.Infoln("Could not find any ACPI power meter path. Is it a VM?")
		}
	}

	return acpi
}

func findACPIPowerPath() string {
	var powerPath string
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

func (ACPI) GetName() string {
	return "acpi"
}

func (a *ACPI) StopPower() {
}

func (a *ACPI) GetCPUCoreFrequency() map[int32]uint64 {
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

func (a *ACPI) IsSystemCollectionSupported() bool {
	return a.CollectEnergy
}

func (a *ACPI) IsHWMONCollectionSupported() bool {
	// we do not use fmt.Sprintf because it is expensive in the performance standpoint
	file := a.powerPath + acpiPowerFilePrefix + "1" + acpiPowerFileSuffix
	_, err := os.ReadFile(file)
	return err == nil
}

// GetEnergyFromHost returns the accumulated energy consumption
func (a *ACPI) GetAbsEnergyFromPlatform() (map[string]float64, error) {
	power := map[string]float64{}

	for i := int32(1); i <= numCPUS; i++ {
		path := a.powerPath + acpiPowerFilePrefix + strconv.Itoa(int(i)) + acpiPowerFileSuffix
		data, err := os.ReadFile(path)
		if err != nil {
			break
		}
		// currPower is in microWatt
		currPower, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
		if err == nil {
			// since Kepler collects metrics at intervals of SamplePeriodSec, which is greater than 1 second, it is
			// necessary to calculate the energy consumption for the entire waiting period
			power[sensorIDPrefix+strconv.Itoa(int(i))] = float64(currPower / 1000 * config.SamplePeriodSec) /*miliJoules*/
		} else {
			return power, err
		}
	}

	return power, nil
}
