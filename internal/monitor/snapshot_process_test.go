// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/sustainable-computing-io/kepler/internal/resource"
)

func TestSnapshotProcessesForCPUTime(t *testing.T) {
	for _, tt := range []struct {
		name         string
		change       func(*Process)
		noRunning    bool
		noTerminated bool
	}{
		{name: "empty snapshot", noRunning: true, noTerminated: true},
		{name: "running only", noTerminated: true},
		{name: "terminated only", noRunning: true},
		{name: "reused PID with unchanged metadata"},
		{name: "different PID", change: func(p *Process) { p.PID = 4002 }},
		{name: "different comm", change: func(p *Process) { p.Comm = "new-process" }},
		{name: "different executable", change: func(p *Process) { p.Exe = "/usr/bin/new-process" }},
		{name: "different type", change: func(p *Process) { p.Type = resource.ContainerProcess }},
		{name: "different container", change: func(p *Process) { p.ContainerID = "new-container" }},
		{name: "different VM", change: func(p *Process) { p.VirtualMachineID = "new-vm" }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			terminated := &Process{
				PID: 4001, StartTime: 100, Comm: "python3", Exe: "/usr/bin/python3",
				Type: resource.RegularProcess, CPUTotalTime: 20,
			}
			running := terminated.Clone()
			running.StartTime = 900
			running.CPUTotalTime = 1
			if tt.change != nil {
				tt.change(running)
			}
			snapshot := NewSnapshot()
			var expected []*Process
			if !tt.noRunning {
				snapshot.Processes[running.StringID()] = running
				expected = append(expected, running)
			}
			if !tt.noTerminated {
				snapshot.TerminatedProcesses[terminated.lifetimeID()] = terminated
				older := running.Clone()
				older.StartTime = 50
				snapshot.TerminatedProcesses[older.lifetimeID()] = older
				if tt.noRunning || tt.change != nil {
					expected = append(expected, terminated)
				}
			}
			before := snapshot.Clone()

			assert.ElementsMatch(t, expected, snapshot.ProcessesForCPUTime())
			assert.Equal(t, before, snapshot.Clone(), "CPU-time selection must preserve both process maps")
		})
	}
}

func TestSnapshotTerminatedProcessesForMetrics(t *testing.T) {
	t.Run("empty snapshot", func(t *testing.T) {
		assert.Empty(t, NewSnapshot().TerminatedProcessesForMetrics())
	})

	for _, tc := range []struct {
		name       string
		change     func(*Process)
		noGPU      bool
		concurrent bool
	}{
		{name: "identical labels", concurrent: true},
		{name: "latest lifetime has no GPU energy", noGPU: true},
		{name: "different PID", change: func(p *Process) { p.PID = 4002 }},
		{name: "different command", change: func(p *Process) { p.Comm = "other" }},
		{name: "different executable", change: func(p *Process) { p.Exe = "/other" }},
		{name: "different type", change: func(p *Process) { p.Type = resource.VMProcess }},
		{name: "different container", change: func(p *Process) { p.ContainerID = "other" }},
		{name: "different VM", change: func(p *Process) { p.VirtualMachineID = "other" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			zones := CreateTestZones()
			old := &Process{
				PID: 4001, StartTime: 100, Comm: "python3", Exe: "/python3", Type: resource.ContainerProcess,
				ContainerID: "container", CPUTotalTime: 20, GPUPower: 5, GPUEnergyTotal: 100 * Joule,
				Zones: ZoneUsageMap{
					zones[0]: {EnergyTotal: 50 * Joule, Power: 5 * Watt},
					zones[1]: {EnergyTotal: 10 * Joule, Power: Watt},
				},
			}
			newer := old.Clone()
			newer.StartTime, newer.CPUTotalTime = 900, 1
			newer.GPUPower, newer.GPUEnergyTotal = 7, 130*Joule
			expectedGPU := 130 * Joule
			if tc.noGPU {
				newer.GPUPower, newer.GPUEnergyTotal = 0, 0
				expectedGPU = old.GPUEnergyTotal
			}
			newer.Zones[zones[0]] = Usage{EnergyTotal: 75 * Joule, Power: 7 * Watt}
			newer.Zones[zones[1]] = Usage{EnergyTotal: 15 * Joule, Power: 2 * Watt}
			if tc.change != nil {
				tc.change(newer)
			}
			snapshot := NewSnapshot()
			snapshot.TerminatedProcesses[old.lifetimeID()] = old
			snapshot.TerminatedProcesses[newer.lifetimeID()] = newer
			before := snapshot.Clone()
			check := func() {
				view := snapshot.TerminatedProcessesForMetrics()
				var packageEnergy, coreEnergy Energy
				for _, process := range view {
					packageEnergy += process.Zones[zones[0]].EnergyTotal
					coreEnergy += process.Zones[zones[1]].EnergyTotal
				}
				assert.Equal(t, 125*Joule, packageEnergy)
				assert.Equal(t, 25*Joule, coreEnergy)
				if tc.change == nil {
					assert.Len(t, view, 1)
					for _, process := range view {
						assert.Equal(t, uint64(900), process.StartTime)
						assert.Equal(t, float64(1), process.CPUTotalTime)
						assert.Equal(t, 7*Watt, process.Zones[zones[0]].Power)
						assert.Equal(t, 2*Watt, process.Zones[zones[1]].Power)
						assert.Equal(t, expectedGPU, process.GPUEnergyTotal)
						assert.Equal(t, newer.GPUPower, process.GPUPower)
					}
					assert.Equal(t, []*Process{newer}, snapshot.ProcessesForCPUTime())
				} else {
					assert.Equal(t, Processes{old.lifetimeID(): old, newer.lifetimeID(): newer}, view)
					assert.ElementsMatch(t, []*Process{old, newer}, snapshot.ProcessesForCPUTime())
				}
			}
			var readers sync.WaitGroup
			if tc.concurrent {
				readers.Go(check)
			}
			check()
			readers.Wait()
			assert.Equal(t, before, snapshot)
			// The projected view owns its zone maps.
			for _, p := range snapshot.TerminatedProcessesForMetrics() {
				p.Zones[zones[0]] = Usage{}
			}
			assert.Equal(t, before, snapshot)
		})
	}
}
