// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package esmi

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/sustainable-computing-io/kepler/internal/device"
)

// MockReader is a mock implementation of Reader for testing
type MockReader struct {
	mock.Mock
}

func (m *MockReader) Zones() ([]device.EnergyZone, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]device.EnergyZone), args.Error(1)
}

// MockZone is a mock implementation of EnergyZone for testing
type MockZone struct {
	mock.Mock
	name  string
	index int
}

func (m *MockZone) Index() int {
	return m.index
}

func (m *MockZone) Path() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockZone) Name() string {
	return m.name
}

func (m *MockZone) MaxEnergy() device.Energy {
	args := m.Called()
	return args.Get(0).(device.Energy)
}

func (m *MockZone) Energy() (device.Energy, error) {
	args := m.Called()
	return args.Get(0).(device.Energy), args.Error(1)
}

func (m *MockZone) Power() (device.Power, error) {
	args := m.Called()
	return args.Get(0).(device.Power), args.Error(1)
}

func TestPowerMeter_Name(t *testing.T) {
	mockReader := new(MockReader)

	meter := &PowerMeter{
		reader:      mockReader,
		logger:      slog.Default(),
		backendName: "test-backend",
	}

	name := meter.Name()
	assert.Equal(t, "esmi/test-backend", name)
}

func TestPowerMeter_Init_Success(t *testing.T) {
	mockReader := new(MockReader)
	mockZone := &MockZone{name: "package-0", index: 0}

	mockZone.On("Energy").Return(device.Energy(1000*device.Joule), nil)
	mockReader.On("Zones").Return([]device.EnergyZone{mockZone}, nil)

	meter := &PowerMeter{
		reader:      mockReader,
		logger:      slog.Default(),
		backendName: "test",
	}

	err := meter.Init()
	assert.NoError(t, err)
	mockReader.AssertExpectations(t)
	mockZone.AssertExpectations(t)
}

func TestPowerMeter_Init_NoZones(t *testing.T) {
	mockReader := new(MockReader)
	mockReader.On("Zones").Return([]device.EnergyZone{}, nil)

	meter := &PowerMeter{
		reader:      mockReader,
		logger:      slog.Default(),
		backendName: "test",
	}

	err := meter.Init()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no energy zones found")
	mockReader.AssertExpectations(t)
}

func TestPowerMeter_Init_ZonesError(t *testing.T) {
	mockReader := new(MockReader)
	mockReader.On("Zones").Return(nil, errors.New("zones failed"))

	meter := &PowerMeter{
		reader:      mockReader,
		logger:      slog.Default(),
		backendName: "test",
	}

	err := meter.Init()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to enumerate zones")
	mockReader.AssertExpectations(t)
}

func TestPowerMeter_Zones(t *testing.T) {
	mockReader := new(MockReader)
	mockZone1 := &MockZone{name: "package-0", index: 0}
	mockZone2 := &MockZone{name: "core-0", index: 0}

	expectedZones := []device.EnergyZone{mockZone1, mockZone2}
	mockReader.On("Zones").Return(expectedZones, nil)

	meter := &PowerMeter{
		reader:      mockReader,
		logger:      slog.Default(),
		backendName: "test",
	}

	zones, err := meter.Zones()
	assert.NoError(t, err)
	assert.Len(t, zones, 2)
	mockReader.AssertExpectations(t)
}

func TestPowerMeter_Zones_Cached(t *testing.T) {
	mockReader := new(MockReader)
	mockZone := &MockZone{name: "package-0", index: 0}

	expectedZones := []device.EnergyZone{mockZone}
	mockReader.On("Zones").Return(expectedZones, nil).Once()

	meter := &PowerMeter{
		reader:      mockReader,
		logger:      slog.Default(),
		backendName: "test",
	}

	// First call
	zones1, err1 := meter.Zones()
	assert.NoError(t, err1)
	assert.Equal(t, expectedZones, zones1)

	// Second call should return cached zones
	zones2, err2 := meter.Zones()
	assert.NoError(t, err2)
	assert.Equal(t, expectedZones, zones2)

	// Zones() should only be called once (caching works)
	mockReader.AssertExpectations(t)
}

func TestPowerMeter_PrimaryEnergyZone(t *testing.T) {
	mockReader := new(MockReader)
	mockZone1 := &MockZone{name: "core-0", index: 0}
	mockZone2 := &MockZone{name: "package-0", index: 0}

	// package should be selected as primary over core
	zones := []device.EnergyZone{mockZone1, mockZone2}
	mockReader.On("Zones").Return(zones, nil)

	meter := &PowerMeter{
		reader:      mockReader,
		logger:      slog.Default(),
		backendName: "test",
		cachedZones: zones,
		topZone:     mockZone2, // Pre-set the topZone to avoid probing
	}

	primaryZone, err := meter.PrimaryEnergyZone()
	assert.NoError(t, err)
	assert.Equal(t, "package-0", primaryZone.Name())
}

func TestPowerMeter_WithZoneFilter(t *testing.T) {
	mockReader := new(MockReader)

	meter, err := NewCPUPowerMeter("/sys",
		WithReader(mockReader),
		WithZoneFilter([]string{"package", "dimm"}),
	)

	assert.NoError(t, err)
	assert.NotNil(t, meter)
	assert.Equal(t, []string{"package", "dimm"}, meter.zoneFilter)
}

func TestPowerMeter_WithLogger(t *testing.T) {
	mockReader := new(MockReader)
	customLogger := slog.Default().With("test", "value")

	meter, err := NewCPUPowerMeter("/sys",
		WithReader(mockReader),
		WithLogger(customLogger),
	)

	assert.NoError(t, err)
	assert.NotNil(t, meter)
	assert.NotNil(t, meter.logger)
}
