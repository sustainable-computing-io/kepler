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

package devices

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sustainable-computing-io/kepler/pkg/config"
	"k8s.io/klog/v2"
)

const (
	// Grace ACPI power paths and identifiers
	graceHwmonPathTemplate = "/sys/class/hwmon/hwmon*"
	graceDevicePath        = "device/"
	gracePowerPrefix       = "power1"
	graceOemInfoFile       = "_oem_info"
	graceAverageFile       = "_average"

	// Grace Hopper module power identifier
	graceModuleLabel = "Module Power Socket" // Total CG1 module power (GPU+HBM)

	// Constants
	microWattToMilliJoule = 1000 // Convert microwatts to mJ assuming 1 second sampling
	graceHwType           = config.GPU
)

var (
	graceAccImpl = gpuGraceACPI{}
	graceType    DeviceType
)

type gpuGraceACPI struct {
	collectionSupported bool
	modulePowerPaths    map[int]string // Module power paths indexed by socket
	currTime            time.Time
}

func graceCheck(r *Registry) {
	if err := graceAccImpl.InitLib(); err != nil {
		klog.V(5).Infof("Error initializing Grace GPU: %v", err)
		return
	}
	graceType = GRACE
	if err := addDeviceInterface(r, graceType, graceHwType, graceDeviceStartup); err == nil {
		klog.Infof("Using %s to obtain Grace GPU power", graceAccImpl.Name())
	} else {
		klog.V(5).Infof("Error registering Grace GPU: %v", err)
	}
}

func graceDeviceStartup() Device {
	if err := graceAccImpl.Init(); err != nil {
		klog.Errorf("failed to init Grace GPU device: %v", err)
		return nil
	}
	return &graceAccImpl
}

func (g *gpuGraceACPI) findModulePowerPaths() error {
	g.modulePowerPaths = make(map[int]string)

	hwmonDirs, err := filepath.Glob(graceHwmonPathTemplate)
	if err != nil {
		return fmt.Errorf("failed to find hwmon directories: %v", err)
	}

	for _, hwmonDir := range hwmonDirs {
		deviceDir := hwmonDir + "/" + graceDevicePath
		oemFile := deviceDir + gracePowerPrefix + graceOemInfoFile
		data, err := os.ReadFile(oemFile)
		if err != nil {
			continue
		}
		label := strings.TrimSpace(string(data))

		if !strings.HasPrefix(label, graceModuleLabel) {
			continue
		}

		var socketNum int
		if strings.HasSuffix(label, "Socket 0") {
			socketNum = 0
		} else if strings.HasSuffix(label, "Socket 1") {
			socketNum = 1
		} else {
			continue
		}

		avgFile := deviceDir + gracePowerPrefix + graceAverageFile
		g.modulePowerPaths[socketNum] = avgFile
	}

	return nil
}

func (g *gpuGraceACPI) readPowerFile(path string) (uint64, error) {
	if path == "" {
		return 0, fmt.Errorf("power path not initialized")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("failed to read power file %s: %v", path, err)
	}

	power, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse power value: %v", err)
	}

	now := time.Now()
	if g.currTime.IsZero() {
		g.currTime = now
		return 0, nil
	}

	diff := now.Sub(g.currTime)
	seconds := diff.Seconds()
	g.currTime = now

	energy := uint64(float64(power) * seconds / microWattToMilliJoule)
	return energy, nil
}

func (g *gpuGraceACPI) Name() string {
	return graceType.String()
}

func (g *gpuGraceACPI) DevType() DeviceType {
	return graceType
}

func (g *gpuGraceACPI) HwType() string {
	return graceHwType
}

func (g *gpuGraceACPI) InitLib() error {
	return nil
}

func (g *gpuGraceACPI) Init() error {
	if err := g.findModulePowerPaths(); err != nil {
		return err
	}
	g.collectionSupported = len(g.modulePowerPaths) > 0
	if g.collectionSupported {
		klog.V(4).Infof("Detected Grace Hopper system with %d GPUs", len(g.modulePowerPaths))
	}
	return nil
}

func (g *gpuGraceACPI) IsDeviceCollectionSupported() bool {
	return g.collectionSupported
}

func (g *gpuGraceACPI) SetDeviceCollectionSupported(supported bool) {
	g.collectionSupported = supported
}

func (g *gpuGraceACPI) AbsEnergyFromDevice() []uint32 {
	var energies []uint32
	for socketNum := 0; socketNum < len(g.modulePowerPaths); socketNum++ {
		if path, ok := g.modulePowerPaths[socketNum]; ok {
			energy, err := g.readPowerFile(path)
			if err != nil {
				klog.V(3).Infof("Failed to read GPU power for socket %d: %v", socketNum, err)
				energies = append(energies, 0)
				continue
			}
			energies = append(energies, uint32(energy))
		}
	}
	return energies
}

func (g *gpuGraceACPI) DevicesByID() map[int]any {
	devs := make(map[int]any)
	for socketNum := range g.modulePowerPaths {
		devs[socketNum] = GPUDevice{
			ID:          socketNum,
			IsSubdevice: false,
		}
	}
	return devs
}

func (g *gpuGraceACPI) DevicesByName() map[string]any {
	return make(map[string]any)
}

func (g *gpuGraceACPI) DeviceInstances() map[int]map[int]any {
	return make(map[int]map[int]any)
}

func (g *gpuGraceACPI) DeviceUtilizationStats(dev any) (map[any]any, error) {
	return make(map[any]any), nil
}

func (g *gpuGraceACPI) ProcessResourceUtilizationPerDevice(dev any, since time.Duration) (map[uint32]any, error) {
	// Grace Hopper doesn't provide per-process GPU utilization through ACPI
	return make(map[uint32]any), nil
}

func (g *gpuGraceACPI) Shutdown() bool {
	g.currTime = time.Time{}
	return true
}
