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
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/utils"
	"k8s.io/klog/v2"
)

const (
	freqPathDir         = "/sys/devices/system/cpu/cpufreq/"
	freqPath            = "/sys/devices/system/cpu/cpufreq/policy%d/scaling_cur_freq"
	hwmonRoot           = "/sys/class/hwmon"
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
	IsInitialized bool
	powerPath     string
}

func NewACPIPowerMeter() *ACPI {

	path, err := detecthwmonACPIPath()
	if err != nil {
		klog.V(0).ErrorS(err, "initialization of ACPI power meter failed.")
		return nil
	}
	klog.V(0).Infof("acpi power source initialized with path: %q", path)

	return &ACPI{powerPath: path, IsInitialized: true}
}

/*
detecthwmonACPIPath looks for an entry in /sys/class/hwmon which has an attribute
"name" set to "power_meter" and a subsystem named "acpi"
*/
func detecthwmonACPIPath() (string, error) {
	d, err := os.ReadDir(hwmonRoot)
	if err != nil {
		return "", fmt.Errorf("could not read %s", hwmonRoot)
	}
	for _, ent := range d {
		var name []byte
		devicePath, err := utils.Realpath(filepath.Join(hwmonRoot, ent.Name(), "device"))
		if err != nil {
			return "", fmt.Errorf("error occurred in reading hwmon device %w", err)
		}
		name, err = os.ReadFile(filepath.Join(hwmonRoot, ent.Name(), "name"))
		if err != nil {
			name, err = os.ReadFile(filepath.Join(devicePath, "name"))
			if err != nil {
				return "", fmt.Errorf("error occurred in reading file %w", err)
			}
		}
		strname := strings.Trim(string(name), "\n ")
		ssname, err := utils.Realpath(filepath.Join(devicePath, "subsystem"))
		if err != nil {
			return "", fmt.Errorf("error occurred in reading hwmon device %w", err)
		}
		ssname = filepath.Base(ssname)
		if strname == "power_meter" && ssname == "acpi" {
			return devicePath, nil
		}
	}
	return "", fmt.Errorf("could not find acpi power meter in hwmon")
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
	return a.IsInitialized
}

// GetEnergyFromHost returns the accumulated energy consumption
func (a *ACPI) GetAbsEnergyFromPlatform() (map[string]float64, error) {
	power := map[string]float64{}

	// TODO: the files in acpi power meter device does not depend on number of CPUs. The below loop will run only once
	for i := int32(1); i <= numCPUS; i++ {
		path := a.powerPath + "/" + acpiPowerFilePrefix + strconv.Itoa(int(i)) + acpiPowerFileSuffix
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
