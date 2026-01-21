// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package nvidia

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/sustainable-computing-io/kepler/internal/device/gpu"
)

func TestNewSharingModeDetector(t *testing.T) {
	t.Run("with logger", func(t *testing.T) {
		logger := slog.Default()
		mockNVML := new(MockNVMLBackend)

		detector := NewSharingModeDetector(logger, mockNVML)

		assert.NotNil(t, detector)
	})

	t.Run("with nil logger uses default", func(t *testing.T) {
		mockNVML := new(MockNVMLBackend)

		detector := NewSharingModeDetector(nil, mockNVML)

		assert.NotNil(t, detector)
	})
}

func TestSharingModeDetector_DetectMode(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*MockNVMLBackend, *MockNVMLDevice)
		deviceIndex   int
		expectedMode  gpu.SharingMode
		expectedError bool
	}{
		{
			name: "MIG enabled returns partitioned mode",
			setupMock: func(backend *MockNVMLBackend, device *MockNVMLDevice) {
				backend.On("GetDevice", 0).Return(device, nil)
				device.On("IsMIGEnabled").Return(true, nil)
			},
			deviceIndex:   0,
			expectedMode:  gpu.SharingModePartitioned,
			expectedError: false,
		},
		{
			name: "exclusive process mode",
			setupMock: func(backend *MockNVMLBackend, device *MockNVMLDevice) {
				backend.On("GetDevice", 0).Return(device, nil)
				device.On("IsMIGEnabled").Return(false, nil)
				device.On("GetComputeMode").Return(ComputeModeExclusiveProcess, nil)
			},
			deviceIndex:   0,
			expectedMode:  gpu.SharingModeExclusive,
			expectedError: false,
		},
		{
			name: "exclusive thread mode",
			setupMock: func(backend *MockNVMLBackend, device *MockNVMLDevice) {
				backend.On("GetDevice", 0).Return(device, nil)
				device.On("IsMIGEnabled").Return(false, nil)
				device.On("GetComputeMode").Return(ComputeModeExclusiveThread, nil)
			},
			deviceIndex:   0,
			expectedMode:  gpu.SharingModeExclusive,
			expectedError: false,
		},
		{
			name: "default mode returns time slicing",
			setupMock: func(backend *MockNVMLBackend, device *MockNVMLDevice) {
				backend.On("GetDevice", 0).Return(device, nil)
				device.On("IsMIGEnabled").Return(false, nil)
				device.On("GetComputeMode").Return(ComputeModeDefault, nil)
			},
			deviceIndex:   0,
			expectedMode:  gpu.SharingModeTimeSlicing,
			expectedError: false,
		},
		{
			name: "prohibited mode returns time slicing",
			setupMock: func(backend *MockNVMLBackend, device *MockNVMLDevice) {
				backend.On("GetDevice", 0).Return(device, nil)
				device.On("IsMIGEnabled").Return(false, nil)
				device.On("GetComputeMode").Return(ComputeModeProhibited, nil)
			},
			deviceIndex:   0,
			expectedMode:  gpu.SharingModeTimeSlicing,
			expectedError: false,
		},
		{
			name: "device not found returns error",
			setupMock: func(backend *MockNVMLBackend, device *MockNVMLDevice) {
				backend.On("GetDevice", 99).Return(nil, gpu.ErrGPUNotFound{DeviceIndex: 99})
			},
			deviceIndex:   99,
			expectedMode:  gpu.SharingModeUnknown,
			expectedError: true,
		},
		{
			name: "MIG check error defaults to disabled",
			setupMock: func(backend *MockNVMLBackend, device *MockNVMLDevice) {
				backend.On("GetDevice", 0).Return(device, nil)
				device.On("IsMIGEnabled").Return(false, errors.New("MIG check failed"))
				device.On("GetComputeMode").Return(ComputeModeDefault, nil)
			},
			deviceIndex:   0,
			expectedMode:  gpu.SharingModeTimeSlicing,
			expectedError: false,
		},
		{
			name: "compute mode error defaults to time slicing",
			setupMock: func(backend *MockNVMLBackend, device *MockNVMLDevice) {
				backend.On("GetDevice", 0).Return(device, nil)
				device.On("IsMIGEnabled").Return(false, nil)
				device.On("GetComputeMode").Return(ComputeModeDefault, errors.New("compute mode failed"))
			},
			deviceIndex:   0,
			expectedMode:  gpu.SharingModeTimeSlicing,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockBackend := new(MockNVMLBackend)
			mockDevice := new(MockNVMLDevice)
			tt.setupMock(mockBackend, mockDevice)

			detector := NewSharingModeDetector(nil, mockBackend)
			mode, err := detector.DetectMode(tt.deviceIndex)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedMode, mode)

			mockBackend.AssertExpectations(t)
			mockDevice.AssertExpectations(t)
		})
	}
}

