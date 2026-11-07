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

// TestProcessPowerGPUTailOnTerminatedProcess checks that GPU power reported for a process that
// has just exited goes to its terminated record, unless the PID is already running again.
func TestProcessPowerGPUTailOnTerminatedProcess(t *testing.T) {
	for _, tt := range []struct {
		name               string
		reused             bool
		wantTerminatedGPU  Energy
		wantTerminatedWatt float64
	}{
		{
			name:               "exited process gets the tail",
			wantTerminatedGPU:  100*Joule + 30*5*Joule,
			wantTerminatedWatt: 30,
		},
		{
			name:               "running process that reuses the PID keeps the power",
			reused:             true,
			wantTerminatedGPU:  100 * Joule,
			wantTerminatedWatt: 20,
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
				gpuMeters:                  []gpu.GPUPowerMeter{meter},
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
				assert.Equal(t, 30.0, next.Processes["4001"].GPUPower)
			}
			assert.Equal(t, 100*Joule, previous.Processes["4001"].GPUEnergyTotal, "the previous snapshot is immutable")
		})
	}
}
