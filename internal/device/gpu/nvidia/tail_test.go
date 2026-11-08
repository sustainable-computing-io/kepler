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
		assert.Equal(t, result, collector.GetProcessTailPower(), "all of it is tail power")

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
		assert.Empty(t, collector.GetProcessTailPower())
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
			var running []gpu.ProcessGPUInfo
			if act > 0 {
				running = []gpu.ProcessGPUInfo{{PID: pids[i]}}
			}
			migs[i].On("GetComputeRunningProcesses").Return(running, nil)
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

	t.Run("a process between kernels keeps its shares", func(t *testing.T) {
		collector, dev, dcgm, mig1, mig2 := setup(t)
		migs := []*MockNVMLDevice{mig1, mig2}

		reading(dev, dcgm, migs, 100, []float64{0.5, 0}, []uint32{1001, 2001})
		_, err := collector.GetProcessPower()
		require.NoError(t, err)

		// No activity, but the process is still on the instance: nothing is attributed yet.
		reading(dev, dcgm, migs, 70, []float64{0, 0}, []uint32{1001, 2001})
		mig1.On("GetComputeRunningProcesses").Unset()
		mig1.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{{PID: 1001}}, nil)
		result, err := collector.GetProcessPower()
		require.NoError(t, err)
		assert.Empty(t, result)
		assert.Contains(t, collector.tailShares, migTailKey(0, 1))

		// A new process on the instance replaces the old one: its shares are dropped.
		mig1.On("GetComputeRunningProcesses").Unset()
		mig1.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{{PID: 3001}}, nil)
		result, err = collector.GetProcessPower()
		require.NoError(t, err)
		assert.Empty(t, result)
		assert.Empty(t, collector.tailShares)
	})

	t.Run("the tail of instances with shares from different collections is at most the active power", func(t *testing.T) {
		collector, dev, dcgm, mig1, mig2 := setup(t)
		migs := []*MockNVMLDevice{mig1, mig2}

		// Only instance 1 is active, so its process gets all of the active power.
		reading(dev, dcgm, migs, 100, []float64{0.5, 0}, []uint32{1001, 2001})
		_, err := collector.GetProcessPower()
		require.NoError(t, err)

		// Instance 1's process is between kernels and only instance 2 is active.
		reading(dev, dcgm, migs, 100, []float64{0, 0.5}, []uint32{1001, 2001})
		mig1.On("GetComputeRunningProcesses").Unset()
		mig1.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{{PID: 1001}}, nil)
		result, err := collector.GetProcessPower()
		require.NoError(t, err)
		assert.Equal(t, map[uint32]float64{2001: 60}, result)

		// Both processes exit: each instance has a share of 1, so they split the tail.
		reading(dev, dcgm, migs, 70, []float64{0, 0}, []uint32{1001, 2001})
		result, err = collector.GetProcessPower()
		require.NoError(t, err)
		assert.Equal(t, map[uint32]float64{1001: 15, 2001: 15}, result)
	})

	t.Run("a process that left an instance between kernels loses its share", func(t *testing.T) {
		collector, dev, dcgm, mig1, mig2 := setup(t)
		migs := []*MockNVMLDevice{mig1, mig2}

		// Two processes share instance 1.
		reading(dev, dcgm, migs, 100, []float64{0.5, 0}, []uint32{1001, 2001})
		mig1.On("GetProcessUtilization", uint64(0)).Unset()
		mig1.On("GetProcessUtilization", uint64(0)).Return([]gpu.ProcessUtilization{
			{PID: 1001, ComputeUtil: 50}, {PID: 1002, ComputeUtil: 50},
		}, nil)
		result, err := collector.GetProcessPower()
		require.NoError(t, err)
		assert.Equal(t, map[uint32]float64{1001: 30, 1002: 30}, result)

		// 1002 exits while 1001 is between kernels: 1001 takes over the instance's share.
		reading(dev, dcgm, migs, 70, []float64{0, 0}, []uint32{1001, 2001})
		mig1.On("GetComputeRunningProcesses").Unset()
		mig1.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{{PID: 1001}}, nil)
		result, err = collector.GetProcessPower()
		require.NoError(t, err)
		assert.Empty(t, result)
		assert.Equal(t, map[uint32]float64{1001: 1}, collector.tailShares[migTailKey(0, 1)].shares)

		// Then 1001 exits and gets the whole tail.
		reading(dev, dcgm, migs, 70, []float64{0, 0}, []uint32{1001, 2001})
		result, err = collector.GetProcessPower()
		require.NoError(t, err)
		assert.Equal(t, map[uint32]float64{1001: 30}, result)
	})

	t.Run("no tail while another instance has a process between kernels", func(t *testing.T) {
		collector, dev, dcgm, mig1, mig2 := setup(t)
		migs := []*MockNVMLDevice{mig1, mig2}

		reading(dev, dcgm, migs, 100, []float64{0.5, 0.5}, []uint32{1001, 2001})
		_, err := collector.GetProcessPower()
		require.NoError(t, err)

		// 1001 is between kernels and 2001 exits: 2001 must not get the whole tail.
		reading(dev, dcgm, migs, 70, []float64{0, 0}, []uint32{1001, 2001})
		mig1.On("GetComputeRunningProcesses").Unset()
		mig1.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{{PID: 1001}}, nil)
		result, err := collector.GetProcessPower()
		require.NoError(t, err)
		assert.Empty(t, result)
		assert.Contains(t, collector.tailShares, migTailKey(0, 1))
		assert.NotContains(t, collector.tailShares, migTailKey(0, 2))
	})

	t.Run("no tail when an instance cannot be read", func(t *testing.T) {
		collector, dev, dcgm, mig1, mig2 := setup(t)
		migs := []*MockNVMLDevice{mig1, mig2}

		reading(dev, dcgm, migs, 100, []float64{0.5, 0.5}, []uint32{1001, 2001})
		_, err := collector.GetProcessPower()
		require.NoError(t, err)

		reading(dev, dcgm, migs, 70, []float64{0, 0}, []uint32{1001, 2001})
		dcgm.On("GetMIGInstanceActivity", 0, uint(2)).Unset()
		dcgm.On("GetMIGInstanceActivity", 0, uint(2)).Return(0.0, assert.AnError)
		result, err := collector.GetProcessPower()
		require.NoError(t, err)
		assert.Empty(t, result)
		assert.Empty(t, collector.tailShares)
	})

	t.Run("switching to the fallback drops the instance shares", func(t *testing.T) {
		collector, dev, dcgm, mig1, mig2 := setup(t)
		migs := []*MockNVMLDevice{mig1, mig2}

		reading(dev, dcgm, migs, 100, []float64{0.5, 0.5}, []uint32{1001, 2001})
		_, err := collector.GetProcessPower()
		require.NoError(t, err)

		// DCGM goes away and the processes exit: the instance shares can't be used, so the
		// reading is used for idle detection.
		dcgm.ExpectedCalls = nil
		dcgm.On("IsInitialized").Return(false)
		setDeviceReading(dev, 30, []gpu.ProcessGPUInfo{}, nil)
		result, err := collector.GetProcessPower()
		require.NoError(t, err)
		assert.Empty(t, result)
		assert.Empty(t, collector.tailShares)
		assert.Equal(t, 30.0, collector.minObservedPower["GPU-123"])
	})
}
