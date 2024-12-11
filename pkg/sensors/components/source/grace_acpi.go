/*
Copyright 2024.

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
	"strconv"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

// Per https://docs.nvidia.com/grace-perf-tuning-guide/index.html#power-and-thermal-management
const (
	// Grace ACPI power paths and identifiers
	graceHwmonPathTemplate = "/sys/class/hwmon/hwmon*"
	graceDevicePath        = "device/"
	gracePowerPrefix       = "power1"
	graceOemInfoFile       = "_oem_info"
	graceAverageFile       = "_average"

	// Conversion factors
	microWattToMilliJoule = 1000 // Convert microwatts to mJ assuming 1 second sampling
)

type socketPowerPaths struct {
	totalPowerPath string // Grace Power Socket path
	cpuPowerPath   string // CPU Power Socket path
}

type GraceACPI struct {
	sockets  map[int]*socketPowerPaths // Power paths per socket
	currTime time.Time
}

func (GraceACPI) GetName() string {
	return "grace-acpi"
}

// findPowerPathsByLabel searches through hwmon directories to find power measurement files
// and matches them with their corresponding OEM info labels
func (g *GraceACPI) findPowerPathsByLabel() error {
	g.sockets = make(map[int]*socketPowerPaths)

	hwmonDirs, err := filepath.Glob(graceHwmonPathTemplate)
	if err != nil {
		return fmt.Errorf("failed to find hwmon directories: %v", err)
	}

	for _, hwmonDir := range hwmonDirs {
		deviceDir := hwmonDir + "/" + graceDevicePath

		// Check for power OEM info file
		oemFile := deviceDir + gracePowerPrefix + graceOemInfoFile
		data, err := os.ReadFile(oemFile)
		if err != nil {
			continue
		}
		label := strings.TrimSpace(string(data))

		// Extract socket number and power type
		// Per docs, Grace has 2 sockets, Grace Hopper has 1 CPU and 1 GPU socket
		var socketNum int
		if strings.HasSuffix(label, "Socket 0") {
			socketNum = 0
		} else if strings.HasSuffix(label, "Socket 1") {
			socketNum = 1
		} else {
			continue
		}

		// Initialize socket power paths if not exists
		if g.sockets[socketNum] == nil {
			g.sockets[socketNum] = &socketPowerPaths{}
		}

		// Store the power measurement path based on label type
		avgFile := deviceDir + gracePowerPrefix + graceAverageFile
		if strings.HasPrefix(label, "Grace Power") {
			g.sockets[socketNum].totalPowerPath = avgFile
		} else if strings.HasPrefix(label, "CPU Power") {
			g.sockets[socketNum].cpuPowerPath = avgFile
		}
	}

	if len(g.sockets) == 0 {
		return fmt.Errorf("no Grace power measurement files found")
	}

	klog.V(4).Infof("Detected Grace system with %d sockets", len(g.sockets))
	return nil
}

// readPowerFile reads the power value from a given file path
func (g *GraceACPI) readPowerFile(path string) (uint64, error) {
	if path == "" {
		return 0, fmt.Errorf("power path not initialized")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("failed to read power file %s: %v", path, err)
	}

	// Power values are in microWatts
	power, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse power value: %v", err)
	}

	now := time.Now()
	if g.currTime.IsZero() {
		g.currTime = now
		return 0, nil
	}

	// Calculate energy consumption over the time period
	diff := now.Sub(g.currTime)
	seconds := diff.Seconds()
	g.currTime = now

	// Convert power to energy
	energy := uint64(float64(power) * seconds / microWattToMilliJoule)
	return energy, nil
}

func (g *GraceACPI) Init() error {
	return g.findPowerPathsByLabel()
}

func (g *GraceACPI) IsSystemCollectionSupported() bool {
	if err := g.Init(); err != nil {
		klog.V(3).Infof("Grace ACPI power collection not supported: %v", err)
		return false
	}
	return true
}

// GetAbsEnergyFromCore returns the sum of CPU rail power across all sockets
func (g *GraceACPI) GetAbsEnergyFromCore() (uint64, error) {
	var totalEnergy uint64
	for socketNum, paths := range g.sockets {
		energy, err := g.readPowerFile(paths.cpuPowerPath)
		if err != nil {
			klog.V(3).Infof("Failed to read CPU power for socket %d: %v", socketNum, err)
			continue
		}
		totalEnergy += energy
	}
	return totalEnergy, nil
}

func (g *GraceACPI) GetAbsEnergyFromDram() (uint64, error) {
	// DRAM power is included in total socket power but not separately measured
	return 0, nil
}

func (g *GraceACPI) GetAbsEnergyFromUncore() (uint64, error) {
	return 0, nil
}

func (g *GraceACPI) GetAbsEnergyFromPackage() (uint64, error) {
	var totalEnergy uint64
	for socketNum, paths := range g.sockets {
		energy, err := g.readPowerFile(paths.totalPowerPath)
		if err != nil {
			klog.V(3).Infof("Failed to read total power for socket %d: %v", socketNum, err)
			continue
		}
		totalEnergy += energy
	}
	return totalEnergy, nil
}

func (g *GraceACPI) GetAbsEnergyFromNodeComponents() map[int]NodeComponentsEnergy {
	componentsEnergies := make(map[int]NodeComponentsEnergy)

	for socketNum, paths := range g.sockets {
		pkgEnergy, _ := g.readPowerFile(paths.totalPowerPath)
		coreEnergy, _ := g.readPowerFile(paths.cpuPowerPath)

		componentsEnergies[socketNum] = NodeComponentsEnergy{
			Core: coreEnergy,
			Pkg:  pkgEnergy,
			// DRAM is included in package power
		}
	}
	return componentsEnergies
}

func (g *GraceACPI) StopPower() {
	g.currTime = time.Time{}
}
