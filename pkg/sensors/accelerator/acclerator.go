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
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator/device"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	// Add supported devices.
	_ "github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator/device/sources"
)

const (
	GPU = iota
	// Add other accelerator types here
)

var (
	globalRegistry *AcceleratorRegistry
	once           sync.Once
)

// Accelerator represents an implementation of... equivalent Accelerator device.
type Accelerator interface {
	// Device returns an underlying accelerator device implementation...
	Device() device.DeviceInterface
	// DeviceType ...
	DeviceType() device.DeviceType
	// IsRunning ...
	IsRunning() bool
	// StopAccelerator ...
	Stop()
	AccType() AcceleratorType
}

type AcceleratorType int

type AcceleratorRegistry struct {
	m        sync.Mutex
	Registry map[device.DeviceType]Accelerator
}
type accelerator struct {
	dev           device.DeviceInterface // Device Accelerator Interface
	accType       AcceleratorType
	running       bool
	installedTime metav1.Time
}

func (a AcceleratorType) String() string {
	return [...]string{"GPU"}[a]
}

// Registry gets the default device AcceleratorRegistry instance
func Registry() *AcceleratorRegistry {
	once.Do(func() {
		globalRegistry = &AcceleratorRegistry{
			m:        sync.Mutex{},
			Registry: map[device.DeviceType]Accelerator{},
		}
	})
	return globalRegistry
}

// SetRegistry replaces the global registry instance
// NOTE: All plugins will need to be manually registered
// after this function is called.
func SetRegistry(registry *AcceleratorRegistry) {
	globalRegistry = registry
}

// GetAccelerators returns a map of supported accelerators.
func (r *AcceleratorRegistry) Accelerators() (map[device.DeviceType]Accelerator, error) {
	if len(r.Registry) == 0 {
		return nil, errors.New("no accelerators found")
	}
	return r.Registry, nil
}

// ActiveAcceleratorsByType returns a map of supported accelerators based on the specified type...
func (r *AcceleratorRegistry) ActiveAcceleratorsByType(t AcceleratorType) (map[device.DeviceType]Accelerator, error) {
	acc := map[device.DeviceType]Accelerator{}
	for _, a := range r.Registry {
		if a.AccType() == t && a.IsRunning() {
			d := a.Device()
			if d.IsDeviceCollectionSupported() {
				acc[d.DevType()] = a
			}
		}
	}
	if len(acc) == 0 {
		return nil, errors.New("accelerators not found")
	}
	return acc, nil
}

func CreateAndRegister(atype AcceleratorType, r *AcceleratorRegistry, sleep bool) error {
	var numDevs int
	var getDevices func() []device.DeviceType
	maxDeviceInitRetry := 10

	switch atype {
	case GPU:
		numDevs = len(device.GetGpuDevices())
		getDevices = device.GetGpuDevices // returns a slice of registered GPU devices[NVML|DCGM|HABANA]. TODO CHECK IF WE NEED TO MAKE THIS SINGULAR
	default:
		return errors.New("unsupported accelerator")
	}

	if numDevs != 0 {
		devices := getDevices()

		klog.Infof("Initializing the %s Accelerator collectors in type %v", atype.String(), devices)

		for _, devType := range devices {
			for i := 0; i <= maxDeviceInitRetry; i++ {
				// Create a new accelerator and add it to the registry
				if err := newAccelerator(devType, atype, r); err != nil {
					klog.Errorf("Could not init the %s Accelerator collector going to try again", devType.String())
					if sleep {
						// The GPU operators typically takes longer time to initialize than kepler resulting in error to start the gpu driver
						// therefore, we wait up to 1 min to allow the gpu operator initialize
						time.Sleep(6 * time.Second)
					}
					continue
				}
				klog.Infof("Startup %s Accelerator collector successful", devType.String())
				break
			}
		}
	} else {
		klog.Errorf("No %s Accelerator collectors found", atype.String())
		return errors.New("unsupported accelerator")
	}
	return nil
}

// newAccelerator creates a new Accelerator instance with a specific device [NVML|DCGM|DUMMY|HABANA] for the local node.
func newAccelerator(devType device.DeviceType, atype AcceleratorType, r *AcceleratorRegistry) error {
	var d device.DeviceInterface
	var err error

	_, ok := r.Registry[devType] // e.g. accelerators[nvml|dcgm|habana|dummy]
	if ok {
		klog.Infof("Accelerator with type %s already exists", devType)
		return nil
	}

	if d, err = device.Startup(devType); err != nil {
		return errors.Errorf("error starting up the device %v", err)
	}

	r.m.Lock()
	defer r.m.Unlock()

	r.Registry[devType] = &accelerator{
		dev:           d,
		running:       true,
		accType:       atype,
		installedTime: metav1.Now(),
	}

	klog.Infof("Accelerator registered and started with device type %s", devType)

	return nil
}

func Shutdown(atype AcceleratorType) {
	if accelerators, err := Registry().ActiveAcceleratorsByType(atype); err == nil {
		for _, a := range accelerators {
			a.Stop()
		}
	}
}

// Stop shutsdown an accelerator
func (a *accelerator) Stop() {
	devType := a.dev.DevType()
	if !a.dev.Shutdown() {
		klog.Error("error shutting down the accelerator acc")
		return
	}

	if a, err := Registry().Accelerators(); err == nil {
		delete(a, devType)
	}

	a.running = false

	klog.Info("Accelerator acc stopped")
}

// Device returns an accelerator interface
func (a *accelerator) Device() device.DeviceInterface {
	return a.dev
}

// DeviceType returns the accelerator's underlying device type
func (a *accelerator) DeviceType() device.DeviceType {
	return a.dev.DevType()
}

// DeviceType returns the accelerator's underlying device type
func (a *accelerator) AccType() AcceleratorType {
	return a.accType
}

// IsRunning returns the running status of an accelerator
func (a *accelerator) IsRunning() bool {
	return a.running
}
