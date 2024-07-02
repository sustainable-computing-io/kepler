package device

import (
	"sync"
	"time"

	"golang.org/x/exp/maps"
	"k8s.io/klog/v2"
)

const (
	DUMMY = iota
	HABANA
	DCGM
	NVML
)

var (
	globalRegistry *DeviceRegistry
	once           sync.Once
)

type DeviceType int

// Function prototype to create a new deviceCollector.
type deviceStartupFunc func() DeviceInterface
type DeviceRegistry struct {
	gpuDevices   map[DeviceType]deviceStartupFunc // Static map of supported gpuDevices.
	dummyDevices map[DeviceType]deviceStartupFunc // Static map of supported dummyDevices.
}
type DeviceInterface interface {
	// Name returns the name of the device
	Name() string
	// DevType returns the type of the device (nvml, dcgm, habana ...)
	DevType() DeviceType
	// DevTypeName returns the type of the device (nvml, dcgm, habana ...) as a string
	DevTypeName() string
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

func (d DeviceType) String() string {
	return [...]string{"DUMMY", "HABANA", "DCGM", "NVML"}[d]
}

// Registry gets the default device DeviceRegistry instance
func Registry() *DeviceRegistry {
	once.Do(func() {
		globalRegistry = &DeviceRegistry{
			gpuDevices:   map[DeviceType]deviceStartupFunc{},
			dummyDevices: map[DeviceType]deviceStartupFunc{},
		}
	})
	return globalRegistry
}

// SetRegistry replaces the global registry instance
// NOTE: All plugins will need to be manually registered
// after this function is called.
func SetRegistry(registry *DeviceRegistry) {
	globalRegistry = registry
}

// AddDeviceInterface adds a supported device interface, prints a fatal error in case of double registration.
func AddDeviceInterface(dtype DeviceType, accType string, deviceStartup deviceStartupFunc) {
	switch accType {
	case "GPU":
		// Handle GPU devices registration
		if existingDevice := Registry().gpuDevices[dtype]; existingDevice != nil {
			klog.Fatalf("Multiple gpuDevices attempting to register with name %q", dtype.String())
		}

		if dtype == DCGM {
			// Remove "nvml" if "dcgm" is being registered
			delete(Registry().gpuDevices, NVML)
		} else if dtype == NVML {
			// Do not register "nvml" if "dcgm" is already registered
			if _, ok := Registry().gpuDevices[DCGM]; ok {
				return
			}
		}
		Registry().gpuDevices[dtype] = deviceStartup

	case "DUMMY":
		// Handle dummy devices registration
		if existingDevice := Registry().dummyDevices[dtype]; existingDevice != nil {
			klog.Fatalf("Multiple dummyDevices attempting to register with name %q", dtype)
		}
		Registry().dummyDevices[dtype] = deviceStartup

	default:
		klog.Fatalf("Unsupported device type %q", dtype)
	}

	klog.V(5).Infof("Registered %s", dtype)
}

// GetAllDevices returns a slice with all the registered devices.
func GetAllDevices() []DeviceType {
	devices := append(append([]DeviceType{}, maps.Keys(Registry().gpuDevices)...), maps.Keys(Registry().dummyDevices)...)
	return devices
}

// GetGpuDevices returns a slice of the registered gpus.
func GetGpuDevices() []DeviceType {
	return maps.Keys(Registry().gpuDevices)
}

// GetDummyDevices returns only the dummy devices.
func GetDummyDevices() []DeviceType {
	return maps.Keys(Registry().dummyDevices)
}

// Startup initializes and returns a new DeviceInterface according to the given DeviceType [NVML|DCGM|DUMMY|HABANA].
func Startup(d DeviceType) DeviceInterface {
	// Retrieve the global registry
	registry := Registry()

	// Attempt to start the device from the gpuDevices map
	if deviceStartup, ok := registry.gpuDevices[d]; ok {
		klog.V(5).Infof("Starting up %s", d.String())
		return deviceStartup()
	}

	// Attempt to start the device from the dummyDevices map
	if deviceStartup, ok := registry.dummyDevices[d]; ok {
		klog.V(5).Infof("Starting up %s", d.String())
		return deviceStartup()
	}

	// The device type is unsupported
	klog.Errorf("unsupported Device")
	return nil
}
