// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package nvidia

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/sustainable-computing-io/kepler/internal/device"
	"github.com/sustainable-computing-io/kepler/internal/device/gpu"
)

// setDeviceReading replaces the mock device's expectations with one NVML reading.
func setDeviceReading(dev *MockNVMLDevice, watts float64, procs []gpu.ProcessGPUInfo, utils []gpu.ProcessUtilization) {
	dev.ExpectedCalls = nil
	dev.On("GetPowerUsage").Return(device.Power(watts)*device.Watt, nil)
	dev.On("UUID").Return("GPU-123")
	dev.On("GetComputeRunningProcesses").Return(procs, nil)
	dev.On("GetProcessUtilization", mock.Anything).Return(utils, nil)
}

func newTailTestCollector(backend NVMLBackend, mode gpu.SharingMode) *GPUPowerCollector {
	return &GPUPowerCollector{
		logger:           slog.Default(),
		nvml:             backend,
		devices:          []gpu.GPUDevice{{Index: 0, UUID: "GPU-123"}},
		sharingModes:     map[int]gpu.SharingMode{0: mode},
		minObservedPower: map[string]float64{},
		idleObserved:     map[string]bool{},
		tailShares:       map[tailKey]*tailShares{},
	}
}

func TestGPUPowerCollector_tailAfterProcessExit(t *testing.T) {
	t.Run("time slicing gives the first empty collection to the exited processes", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)
		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		collector := newTailTestCollector(mockBackend, gpu.SharingModeTimeSlicing)

		// Kepler starts under load, so no idle baseline yet: all 100 W are active.
		setDeviceReading(mockDevice, 100, []gpu.ProcessGPUInfo{{PID: 1001}, {PID: 1002}},
			[]gpu.ProcessUtilization{{PID: 1001, ComputeUtil: 60}, {PID: 1002, ComputeUtil: 40}})
		result, err := collector.GetProcessPower()
		require.NoError(t, err)
		assert.InDelta(t, 60.0, result[1001], 0.001)
		assert.InDelta(t, 40.0, result[1002], 0.001)

		// The processes exit, the GPU is still clocked up.
		setDeviceReading(mockDevice, 45, []gpu.ProcessGPUInfo{}, nil)
		result, err = collector.GetProcessPower()
		require.NoError(t, err)
		assert.InDelta(t, 27.0, result[1001], 0.001)
		assert.InDelta(t, 18.0, result[1002], 0.001)

		// The monitor reads the device stats after the process power: the tail reading must
		// not become the idle baseline.
		stats, err := collector.GetDevicePowerStats(0)
		require.NoError(t, err)
		assert.Equal(t, 45.0, stats.ActivePower)
		assert.False(t, collector.idleObserved["GPU-123"])

		// The next empty collection is not attributed, and is used for idle detection again.
		setDeviceReading(mockDevice, 11, []gpu.ProcessGPUInfo{}, nil)
		result, err = collector.GetProcessPower()
		require.NoError(t, err)
		assert.Empty(t, result)
		assert.True(t, collector.idleObserved["GPU-123"])
		assert.Equal(t, 11.0, collector.minObservedPower["GPU-123"])
		assert.Empty(t, collector.tailShares)
	})

	t.Run("exclusive mode drops the shares when another process shows up", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)
		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		collector := newTailTestCollector(mockBackend, gpu.SharingModeExclusive)
		collector.minObservedPower["GPU-123"] = 40
		collector.idleObserved["GPU-123"] = true

		setDeviceReading(mockDevice, 100, []gpu.ProcessGPUInfo{{PID: 1001}}, nil)
		result, err := collector.GetProcessPower()
		require.NoError(t, err)
		assert.Equal(t, 60.0, result[1001])

		setDeviceReading(mockDevice, 90, []gpu.ProcessGPUInfo{{PID: 2002}}, nil)
		result, err = collector.GetProcessPower()
		require.NoError(t, err)
		assert.Equal(t, map[uint32]float64{2002: 50}, result)

		setDeviceReading(mockDevice, 70, []gpu.ProcessGPUInfo{}, nil)
		result, err = collector.GetProcessPower()
		require.NoError(t, err)
		assert.Equal(t, map[uint32]float64{2002: 30}, result)
		assert.Equal(t, 40.0, collector.minObservedPower["GPU-123"])
	})

	t.Run("no shares without a previous attribution", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)
		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		collector := newTailTestCollector(mockBackend, gpu.SharingModeTimeSlicing)

		setDeviceReading(mockDevice, 50, []gpu.ProcessGPUInfo{}, nil)
		result, err := collector.GetProcessPower()
		require.NoError(t, err)
		assert.Empty(t, result)
		assert.True(t, collector.idleObserved["GPU-123"])
	})
}

