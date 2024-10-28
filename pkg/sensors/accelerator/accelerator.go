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
	"github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator/devices"
	"k8s.io/klog/v2"
)

var (
	globalRegistry *Registry
	once           sync.Once
)

// Accelerator represents an implementation of... equivalent Accelerator device.
type Accelerator interface {
	// Device returns an underlying accelerator device implementation...
	Device() devices.Device
	// IsRunning returns whether or not that device is running
	IsRunning() bool
	// stop stops an accelerator and unregisters it
	stop()
}

type accelerator struct {
	dev     devices.Device // Device Accelerator Interface
	running bool
}

type Registry struct {
	Registry map[string]Accelerator
}

// Registry gets the default device Registry instance
func GetRegistry() *Registry {
	once.Do(func() {
		globalRegistry = &Registry{
			Registry: map[string]Accelerator{},
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
	_, ok := r.Registry[a.Device().HwType()]
	if ok {
		klog.V(5).Infof("Accelerator with type %s already exists", a.Device().HwType())
		return
	}
	r.Registry[a.Device().HwType()] = a
}

func (r *Registry) Unregister(a Accelerator) bool {
	_, exists := r.Registry[a.Device().HwType()]
	if exists {
		delete(r.Registry, a.Device().HwType())
		return true
	}
	klog.Errorf("Accelerator with type %s doesn't exist", a.Device().HwType())
	return false
}

// Devices returns a map of supported accelerators.
func (r *Registry) accelerators() map[string]Accelerator {
	acc := map[string]Accelerator{}

	if len(r.Registry) == 0 {
		// No accelerators found
		return nil
	}

	for _, a := range r.Registry {
		if a.IsRunning() {
			d := a.Device()
			if d.IsDeviceCollectionSupported() {
				acc[d.HwType()] = a
			}
		}
	}

	return acc
}

// ActiveAcceleratorByType returns a map of supported accelerators based on the specified type...
func (r *Registry) activeAcceleratorByType(t string) Accelerator {
	if len(r.Registry) == 0 {
		// No accelerators found
		klog.V(5).Infof("No accelerators found")
		return nil
	}

	for _, a := range r.Registry {
		if a.Device().HwType() == t && a.IsRunning() {
			d := a.Device()
			if d.IsDeviceCollectionSupported() {
				return a
			}
		}
	}
	return nil
}

func New(atype string, sleep bool) (Accelerator, error) {
	var d devices.Device
	maxDeviceInitRetry := 10

	// Init the available devices.

	devs := devices.GetRegistry().GetAllDeviceTypes()
	numDevs := len(devs)
	if numDevs == 0 || !slices.Contains(devs, atype) {
		return nil, errors.New("no devices found")
	}

	klog.V(5).Infof("Initializing the Accelerator of type %v", atype)

	for i := 0; i < maxDeviceInitRetry; i++ {
		if d = devices.Startup(atype); d == nil {
			klog.Errorf("Could not init the %s device going to try again", atype)
			if sleep {
				// The GPU operators typically takes longer time to initialize than kepler resulting in error to start the gpu driver
				// therefore, we wait up to 1 min to allow the gpu operator initialize
				time.Sleep(6 * time.Second)
			}
			continue
		}
		klog.V(5).Infof("Startup %s Accelerator successful", atype)
		break
	}

	return &accelerator{
		dev:     d,
		running: true,
	}, nil
}

func Shutdown() {
	if accelerators := GetRegistry().accelerators(); accelerators != nil {
		for _, a := range accelerators {
			klog.V(5).Infof("Shutting down %s", a.Device().DevType())
			a.stop()
		}
	} else {
		klog.V(5).Info("No devices to shutdown")
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
func (a *accelerator) Device() devices.Device {
	return a.dev
}

// DeviceType returns the accelerator's underlying device type
func (a *accelerator) DevType() string {
	return a.dev.DevType().String()
}

// IsRunning returns the running status of an accelerator
func (a *accelerator) IsRunning() bool {
	return a.running
}

func GetActiveAcceleratorByType(t string) Accelerator {
	return GetRegistry().activeAcceleratorByType(t)
}

func GetAccelerators() map[string]Accelerator {
	return GetRegistry().accelerators()
}