func TestSharingModeDetector_DetectAllModes(t *testing.T) {
	t.Run("multiple devices with different modes", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice0 := new(MockNVMLDevice)
		mockDevice1 := new(MockNVMLDevice)

		mockBackend.On("DeviceCount").Return(2)
		mockBackend.On("GetDevice", 0).Return(mockDevice0, nil)
		mockBackend.On("GetDevice", 1).Return(mockDevice1, nil)

		// Device 0: MIG enabled
		mockDevice0.On("IsMIGEnabled").Return(true, nil)

		// Device 1: Default mode (time-slicing)
		mockDevice1.On("IsMIGEnabled").Return(false, nil)
		mockDevice1.On("GetComputeMode").Return(ComputeModeDefault, nil)

		detector := NewSharingModeDetector(nil, mockBackend)
		modes, err := detector.DetectAllModes()

		assert.NoError(t, err)
		assert.Len(t, modes, 2)
		assert.Equal(t, gpu.SharingModePartitioned, modes[0])
		assert.Equal(t, gpu.SharingModeTimeSlicing, modes[1])

		mockBackend.AssertExpectations(t)
		mockDevice0.AssertExpectations(t)
		mockDevice1.AssertExpectations(t)
	})

	t.Run("device error sets unknown mode", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)

		mockBackend.On("DeviceCount").Return(1)
		mockBackend.On("GetDevice", 0).Return(nil, gpu.ErrGPUNotFound{DeviceIndex: 0})

		detector := NewSharingModeDetector(nil, mockBackend)
		modes, err := detector.DetectAllModes()

		assert.NoError(t, err)
		assert.Len(t, modes, 1)
		assert.Equal(t, gpu.SharingModeUnknown, modes[0])

		mockBackend.AssertExpectations(t)
	})

	t.Run("zero devices", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockBackend.On("DeviceCount").Return(0)

		detector := NewSharingModeDetector(nil, mockBackend)
		modes, err := detector.DetectAllModes()

		assert.NoError(t, err)
		assert.Empty(t, modes)

		mockBackend.AssertExpectations(t)
	})
}

func TestSharingModeDetector_Refresh(t *testing.T) {
	mockBackend := new(MockNVMLBackend)
	mockDevice := new(MockNVMLDevice)

	mockBackend.On("DeviceCount").Return(1)
	mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
	mockDevice.On("IsMIGEnabled").Return(false, nil)
	mockDevice.On("GetComputeMode").Return(ComputeModeDefault, nil)

	detector := NewSharingModeDetector(nil, mockBackend)
	err := detector.Refresh()

	assert.NoError(t, err)

	mockBackend.AssertExpectations(t)
	mockDevice.AssertExpectations(t)
}

func TestSharingModeDetector_GetCachedMode(t *testing.T) {
	mockBackend := new(MockNVMLBackend)
	mockDevice := new(MockNVMLDevice)

	mockBackend.On("DeviceCount").Return(1)
	mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
	mockDevice.On("IsMIGEnabled").Return(true, nil)

	detector := NewSharingModeDetector(nil, mockBackend).(*sharingModeDetector)

	// Before detection, should return unknown
	mode := detector.GetCachedMode(0)
	assert.Equal(t, gpu.SharingModeUnknown, mode)

	// After detection, should return cached value
	_, _ = detector.DetectAllModes()
	mode = detector.GetCachedMode(0)
	assert.Equal(t, gpu.SharingModePartitioned, mode)

	// Non-existent device should return unknown
	mode = detector.GetCachedMode(99)
	assert.Equal(t, gpu.SharingModeUnknown, mode)

	mockBackend.AssertExpectations(t)
	mockDevice.AssertExpectations(t)
}

// Ensure SharingModeDetector interface is implemented
var _ SharingModeDetector = (*sharingModeDetector)(nil)

// MockSharingModeDetector for collector tests
type MockSharingModeDetector struct {
	mock.Mock
}

func (m *MockSharingModeDetector) DetectMode(deviceIndex int) (gpu.SharingMode, error) {
	args := m.Called(deviceIndex)
	return args.Get(0).(gpu.SharingMode), args.Error(1)
}

func (m *MockSharingModeDetector) DetectAllModes() (map[int]gpu.SharingMode, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[int]gpu.SharingMode), args.Error(1)
}

func (m *MockSharingModeDetector) Refresh() error {
	args := m.Called()
	return args.Error(0)
}

var _ SharingModeDetector = (*MockSharingModeDetector)(nil)
