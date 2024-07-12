package device

import (
	"sync"
	"time"

	"golang.org/x/exp/maps"
	"k8s.io/klog/v2"
)

const (
	DUMMY DeviceType = iota
	HABANA
	DCGM
	NVML
)

var (
	globalRegistry *Registry
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
	return [...]string{"DUMMY", "HABANA", "DCGM", "NVML"}[d]
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
		globalRegistry = &Registry{
			Registry: map[string]map[DeviceType]deviceStartupFunc{},
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

func (r *Registry) MustRegister(a string, d DeviceType, deviceStartup deviceStartupFunc) {
	_, ok := r.Registry[a][d]
	if ok {
		klog.V(5).Infof("Device with type %s already exists", d)
		return
	}

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

// AddDeviceInterface adds a supported device interface, prints a fatal error in case of double registration.
func AddDeviceInterface(dtype DeviceType, accType string, deviceStartup deviceStartupFunc) {
	switch accType {
	case "GPU":
	case "DUMMY":
		// Handle GPU|Dummy device startup function registration
		if existingDevice := GetRegistry().Registry[accType][dtype]; existingDevice != nil {
			klog.Errorf("Multiple Devices attempting to register with name %q", dtype.String())
			return
		}

		if dtype == DCGM {
			// Remove "nvml" if "dcgm" is being registered
			GetRegistry().Unregister(NVML)
		} else if dtype == NVML {
			// Do not register "nvml" if "dcgm" is already registered
			if _, ok := GetRegistry().Registry["GPU"][DCGM]; ok {
				return
			}
		}
		GetRegistry().MustRegister(accType, dtype, deviceStartup)
	default:
		klog.Fatalf("Unsupported device type %q", dtype)
	}

	klog.V(5).Infof("Registered %s", dtype)
}

// GetAllDeviceTypes returns a slice with all the registered devices.
func GetAllDeviceTypes() []string {
	devices := append([]string{}, maps.Keys(GetRegistry().Registry)...)
	return devices
}

// Startup initializes and returns a new Device according to the given DeviceType [NVML|DCGM|DUMMY|HABANA].
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
