package accelerator

import (
	"testing"

	"github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator/device"

	// Add supported devices.

	_ "github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator/device/sources"
)

func newMockDevice() device.Device {
	return device.Startup(AcceleratorType(0).String())
}

func cleanupMockDevice() {
	Shutdown()
}

func TestRegistry(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() *Registry
		expectedLen int
		expectError bool
		cleanup     func()
	}{
		{
			name: "Empty registry",
			setup: func() *Registry {
				registry := Registry{
					Registry: map[AcceleratorType]Accelerator{},
				}
				SetRegistry(&registry)

				return GetRegistry()
			},
			expectedLen: 0,
			expectError: false,
			cleanup:     func() {},
		},
		{
			name: "Non-empty registry",
			setup: func() *Registry {
				registry := &Registry{
					Registry: map[AcceleratorType]Accelerator{},
				}
				SetRegistry(registry)

				a := &accelerator{
					dev:     newMockDevice(),
					accType: AcceleratorType(0),
					running: true,
				}
				registry.MustRegister(a)

				return GetRegistry()
			},
			expectedLen: 1,
			expectError: false,
			cleanup:     func() { cleanupMockDevice() },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var a Accelerator
			var err error
			registry := tt.setup()
			if a, err = New(AcceleratorType(0), true); err == nil {
				registry.MustRegister(a) // Register the accelerator with the registry
			}
			accs := registry.Accelerators()
			if tt.expectError && err == nil {
				t.Errorf("expected an error but got nil")
			}
			if tt.expectError && err != nil {
				t.Errorf("did not expect an error but got %v", err)
			}
			if len(accs) != tt.expectedLen {
				t.Errorf("expected %d accelerators, but got %d", tt.expectedLen, len(accs))
			}
			tt.cleanup()
		})
	}
}

func TestActiveAcceleratorByType(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() *Registry
		accType     AcceleratorType
		expectError bool
		cleanup     func()
	}{
		{
			name: "No accelerators of given type",
			setup: func() *Registry {
				return &Registry{
					Registry: map[AcceleratorType]Accelerator{},
				}
			},
			accType:     AcceleratorType(0),
			expectError: true,
			cleanup:     func() {},
		},
		{
			name: "One active accelerator of given type",
			setup: func() *Registry {
				registry := &Registry{
					Registry: map[AcceleratorType]Accelerator{},
				}
				registry.Registry[AcceleratorType(0)] = &accelerator{
					dev:     newMockDevice(),
					accType: AcceleratorType(0),
					running: true,
				}
				return registry
			},
			accType:     AcceleratorType(0),
			expectError: false,
			cleanup:     func() { cleanupMockDevice() },
		},
		{
			name: "One inactive accelerator of given type",
			setup: func() *Registry {
				registry := &Registry{
					Registry: map[AcceleratorType]Accelerator{},
				}
				registry.Registry[AcceleratorType(0)] = &accelerator{
					dev:     newMockDevice(),
					accType: AcceleratorType(0),
					running: false,
				}
				return registry
			},
			accType:     AcceleratorType(0),
			expectError: true,
			cleanup:     func() {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := tt.setup()
			accs := registry.ActiveAcceleratorByType(tt.accType)
			if tt.expectError && accs != nil {
				t.Errorf("expected an error")
			}
			if !tt.expectError && accs == nil {
				t.Errorf("did not expect an error")
			}
			tt.cleanup()
		})
	}
}

func TestCreateAndRegister(t *testing.T) {
	tests := []struct {
		name        string
		accType     AcceleratorType
		setup       func() *Registry
		sleep       bool
		expectError bool
		cleanup     func()
	}{
		{
			name:    "Unsupported accelerator",
			accType: AcceleratorType(999), // invalid accelerator type
			setup: func() *Registry {
				return &Registry{
					Registry: map[AcceleratorType]Accelerator{},
				}
			},
			sleep:       false,
			expectError: true,
			cleanup:     func() {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := tt.setup()
			SetRegistry(registry)
			_, err := New(tt.accType, tt.sleep)
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
		setup   func() *Registry
		accType AcceleratorType
	}{
		{
			name: "Shutdown active accelerators",
			setup: func() *Registry {
				registry := &Registry{
					Registry: map[AcceleratorType]Accelerator{},
				}

				SetRegistry(registry)
				a := &accelerator{
					dev:     newMockDevice(),
					accType: AcceleratorType(0),
					running: true,
				}
				registry.MustRegister(a)
				return GetRegistry()
			},
			accType: AcceleratorType(0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := tt.setup()
			SetRegistry(registry)
			Shutdown()

			accs := GetRegistry().Accelerators()
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
	if got := acc.Device().DevType(); got != devType {
		t.Errorf("expected device type %v, got %v", devType, got)
	}
	if got := acc.IsRunning(); !got {
		t.Errorf("expected accelerator to be running, got %v", got)
	}
	if got := acc.AccType(); got != AcceleratorType(0) {
		t.Errorf("expected accelerator type AcceleratorType(0), got %v", got)
	}
}
