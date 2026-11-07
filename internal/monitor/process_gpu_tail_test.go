// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sustainable-computing-io/kepler/internal/device/gpu"
	"github.com/sustainable-computing-io/kepler/internal/resource"
)

// tailMeter is a GPU meter that also reports the power read after a process exited.
type tailMeter struct {
	*MockGPUPowerMeter
	tail map[uint32]float64
}

func (m tailMeter) GetProcessTailPower() map[uint32]float64 { return m.tail }

// TestProcessPowerGPUTailOnTerminatedProcess checks that GPU power the meter reports as read
// after a process exited goes to its terminated record, even when the PID is running again,
// and that the rest goes to the running process.
func TestProcessPowerGPUTailOnTerminatedProcess(t *testing.T) {
	for _, tt := range []struct {
		name               string
		reused             bool
		tail               map[uint32]float64
		wantTerminatedGPU  Energy
		wantTerminatedWatt float64
		wantRunningWatt    float64
	}{
		{
			name:               "exited process gets the tail",
			tail:               map[uint32]float64{4001: 30},
			wantTerminatedGPU:  100*Joule + 30*5*Joule,
			wantTerminatedWatt: 30,
		},
		{
			name:               "exited process gets the tail when the PID is reused off the GPU",
			reused:             true,
			tail:               map[uint32]float64{4001: 30},
			wantTerminatedGPU:  100*Joule + 30*5*Joule,
			wantTerminatedWatt: 30,
			wantRunningWatt:    0,
		},
		{
			name:               "running process that reuses the PID on the GPU keeps its power",
			reused:             true,
			wantTerminatedGPU:  100 * Joule,
			wantTerminatedWatt: 20,
			wantRunningWatt:    30,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			zone := CreateTestZones()[0]

			exited := &Process{
				PID: 4001, StartTime: 100, Comm: "trainer", Exe: "/usr/bin/trainer",
				GPUPower: 20, GPUEnergyTotal: 100 * Joule,
				Zones: ZoneUsageMap{zone: {EnergyTotal: 50 * Joule}},
			}
			previous := NewSnapshot()
			previous.Node.Timestamp = time.Unix(1_000, 0)
			previous.Processes["4001"] = exited

			next := NewSnapshot()
			next.Node.Timestamp = previous.Node.Timestamp.Add(5 * time.Second)

			procs := &resource.Processes{
				Running:    map[int]*resource.Process{},
				Terminated: map[int]*resource.Process{4001: {PID: 4001, StartTime: 100}},
			}
			if tt.reused {
				procs.Running[4001] = &resource.Process{PID: 4001, StartTime: 900, Comm: "other", Exe: "/usr/bin/other"}
			}

			informer := &MockResourceInformer{}
			informer.On("Processes").Return(procs).Once()
			informer.On("Node").Return(&resource.Node{ProcessTotalCPUTimeDelta: 1}).Maybe()
			meter := &MockGPUPowerMeter{}
			meter.On("GetProcessPower").Return(map[uint32]float64{4001: 30}, nil).Once()
			meter.On("Devices").Return([]gpu.GPUDevice{})

			monitor := &PowerMonitor{
				logger:                     logger,
				resources:                  informer,
				gpuMeters:                  []gpu.GPUPowerMeter{tailMeter{meter, tt.tail}},
				terminatedProcessesTracker: NewTerminatedResourceTracker[*Process](zone, 10, 0, logger),
			}

			require.NoError(t, monitor.calculateProcessPower(previous, next))

			terminated := next.TerminatedProcessesForMetrics()
			require.Len(t, terminated, 1)
			for _, proc := range terminated {
				assert.Equal(t, tt.wantTerminatedGPU, proc.GPUEnergyTotal)
				assert.Equal(t, tt.wantTerminatedWatt, proc.GPUPower)
			}
			if tt.reused {
				require.Contains(t, next.Processes, "4001")
				assert.Equal(t, tt.wantRunningWatt, next.Processes["4001"].GPUPower)
			}
			assert.Equal(t, 100*Joule, previous.Processes["4001"].GPUEnergyTotal, "the previous snapshot is immutable")
		})
	}
}

