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

import (
	"testing"

	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator/devices"
)

func newMockDevice() devices.Device {
	return devices.Startup(devices.MOCK.String())
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
					Registry: map[string]Accelerator{},
				}
				SetRegistry(&registry)

				return GetRegistry()
			},
			expectedLen: 0,
			expectError: false,
			cleanup:     func() { cleanupMockDevice() },
		},
		{
			name: "Non-empty registry",
			setup: func() *Registry {
				registry := &Registry{
					Registry: map[string]Accelerator{},
				}
				SetRegistry(registry)
				devices.RegisterMockDevice()
				a := &accelerator{
					dev:     newMockDevice(),
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

	if _, err := config.Initialize("."); err != nil {
		t.Fatal(err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var a Accelerator
			var err error
			registry := tt.setup()
			if a, err = New("MOCK", true); err == nil {
				registry.MustRegister(a) // Register the accelerator with the registry
			}
			accs := GetAccelerators()
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
		expectError bool
		cleanup     func()
	}{
		{
			name: "No accelerators of given type",
			setup: func() *Registry {
				return &Registry{
					Registry: map[string]Accelerator{},
				}
			},
			expectError: true,
			cleanup:     func() { cleanupMockDevice() },
		},
		{
			name: "One active accelerator of given type",
			setup: func() *Registry {
				registry := &Registry{
					Registry: map[string]Accelerator{},
				}
				SetRegistry(registry)
				devices.RegisterMockDevice()
				a := &accelerator{
					dev:     newMockDevice(),
					running: true,
				}
				registry.MustRegister(a)

				return GetRegistry()
			},
			expectError: false,
			cleanup:     func() { cleanupMockDevice() },
		},
	}

	if _, err := config.Initialize("."); err != nil {
		t.Fatal(err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			accs := GetActiveAcceleratorByType("MOCK")
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
		accType     string
		setup       func() *Registry
		sleep       bool
		expectError bool
		cleanup     func()
	}{
		{
			name:    "Unsupported accelerator",
			accType: "UNSUPPORTED", // invalid accelerator type
			setup: func() *Registry {
				return &Registry{
					Registry: map[string]Accelerator{},
				}
			},
			sleep:       false,
			expectError: true,
			cleanup:     func() { cleanupMockDevice() },
		},
	}

	if _, err := config.Initialize("."); err != nil {
		t.Fatal(err)
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
		name  string
		setup func() *Registry
	}{
		{
			name: "Shutdown active accelerators",
			setup: func() *Registry {
				registry := &Registry{
					Registry: map[string]Accelerator{},
				}

				SetRegistry(registry)
				devices.RegisterMockDevice()
				a := &accelerator{
					dev:     newMockDevice(),
					running: true,
				}
				registry.MustRegister(a)
				return GetRegistry()
			},
		},
	}
	if _, err := config.Initialize("."); err != nil {
		t.Fatal(err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			Shutdown()

			accs := GetAccelerators()
			for _, a := range accs {
				if a.IsRunning() {
					t.Errorf("expected accelerator to be stopped but it is still running")
				}
			}
		})
	}
}

func TestAcceleratorMethods(t *testing.T) {
	if _, err := config.Initialize("."); err != nil {
		t.Fatal(err)
	}
	registry := &Registry{
		Registry: map[string]Accelerator{},
	}

	SetRegistry(registry)

	devices.RegisterMockDevice()

	acc := &accelerator{
		dev:     newMockDevice(),
		running: true,
	}
	registry.MustRegister(acc)

	devType := acc.dev.HwType()

	if got := acc.Device(); got.HwType() != devType {
		t.Errorf("expected device type %v, got %v", devType, got.DevType())
	}
	if got := acc.Device().HwType(); got != devType {
		t.Errorf("expected device type %v, got %v", devType, got)
	}
	if got := acc.IsRunning(); !got {
		t.Errorf("expected accelerator to be running, got %v", got)
	}
	Shutdown()
}
