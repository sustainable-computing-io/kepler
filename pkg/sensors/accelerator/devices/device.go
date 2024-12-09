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
	"errors"
	"sync"
	"time"

	"github.com/sustainable-computing-io/kepler/pkg/config"
	"golang.org/x/exp/maps"
	"k8s.io/klog/v2"
)

const (
	MOCK DeviceType = iota
	HABANA
	DCGM
	NVML
	GRACE
)

var (
	deviceRegistry *Registry
	once           sync.Once
)

type (
	DeviceType        int
	deviceStartupFunc func() Device // Function prototype to startup a new device instance.
	Registry          struct {
		Registry map[string]map[DeviceType]deviceStartupFunc // Static map of supported Devices Startup functions
	}
)

func (d DeviceType) String() string {
	return [...]string{"MOCK", "HABANA", "DCGM", "NVML", "GRACE HOPPER"}[d]
}

type Device interface {
	// Name returns the name of the device
	Name() string
	// DevType returns the type of the device (nvml, dcgm, habana ...)
	DevType() DeviceType
	// GetHwType returns the type of hw the device is (gpu, processor)
	HwType() string
	// InitLib the external library loading, if any.
	InitLib() error
	// Init initizalizes and start the metric device
	Init() error
	// Shutdown stops the metric device
	Shutdown() bool
	// DevicesByID returns a map with devices identifying then by id
	DevicesByID() map[int]any
	// DevicesByName returns a map with devices identifying then by name
	DevicesByName() map[string]any
	// DeviceInstances returns a map with instances of each Device
	DeviceInstances() map[int]map[int]any
	// AbsEnergyFromDevice returns a map with mJ in each gpu device. Absolute energy is the sum of Idle + Dynamic energy.
	AbsEnergyFromDevice() []uint32
	// DeviceUtilizationStats returns a map with any additional device stats.
	DeviceUtilizationStats(dev any) (map[any]any, error)
	// ProcessResourceUtilizationPerDevice returns a map of UtilizationSample where the key is the process pid
	ProcessResourceUtilizationPerDevice(dev any, since time.Duration) (map[uint32]any, error)
	// IsDeviceCollectionSupported returns if it is possible to use this device
	IsDeviceCollectionSupported() bool
	// SetDeviceCollectionSupported manually set if it is possible to use this device. This is for testing purpose only.
	SetDeviceCollectionSupported(bool)
}

// Registry gets the default device Registry instance
func GetRegistry() *Registry {
	once.Do(func() {
		deviceRegistry = newRegistry()
		registerDevices(deviceRegistry)
	})
	return deviceRegistry
}

// NewRegistry creates a new instance of Registry without registering devices
func newRegistry() *Registry {
	return &Registry{
		Registry: map[string]map[DeviceType]deviceStartupFunc{},
	}
}

// SetRegistry replaces the global registry instance
// NOTE: All plugins will need to be manually registered
// after this function is called.
func SetRegistry(registry *Registry) {
	deviceRegistry = registry
	registerDevices(deviceRegistry)
}

// Register all available devices in the global registry
func registerDevices(r *Registry) {
	// Call individual device check functions
	dcgmCheck(r)
	habanaCheck(r)
	nvmlCheck(r)
	graceCheck(r)
}

func (r *Registry) MustRegister(a string, d DeviceType, deviceStartup deviceStartupFunc) {
	_, ok := r.Registry[a][d]
	if ok {
		klog.Infof("Device with type %s already exists", d)
		return
	}
	klog.V(5).Infof("Adding the device to the registry [%s][%s]", a, d.String())
	r.Registry[a] = map[DeviceType]deviceStartupFunc{
		d: deviceStartup,
	}
}

func (r *Registry) Unregister(d DeviceType) {
	for a := range r.Registry {
		_, exists := r.Registry[a][d]
		if exists {
			delete(r.Registry[a], d)
			return
		}
	}
	klog.Errorf("Device with type %s doesn't exist", d)
}

// GetAllDeviceTypes returns a slice with all the registered devices.
func (r *Registry) GetAllDeviceTypes() []string {
	devices := append([]string{}, maps.Keys(r.Registry)...)
	return devices
}

func addDeviceInterface(registry *Registry, dtype DeviceType, accType string, deviceStartup deviceStartupFunc) error {
	switch accType {
	case config.GPU:
		// Check if device is already registered
		if existingDevice := registry.Registry[accType][dtype]; existingDevice != nil {
			klog.Errorf("Multiple Devices attempting to register with name %q", dtype.String())
			return errors.New("multiple Devices attempting to register with name")
		}

		if dtype == DCGM {
			// Remove "nvml" if "dcgm" is being registered
			registry.Unregister(NVML)
		} else if dtype == NVML {
			// Do not register "nvml" if "dcgm" is already registered
			if _, ok := registry.Registry[config.GPU][DCGM]; ok {
				return errors.New("DCGM already registered. Skipping NVML")
			}
		}

		klog.V(5).Infof("Try to Register %s", dtype)
		registry.MustRegister(accType, dtype, deviceStartup)
	default:
		klog.V(5).Infof("Try to Register %s", dtype)
		registry.MustRegister(accType, dtype, deviceStartup)
	}

	klog.V(5).Infof("Registered %s", dtype)

	return nil
}

// Startup initializes and returns a new Device according to the given DeviceType [NVML|DCGM|HABANA].
func Startup(a string) Device {
	// Retrieve the global registry
	registry := GetRegistry()

	for d := range registry.Registry[a] {
		// Attempt to start the device from the registry
		if deviceStartup, ok := registry.Registry[a][d]; ok {
			klog.V(5).Infof("Starting up %s", d.String())
			return deviceStartup()
		}
	}

	// The device type is unsupported
	klog.Errorf("unsupported Device")
	return nil
}