func TestGPUPowerCollector_tailAfterProcessExit_partitioned(t *testing.T) {
	setup := func(t *testing.T) (*GPUPowerCollector, *MockNVMLDevice, *MockDCGMBackend, *MockNVMLDevice, *MockNVMLDevice) {
		t.Helper()
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)
		mockDCGM := new(MockDCGMBackend)
		mig1 := new(MockNVMLDevice)
		mig2 := new(MockNVMLDevice)
		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDCGM.On("IsInitialized").Return(true)

		collector := newTailTestCollector(mockBackend, gpu.SharingModePartitioned)
		collector.minObservedPower["GPU-123"] = 40
		collector.idleObserved["GPU-123"] = true
		collector.dcgm = mockDCGM
		collector.migInstancesByDevice = map[int][]MIGGPUInstance{
			0: {{ParentGPUIndex: 0, GPUInstanceID: 1}, {ParentGPUIndex: 0, GPUInstanceID: 2}},
		}
		return collector, mockDevice, mockDCGM, mig1, mig2
	}

	// reading sets the parent power and, per instance, its activity and process (none if activity is 0).
	reading := func(dev *MockNVMLDevice, dcgm *MockDCGMBackend, migs []*MockNVMLDevice, watts float64, activity []float64, pids []uint32) {
		var parentProcs []gpu.ProcessGPUInfo
		dcgm.ExpectedCalls = nil
		dcgm.On("IsInitialized").Return(true)
		for i, act := range activity {
			if act > 0 {
				parentProcs = append(parentProcs, gpu.ProcessGPUInfo{PID: pids[i]})
			}
		}
		setDeviceReading(dev, watts, parentProcs, nil)
		for i, act := range activity {
			id := uint(i + 1)
			dcgm.On("GetMIGInstanceActivity", 0, id).Return(act, nil)
			dev.On("GetMIGDeviceByInstanceID", id).Return(migs[i], nil)
			migs[i].ExpectedCalls = nil
			migs[i].On("GetProcessUtilization", uint64(0)).Return([]gpu.ProcessUtilization{{PID: pids[i], ComputeUtil: 50}}, nil)
		}
	}

	t.Run("the tail goes to the processes of all instances when every instance is empty", func(t *testing.T) {
		collector, dev, dcgm, mig1, mig2 := setup(t)
		migs := []*MockNVMLDevice{mig1, mig2}

		reading(dev, dcgm, migs, 100, []float64{0.5, 0.5}, []uint32{1001, 2001})
		result, err := collector.GetProcessPower()
		require.NoError(t, err)
		assert.Equal(t, map[uint32]float64{1001: 30, 2001: 30}, result)

		reading(dev, dcgm, migs, 70, []float64{0, 0}, []uint32{1001, 2001})
		result, err = collector.GetProcessPower()
		require.NoError(t, err)
		assert.Equal(t, map[uint32]float64{1001: 15, 2001: 15}, result)

		reading(dev, dcgm, migs, 60, []float64{0, 0}, []uint32{1001, 2001})
		result, err = collector.GetProcessPower()
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("an instance that went empty while another runs loses its shares", func(t *testing.T) {
		collector, dev, dcgm, mig1, mig2 := setup(t)
		migs := []*MockNVMLDevice{mig1, mig2}

		reading(dev, dcgm, migs, 100, []float64{0.5, 0.5}, []uint32{1001, 2001})
		_, err := collector.GetProcessPower()
		require.NoError(t, err)

		// Instance 1's process exits, instance 2 keeps running and gets the active power.
		reading(dev, dcgm, migs, 100, []float64{0, 0.5}, []uint32{1001, 2001})
		result, err := collector.GetProcessPower()
		require.NoError(t, err)
		assert.Equal(t, map[uint32]float64{2001: 60}, result)

		// Then instance 2's process exits too: only it gets the tail.
		reading(dev, dcgm, migs, 70, []float64{0, 0}, []uint32{1001, 2001})
		result, err = collector.GetProcessPower()
		require.NoError(t, err)
		assert.Equal(t, map[uint32]float64{2001: 30}, result)
	})
}
