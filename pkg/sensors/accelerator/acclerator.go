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
	"strings"
	"sync"
	"time"

	"golang.org/x/exp/slices"

	"github.com/pkg/errors"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator/device"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	// Add supported devices.
	_ "github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator/device/sources"
)

var (
	accelerators map[string]Accelerator
)

// Accelerator represents an implementation of... equivalent Accelerator device.
type Accelerator interface {
	// StartupAccelerator ...
	StartupAccelerator() error
	// GetAccelerator ...
	GetAccelerator() device.AcceleratorInterface
	// GetAcceleratorType ...
	GetAcceleratorType() string
	// IsRunning ...
	IsRunning() bool
	// StopAccelerator ...
	StopAccelerator()
}

type accelerator struct {
	sync.Mutex
	acc           device.AcceleratorInterface // Device Accelerator Interface
	accType       string                      // NVML|DCGM|Habana|Dummy|QAT
	running       bool
	installedtime metav1.Time
}

func InitAcc(atype string, sleep bool) error {
	var numDevs int
	var getDevices func() []string
	maxDeviceInitRetry := 10

	switch atype {
	case "gpu":
		numDevs = len(device.GetGpuDevices())
		getDevices = device.GetGpuDevices
	case "qat":
		numDevs = len(device.GetQATDevices())
		getDevices = device.GetQATDevices
	default:
		return errors.New("unsupported accelerator")
	}

	if numDevs != 0 {
		var err error
		var a Accelerator
		devices := getDevices()

		klog.Infof("Initializing the %s Accelerator collectors in %v", strings.ToUpper(atype), devices)

		for _, accName := range devices {
			for i := 0; i <= maxDeviceInitRetry; i++ {
				if a = NewAccelerator(accName); a == nil {
					klog.Errorf("Could not init the %s Accelerator collector going to try again", strings.ToUpper(atype))
					if sleep {
						// The GPU operators typically takes longer time to initialize than kepler resulting in error to start the gpu driver
						// therefore, we wait up to 1 min to allow the gpu operator initialize
						time.Sleep(6 * time.Second)
					}
					continue
				}
				if err = a.StartupAccelerator(); err != nil {
					klog.Errorf("Could not Startup the %s Accelerator collector", strings.ToUpper(atype))
					break
				}
				klog.Infof("Startup %s Accelerator collector successful", strings.ToUpper(atype))
				break
			}
		}
	} else {
		klog.Errorf("No %s Accelerator collectors found", strings.ToUpper(atype))
		return errors.New("unsupported accelerator")
	}
	return nil
}

// GetAccelerators returns a map of supported accelerators.
func GetAccelerators() (map[string]Accelerator, error) {
	if len(accelerators) == 0 {
		return nil, errors.New("no accelerators found")
	}
	return accelerators, nil
}

// GetActiveAcceleratorsByType returns a map of supported accelerators based on the specified type gpu|qat|...
func GetActiveAcceleratorsByType(t string) (map[string]Accelerator, error) {
	accs := map[string]Accelerator{}
	for _, a := range accelerators {
		d := a.GetAccelerator()
		if d.GetHwType() == t && d.IsDeviceCollectionSupported() {
			accs[a.GetAcceleratorType()] = a
		}
	}
	if len(accs) == 0 {
		return nil, errors.New("accelerators not found")
	}
	return accs, nil
}

// NewAccelerator creates a new Accelerator instance [NVML|DCGM|DUMMY|HABANA|QAT] for the local node.
func NewAccelerator(accType string) Accelerator {

	containsType := slices.Contains(device.GetAllDevices(), accType)
	if !containsType {
		klog.Error("Invalid Device Type")
		return nil
	}

	_, ok := accelerators[accType] // e.g. accelerators[nvml|dcgm|habana|dummy|qat]
	if ok {
		klog.Infof("Accelerator with type %s already exists", accType)
		return accelerators[accType]
	}

	accelerators = map[string]Accelerator{
		accType: &accelerator{
			acc:           nil,
			running:       false,
			accType:       accType,
			installedtime: metav1.Time{},
		},
	}

	return accelerators[accType]
}

// StartupAccelerator of a particular type
func (a *accelerator) StartupAccelerator() error {
	var err error

	a.Lock()
	defer a.Unlock()

	if a.acc, err = device.StartupDevice(a.accType); err != nil {
		return errors.Wrap(err, "error creating the acc")
	}

	a.running = true
	a.installedtime = metav1.Now()

	klog.Infof("Accelerator started with acc type %s", a.accType)

	return nil
}

// StopAccelerator shutsdown an accelerator
func (a *accelerator) StopAccelerator() {
	a.Lock()
	defer a.Unlock()

	if !a.acc.Shutdown() {
		klog.Error("error shutting down the accelerator acc")
		return
	}

	delete(accelerators, a.accType)

	a.running = false

	klog.Info("Accelerator acc stopped")
}

// GetAcceleratorType returns the accelerator type
func (a *accelerator) GetAcceleratorType() string {
	a.Lock()
	defer a.Unlock()

	return a.accType
}

// IsRunning returns the running status of an accelerator
func (a *accelerator) IsRunning() bool {
	a.Lock()
	defer a.Unlock()

	return a.running
}

// GetAccelerator returns an accelerator interface
func (a *accelerator) GetAccelerator() device.AcceleratorInterface {
	a.Lock()
	defer a.Unlock()

	return a.acc
}