// TestGPUTailReachesContainersAndPods checks that the GPU power of a process that has just
// exited also goes to its container and pod, whether they are still running or not.
func TestGPUTailReachesContainersAndPods(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	zone := CreateTestZones()[0]
	zones := func() ZoneUsageMap { return ZoneUsageMap{zone: {EnergyTotal: 50 * Joule}} }

	// c1 keeps running in p1, c2 ends but p1 keeps running, c3 ends with p2.
	previous := NewSnapshot()
	previous.Node.Timestamp = time.Unix(1_000, 0)
	for pid, cntr := range map[string]string{"4001": "c1", "4002": "c2", "4003": "c3"} {
		previous.Processes[pid] = &Process{PID: 0, StartTime: 100, ContainerID: cntr, GPUPower: 20, Zones: zones()}
	}
	previous.Containers["c1"] = &Container{ID: "c1", PodID: "p1", Zones: zones()}
	previous.Containers["c2"] = &Container{ID: "c2", PodID: "p1", Zones: zones()}
	previous.Containers["c3"] = &Container{ID: "c3", PodID: "p2", Zones: zones()}
	previous.Pods["p1"] = &Pod{ID: "p1", Zones: zones()}
	previous.Pods["p2"] = &Pod{ID: "p2", Zones: zones()}

	next := NewSnapshot()
	next.Node.Timestamp = previous.Node.Timestamp.Add(5 * time.Second)

	p1 := &resource.Pod{ID: "p1"}
	informer := &MockResourceInformer{}
	informer.On("Processes").Return(&resource.Processes{
		Running: map[int]*resource.Process{},
		Terminated: map[int]*resource.Process{
			4001: {PID: 4001, StartTime: 100}, 4002: {PID: 4002, StartTime: 100}, 4003: {PID: 4003, StartTime: 100},
		},
	})
	informer.On("Containers").Return(&resource.Containers{
		Running:    map[string]*resource.Container{"c1": {ID: "c1", Pod: p1}},
		Terminated: map[string]*resource.Container{"c2": {ID: "c2"}, "c3": {ID: "c3"}},
	})
	informer.On("Pods").Return(&resource.Pods{
		Running:    map[string]*resource.Pod{"p1": p1},
		Terminated: map[string]*resource.Pod{"p2": {ID: "p2"}},
	})
	informer.On("Node").Return(&resource.Node{ProcessTotalCPUTimeDelta: 1}).Maybe()
	meter := &MockGPUPowerMeter{}
	meter.On("GetProcessPower").Return(map[uint32]float64{4001: 10, 4002: 10, 4003: 10}, nil).Once()
	meter.On("Devices").Return([]gpu.GPUDevice{})

	monitor := &PowerMonitor{
		logger:                      logger,
		resources:                   informer,
		gpuMeters:                   []gpu.GPUPowerMeter{tailMeter{meter, map[uint32]float64{4001: 10, 4002: 10, 4003: 10}}},
		terminatedProcessesTracker:  NewTerminatedResourceTracker[*Process](zone, 10, 0, logger),
		terminatedContainersTracker: NewTerminatedResourceTracker[*Container](zone, 10, 0, logger),
		terminatedPodsTracker:       NewTerminatedResourceTracker[*Pod](zone, 10, 0, logger),
	}

	require.NoError(t, monitor.calculateProcessPower(previous, next))
	require.NoError(t, monitor.calculateContainerPower(previous, next))
	require.NoError(t, monitor.calculatePodPower(previous, next))

	require.Contains(t, next.Containers, "c1")
	assert.Equal(t, 10.0, next.Containers["c1"].GPUPower)
	assert.Equal(t, 50*Joule, next.Containers["c1"].GPUEnergyTotal)

	require.Len(t, next.TerminatedContainers, 2)
	for _, c := range next.TerminatedContainers {
		assert.Equal(t, 10.0, c.GPUPower, c.ID)
		assert.Equal(t, 50*Joule, c.GPUEnergyTotal, c.ID)
	}

	require.Contains(t, next.Pods, "p1")
	assert.Equal(t, 20.0, next.Pods["p1"].GPUPower)
	assert.Equal(t, 100*Joule, next.Pods["p1"].GPUEnergyTotal)

	require.Len(t, next.TerminatedPods, 1)
	for _, p := range next.TerminatedPods {
		assert.Equal(t, 10.0, p.GPUPower)
		assert.Equal(t, 50*Joule, p.GPUEnergyTotal)
	}

	assert.Zero(t, previous.Containers["c2"].GPUEnergyTotal, "the previous snapshot is immutable")
	assert.Zero(t, previous.Pods["p2"].GPUEnergyTotal, "the previous snapshot is immutable")
}
