//go:build linux
// +build linux

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
	"strconv"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

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
