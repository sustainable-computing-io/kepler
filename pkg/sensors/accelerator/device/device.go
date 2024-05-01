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

package device

import (
	"errors"
	"time"

	"golang.org/x/exp/maps"
	"k8s.io/klog/v2"
)

var (
	gpuDevices   = map[string]deviceStartupFunc{} // Static map of supported gpuDevices.
	dummyDevices = map[string]deviceStartupFunc{} // Static map of supported dummyDevices.
	qatDevices   = map[string]deviceStartupFunc{} // Static map of supported qatDevices.
)

type AcceleratorInterface interface {
	// GetName returns the name of the device
	GetName() string
	// GetType returns the type of the device (nvml, qat, dcgm ...)
	GetType() string
	// GetHwType returns the type of hw the device is (gpu, processor)
	GetHwType() string
	// Init the external library loading, if any.
	InitLib() error
	// Init initizalize and start the metric device
	Init() error
	// Shutdown stops the metric device
	Shutdown() bool
	// GetDevicesByID returns a map with devices identifying then by id
	GetDevicesByID() map[int]any
	// GetDevicesByName returns a map with devices identifying then by name
	GetDevicesByName() map[string]any
	// GetDeviceInstances returns a map with instances of each Device
	GetDeviceInstances() map[int]map[int]any
	// GetAbsEnergyFromDevice returns a map with mJ in each gpu device. Absolute energy is the sum of Idle + Dynamic energy.
	GetAbsEnergyFromDevice() []uint32
	// GetDeviceUtilizationStats returns a map with any additional device stats.
	GetDeviceUtilizationStats(device any) (map[any]interface{}, error)
	// GetProcessResourceUtilizationPerDevice returns a map of UtilizationSample where the key is the process pid
	GetProcessResourceUtilizationPerDevice(device any, since time.Duration) (map[uint32]any, error)
	// IsDeviceCollectionSupported returns if it is possible to use this device
	IsDeviceCollectionSupported() bool
	// SetDeviceCollectionSupported manually set if it is possible to use this device. This is for testing purpose only.
	SetDeviceCollectionSupported(bool)
}

// Function prototype to create a new deviceCollector.
type deviceStartupFunc func() (AcceleratorInterface, error)

// AddDeviceInterface adds a supported device interface, prints a fatal error in case of double registration.
func AddDeviceInterface(name, dtype string, deviceStartup deviceStartupFunc) {
	switch dtype {
	case "gpu":
		// Handle GPU devices registration
		if existingDevice := gpuDevices[name]; existingDevice != nil {
			klog.Fatalf("Multiple gpuDevices attempting to register with name %q", name)
		}

		if name == "dcgm" {
			// Remove "nvml" if "dcgm" is being registered
			delete(gpuDevices, "nvml")
		} else if name == "nvml" {
			// Do not register "nvml" if "dcgm" is already registered
			if _, ok := gpuDevices["dcgm"]; ok {
				return
			}
		}
		gpuDevices[name] = deviceStartup

	case "dummy":
		// Handle dummy devices registration
		if existingDevice := dummyDevices[name]; existingDevice != nil {
			klog.Fatalf("Multiple dummyDevices attempting to register with name %q", name)
		}
		dummyDevices[name] = deviceStartup

	case "qat":
		// Handle qat devices registration
		if existingDevice := qatDevices[name]; existingDevice != nil {
			klog.Fatalf("Multiple qatDevices attempting to register with name %q", name)
		}
		qatDevices[name] = deviceStartup

	default:
		klog.Fatalf("Unsupported device type %q", dtype)
	}

	klog.Infof("Registered %s", name)
}

// GetAllDevices returns a slice with all the registered devices.
func GetAllDevices() []string {
	devices := append(append(append([]string{}, maps.Keys(gpuDevices)...), maps.Keys(gpuDevices)...), maps.Keys(qatDevices)...)
	return devices
}

// GetGpuDevices returns a slice of the registered gpus.
func GetGpuDevices() []string {
	return maps.Keys(gpuDevices)
}

// GetDummyDevices returns only the dummy devices.
func GetDummyDevices() []string {
	return maps.Keys(dummyDevices)
}

// GetQATDevices returns a slice of the registered QAT devices.
func GetQATDevices() []string {
	return maps.Keys(qatDevices)
}

// StartupDevice Returns a new AcceleratorInterface according the required name[nvml|dcgm|dummy|habana|qat].
func StartupDevice(name string) (AcceleratorInterface, error) {

	if deviceStartup, ok := gpuDevices[name]; ok {
		klog.Infof("Starting up %s", name)
		return deviceStartup()
	} else if deviceStartup, ok := dummyDevices[name]; ok {
		klog.Infof("Starting up %s", name)
		return deviceStartup()
	} else if deviceStartup, ok := qatDevices[name]; ok {
		klog.Infof("Starting up %s", name)
		return deviceStartup()
	}

	return nil, errors.New("unsupported Device")
}
