// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package gpu

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sustainable-computing-io/kepler/internal/device"
)

// mockGPUPowerMeter is a test implementation of GPUPowerMeter
type mockGPUPowerMeter struct {
	vendor      Vendor
	devices     []GPUDevice
	initErr     error
	initialized bool
	shutdown    bool
}

func (m *mockGPUPowerMeter) Name() string { return "mock-" + string(m.vendor) }

func (m *mockGPUPowerMeter) Init() error {
	if m.initErr != nil {
		return m.initErr
	}
	m.initialized = true
	return nil
}

func (m *mockGPUPowerMeter) Shutdown() error {
	m.shutdown = true
	return nil
}

func (m *mockGPUPowerMeter) Vendor() Vendor {
	return m.vendor
}

func (m *mockGPUPowerMeter) Devices() []GPUDevice {
	return m.devices
}

func (m *mockGPUPowerMeter) GetPowerUsage(_ int) (device.Power, error) {
	return 0, nil
}

func (m *mockGPUPowerMeter) GetTotalEnergy(_ int) (device.Energy, error) {
	return 0, nil
}

func (m *mockGPUPowerMeter) GetDevicePowerStats(_ int) (GPUPowerStats, error) {
	return GPUPowerStats{}, nil
}

func (m *mockGPUPowerMeter) GetProcessPower() (map[uint32]float64, error) {
	return nil, nil
}

func (m *mockGPUPowerMeter) GetProcessInfo() ([]ProcessGPUInfo, error) {
	return nil, nil
}

// Verify mockGPUPowerMeter implements GPUPowerMeter
var _ GPUPowerMeter = (*mockGPUPowerMeter)(nil)

func TestRegisterAndDiscoverAll(t *testing.T) {
	// Clear registry before test
	ClearRegistry()
	defer ClearRegistry()

	logger := slog.Default()

	// Register a mock NVIDIA backend with devices
	nvidiaMeter := &mockGPUPowerMeter{
		vendor: VendorNVIDIA,
		devices: []GPUDevice{
			{Index: 0, UUID: "GPU-123", Name: "Test GPU", Vendor: VendorNVIDIA},
		},
	}
	Register(VendorNVIDIA, func(_ *slog.Logger) (GPUPowerMeter, error) {
		return nvidiaMeter, nil
	})

	// Discover all
	meters := DiscoverAll(logger)

	require.Len(t, meters, 1)
	assert.Equal(t, VendorNVIDIA, meters[0].Vendor())
	assert.True(t, nvidiaMeter.initialized, "meter should be initialized")
}

func TestDiscoverAllSkipsFailedInit(t *testing.T) {
	ClearRegistry()
	defer ClearRegistry()

	logger := slog.Default()

	// Register backend that fails init
	Register(VendorAMD, func(_ *slog.Logger) (GPUPowerMeter, error) {
		return &mockGPUPowerMeter{
			vendor:  VendorAMD,
			initErr: errors.New("driver not available"),
		}, nil
	})

	meters := DiscoverAll(logger)
	assert.Empty(t, meters)
}

func TestDiscoverAllSkipsNoDevices(t *testing.T) {
	ClearRegistry()
	defer ClearRegistry()

	logger := slog.Default()

	// Register backend with no devices
	noDevicesMeter := &mockGPUPowerMeter{
		vendor:  VendorIntel,
		devices: []GPUDevice{}, // empty
	}
	Register(VendorIntel, func(_ *slog.Logger) (GPUPowerMeter, error) {
		return noDevicesMeter, nil
	})

	meters := DiscoverAll(logger)
	assert.Empty(t, meters)
	assert.True(t, noDevicesMeter.shutdown, "meter with no devices should be shutdown")
}

func TestDiscoverAllSkipsFactoryError(t *testing.T) {
	ClearRegistry()
	defer ClearRegistry()

	logger := slog.Default()

	// Register factory that returns error
	Register(VendorNVIDIA, func(_ *slog.Logger) (GPUPowerMeter, error) {
		return nil, errors.New("NVML not available")
	})

	meters := DiscoverAll(logger)
	assert.Empty(t, meters)
}

func TestDiscover(t *testing.T) {
	ClearRegistry()
	defer ClearRegistry()

	logger := slog.Default()

	// Register NVIDIA
	Register(VendorNVIDIA, func(_ *slog.Logger) (GPUPowerMeter, error) {
		return &mockGPUPowerMeter{
			vendor: VendorNVIDIA,
			devices: []GPUDevice{
				{Index: 0, UUID: "GPU-456", Name: "A100", Vendor: VendorNVIDIA},
			},
		}, nil
	})

	// Discover specific vendor
	meter := Discover(VendorNVIDIA, logger)
	require.NotNil(t, meter)
	assert.Equal(t, VendorNVIDIA, meter.Vendor())

	// Discover unregistered vendor
	meter = Discover(VendorAMD, logger)
	assert.Nil(t, meter)
}

func TestRegisteredVendors(t *testing.T) {
	ClearRegistry()
	defer ClearRegistry()

	// Initially empty
	vendors := RegisteredVendors()
	assert.Empty(t, vendors)

	// Register some vendors
	Register(VendorNVIDIA, func(_ *slog.Logger) (GPUPowerMeter, error) {
		return nil, nil
	})
	Register(VendorAMD, func(_ *slog.Logger) (GPUPowerMeter, error) {
		return nil, nil
	})

	vendors = RegisteredVendors()
	assert.Len(t, vendors, 2)
	assert.Contains(t, vendors, VendorNVIDIA)
	assert.Contains(t, vendors, VendorAMD)
}

func TestClearRegistry(t *testing.T) {
	ClearRegistry()

	Register(VendorNVIDIA, func(_ *slog.Logger) (GPUPowerMeter, error) {
		return nil, nil
	})
	require.Len(t, RegisteredVendors(), 1)

	ClearRegistry()
	assert.Empty(t, RegisteredVendors())
}
