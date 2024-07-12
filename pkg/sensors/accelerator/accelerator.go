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

package accelerator

//nolint:gci // The supported device imports are kept separate.
import (
	"slices"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator/device"
	"k8s.io/klog/v2"

	// Add supported devices.
	_ "github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator/device/sources"
)

var (
	globalRegistry *Registry
	once           sync.Once
)

const (
	DUMMY AcceleratorType = iota
	GPU
	// Add other accelerator types here [IPU|DPU|...]
)

type AcceleratorType int

func (a AcceleratorType) String() string {
	return [...]string{"DUMMY", "GPU"}[a]
}

// Accelerator represents an implementation of... equivalent Accelerator device.
type Accelerator interface {
	// Device returns an underlying accelerator device implementation...
	Device() device.Device
	// IsRunning returns whether or not that device is running
	IsRunning() bool
	// AccType returns the acclerator type.
	AccType() AcceleratorType
	// stop stops an accelerator and unregisters it
	stop()
}

type accelerator struct {
	dev     device.Device // Device Accelerator Interface
	accType AcceleratorType
	running bool
}

type Registry struct {
	Registry map[AcceleratorType]Accelerator
}

// Registry gets the default device Registry instance
func GetRegistry() *Registry {
	once.Do(func() {
		globalRegistry = &Registry{
			Registry: map[AcceleratorType]Accelerator{},
		}
	})
	return globalRegistry
}

// SetRegistry replaces the global registry instance
// NOTE: All plugins will need to be manually registered
// after this function is called.
func SetRegistry(registry *Registry) {
	globalRegistry = registry
}

func (r *Registry) MustRegister(a Accelerator) {
	_, ok := r.Registry[a.AccType()]
	if ok {
		klog.V(5).Infof("Accelerator with type %s already exists", a.AccType())
		return
	}
	r.Registry[a.AccType()] = a
}

func (r *Registry) Unregister(a Accelerator) bool {
	_, exists := r.Registry[a.AccType()]
	if exists {
		delete(r.Registry, a.AccType())
		return true
	}
	klog.Errorf("Accelerator with type %s doesn't exist", a.AccType())
	return false
}

// Devices returns a map of supported accelerators.
func (r *Registry) Accelerators() map[device.DeviceType]Accelerator {
	acc := map[device.DeviceType]Accelerator{}

	if len(r.Registry) == 0 {
		// No accelerators found
		return nil
	}

	for _, a := range r.Registry {
		if a.IsRunning() {
			d := a.Device()
			if d.IsDeviceCollectionSupported() {
				acc[d.DevType()] = a
			}
		}
	}

	return acc
}

// ActiveAcceleratorsByType returns a map of supported accelerators based on the specified type...
func (r *Registry) ActiveAcceleratorsByType(t AcceleratorType) (map[AcceleratorType]Accelerator, error) {
	acc := map[AcceleratorType]Accelerator{}
	for _, a := range r.Registry {
		if a.AccType() == t && a.IsRunning() {
			d := a.Device()
			if d.IsDeviceCollectionSupported() {
				acc[t] = a
			}
		}
	}
	if len(acc) == 0 {
		return nil, errors.New("accelerators not found")
	}
	return acc, nil
}

func New(atype AcceleratorType, sleep bool) (Accelerator, error) {
	var numDevs int
	maxDeviceInitRetry := 10
	var d device.Device

	switch atype {
	case GPU:
		numDevs = len(device.GetAllDeviceTypes())
	default:
		return nil, errors.New("unsupported accelerator")
	}

	if numDevs != 0 {
		devices := device.GetAllDeviceTypes()
		if !slices.Contains(devices, atype.String()) {
			return nil, errors.New("no devices found")
		}
		klog.V(5).Infof("Initializing the %s Accelerator collectors in type %v", atype.String(), devices)

		for i := 0; i < maxDeviceInitRetry; i++ {
			if d = device.Startup(atype.String()); d == nil {
				klog.Errorf("Could not init the %s device going to try again", atype.String())
				if sleep {
					// The GPU operators typically takes longer time to initialize than kepler resulting in error to start the gpu driver
					// therefore, we wait up to 1 min to allow the gpu operator initialize
					time.Sleep(6 * time.Second)
				}
				continue
			}
			if d == nil {
				return nil, errors.New("could not startup the device")
			}
			klog.V(5).Infof("Startup %s Accelerator collector successful", atype.String())
			break

		}
	} else {
		return nil, errors.New("No Accelerator devices found")
	}

	return &accelerator{
		dev:     d,
		running: true,
		accType: atype,
	}, nil
}

func Shutdown() {
	if accelerators := GetRegistry().Accelerators(); accelerators != nil {
		for _, a := range accelerators {
			klog.V(5).Infof("Shutting down %s", a.AccType().String())
			a.stop()
		}
	}
}

// Stop shutsdown an accelerator
func (a *accelerator) stop() {
	if !a.dev.Shutdown() {
		klog.Error("error shutting down the device")
		return
	}

	if shutdown := GetRegistry().Unregister(a); !shutdown {
		klog.Error("error shutting down the accelerator")
		return
	}

	klog.V(5).Info("Accelerator stopped")
}

// Device returns an accelerator interface
func (a *accelerator) Device() device.Device {
	return a.dev
}

// DeviceType returns the accelerator's underlying device type
func (a *accelerator) AccType() AcceleratorType {
	return a.accType
}

// IsRunning returns the running status of an accelerator
func (a *accelerator) IsRunning() bool {
	return a.running
}
