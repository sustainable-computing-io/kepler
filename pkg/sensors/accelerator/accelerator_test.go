package accelerator

import (
	"testing"

	"github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator/device"

	// Add supported devices.

	_ "github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator/device/sources"
)

func newMockDevice() device.DeviceInterface {
	return device.Startup(device.DeviceType(0))
}

func cleanupMockDevice(atype AcceleratorType) {
	Shutdown(atype)
}

func TestRegistry(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() *AcceleratorRegistry
		expectedLen int
		expectError bool
		cleanup     func()
	}{
		{
			name: "Empty registry",
			setup: func() *AcceleratorRegistry {
				registry := AcceleratorRegistry{
					Registry: map[device.DeviceType]Accelerator{},
				}
				SetRegistry(&registry)

				return Registry()
			},
			expectedLen: 0,
			expectError: true,
			cleanup:     func() {},
		},
		{
			name: "Non-empty registry",
			setup: func() *AcceleratorRegistry {
				registry := &AcceleratorRegistry{
					Registry: map[device.DeviceType]Accelerator{},
				}
				registry.Registry[device.DeviceType(0)] = &accelerator{
					dev: newMockDevice(),
				}
				SetRegistry(registry)
				return Registry()
			},
			expectedLen: 1,
			expectError: false,
			cleanup:     func() { cleanupMockDevice(AcceleratorType(0)) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := tt.setup()
			accs, err := registry.Accelerators()
			if tt.expectError && err == nil {
				t.Errorf("expected an error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("did not expect an error but got %v", err)
			}
			if len(accs) != tt.expectedLen {
				t.Errorf("expected %d accelerators, but got %d", tt.expectedLen, len(accs))
			}
			tt.cleanup()
		})
	}
}

func TestActiveAcceleratorsByType(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() *AcceleratorRegistry
		accType     AcceleratorType
		expectedLen int
		expectError bool
		cleanup     func()
	}{
		{
			name: "No accelerators of given type",
			setup: func() *AcceleratorRegistry {
				return &AcceleratorRegistry{
					Registry: map[device.DeviceType]Accelerator{},
				}
			},
			accType:     AcceleratorType(0),
			expectedLen: 0,
			expectError: true,
			cleanup:     func() {},
		},
		{
			name: "One active accelerator of given type",
			setup: func() *AcceleratorRegistry {
				registry := &AcceleratorRegistry{
					Registry: map[device.DeviceType]Accelerator{},
				}
				registry.Registry[device.DeviceType(0)] = &accelerator{
					dev:     newMockDevice(),
					accType: AcceleratorType(0),
					running: true,
				}
				return registry
			},
			accType:     AcceleratorType(0),
			expectedLen: 1,
			expectError: false,
			cleanup:     func() { cleanupMockDevice(AcceleratorType(0)) },
		},
		{
			name: "One inactive accelerator of given type",
			setup: func() *AcceleratorRegistry {
				registry := &AcceleratorRegistry{
					Registry: map[device.DeviceType]Accelerator{},
				}
				registry.Registry[device.DeviceType(0)] = &accelerator{
					dev:     newMockDevice(),
					accType: AcceleratorType(0),
					running: false,
				}
				return registry
			},
			accType:     AcceleratorType(0),
			expectedLen: 0,
			expectError: true,
			cleanup:     func() {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := tt.setup()
			accs, err := registry.ActiveAcceleratorsByType(tt.accType)
			if tt.expectError && err == nil {
				t.Errorf("expected an error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("did not expect an error but got %v", err)
			}
			if len(accs) != tt.expectedLen {
				t.Errorf("expected %d active accelerators, but got %d", tt.expectedLen, len(accs))
			}
			tt.cleanup()
		})
	}
}

func TestCreateAndRegister(t *testing.T) {
	tests := []struct {
		name        string
		accType     AcceleratorType
		setup       func() *AcceleratorRegistry
		sleep       bool
		expectError bool
		cleanup     func()
	}{
		{
			name:    "Unsupported accelerator",
			accType: AcceleratorType(999), // invalid accelerator type
			setup: func() *AcceleratorRegistry {
				return &AcceleratorRegistry{
					Registry: map[device.DeviceType]Accelerator{},
				}
			},
			sleep:       false,
			expectError: true,
			cleanup:     func() {},
		},
		{
			name:    "Devices found and registered",
			accType: AcceleratorType(0),
			setup: func() *AcceleratorRegistry {
				return &AcceleratorRegistry{
					Registry: map[device.DeviceType]Accelerator{},
				}
			},
			sleep:       false,
			expectError: false,
			cleanup:     func() { cleanupMockDevice(AcceleratorType(0)) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := tt.setup()
			err := CreateAndRegister(tt.accType, registry, tt.sleep)
			if tt.expectError && err == nil {
				t.Errorf("expected an error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("did not expect an error but got %v", err)
			}
			tt.cleanup()
		})
	}
}

func TestShutdown(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *AcceleratorRegistry
		accType AcceleratorType
	}{
		{
			name: "Shutdown active accelerators",
			setup: func() *AcceleratorRegistry {
				registry := &AcceleratorRegistry{
					Registry: map[device.DeviceType]Accelerator{},
				}
				registry.Registry[device.DeviceType(0)] = &accelerator{
					dev:     newMockDevice(),
					accType: AcceleratorType(0),
					running: true,
				}
				return registry
			},
			accType: AcceleratorType(0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := tt.setup()
			SetRegistry(registry)
			Shutdown(tt.accType)

			accs, _ := Registry().Accelerators()
			for _, a := range accs {
				if a.IsRunning() {
					t.Errorf("expected accelerator to be stopped but it is still running")
				}
			}
		})
	}
}

func TestAcceleratorMethods(t *testing.T) {
	devType := device.DeviceType(0)
	acc := &accelerator{
		dev:     newMockDevice(),
		running: true,
		accType: AcceleratorType(0),
	}

	if got := acc.Device(); got.DevType() != devType {
		t.Errorf("expected device type %v, got %v", devType, got.DevType())
	}
	if got := acc.DeviceType(); got != devType {
		t.Errorf("expected device type %v, got %v", devType, got)
	}
	if got := acc.IsRunning(); !got {
		t.Errorf("expected accelerator to be running, got %v", got)
	}
	if got := acc.AccType(); got != AcceleratorType(0) {
		t.Errorf("expected accelerator type AcceleratorType(0), got %v", got)
	}
}
