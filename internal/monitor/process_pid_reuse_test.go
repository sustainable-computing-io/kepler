// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sustainable-computing-io/kepler/internal/device"
	"github.com/sustainable-computing-io/kepler/internal/resource"
	testingclock "k8s.io/utils/clock/testing"
)

// fakeProcfs is a synthetic procfs directory for the real resource informer
type fakeProcfs struct {
	t          *testing.T
	root       string
	busy, idle uint64 // cumulative CPU ticks of the node
}

func newFakeProcfs(t *testing.T) *fakeProcfs {
	t.Helper()
	return &fakeProcfs{t: t, root: t.TempDir()}
}

func (f *fakeProcfs) write(rel, content string) {
	f.t.Helper()
	path := filepath.Join(f.root, rel)
	require.NoError(f.t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(f.t, os.WriteFile(path, []byte(content), 0o600))
}

// tick adds one interval of node CPU time, half of it busy, so the informer reports a usage ratio of 0.5
func (f *fakeProcfs) tick() {
	f.t.Helper()
	f.busy += 50
	f.idle += 50
	f.write("stat", fmt.Sprintf("cpu  %d 0 0 %d 0 0 0 0 0 0\n", f.busy, f.idle))
}

// stat writes /proc/<pid>/stat. Only utime (field 14) and starttime (field 22) matter to Kepler.
func (f *fakeProcfs) stat(pid int, comm string, cpuSeconds float64, startTicks uint64) {
	f.t.Helper()
	f.write(filepath.Join(strconv.Itoa(pid), "stat"), fmt.Sprintf(
		"%d (%s) S 1 %d %d 0 -1 4194304 0 0 0 0 %d 0 0 0 20 0 1 0 %d 0 0 18446744073709551615%s\n",
		pid, comm, pid, pid, uint64(cpuSeconds*100), startTicks, strings.Repeat(" 0", 24)))
}

// run sets a process the informer can read completely
func (f *fakeProcfs) run(pid int, comm string, cpuSeconds float64, startTicks uint64) {
	f.t.Helper()
	dir := strconv.Itoa(pid)
	f.stat(pid, comm, cpuSeconds, startTicks)
	f.write(filepath.Join(dir, "comm"), comm+"\n")
	f.write(filepath.Join(dir, "cgroup"), "0::/system.slice/"+comm+".service\n")
	f.write(filepath.Join(dir, "cmdline"), comm+"\x00")
}

// vanishing sets a process that exits between the informer reading its stat and its comm
func (f *fakeProcfs) vanishing(pid int, comm string, startTicks uint64) {
	f.t.Helper()
	f.remove(pid)
	f.stat(pid, comm, 0, startTicks)
}

func (f *fakeProcfs) remove(pid int) {
	f.t.Helper()
	require.NoError(f.t, os.RemoveAll(filepath.Join(f.root, strconv.Itoa(pid))))
}

// TestPIDReuseWhenTheIntervalThatSawItIsLost runs the real informer on a synthetic procfs directory and
// checks that a process which takes over a PID never inherits the energy of the one that held it, even
// when the interval that first saw the takeover produced no snapshot. The informer reports the takeover
// only in the refresh that detects it, so the monitor has to tell from its own previous snapshot.
func TestPIDReuseWhenTheIntervalThatSawItIsLost(t *testing.T) {
	const (
		reusedPID  = 4001
		reusedKey  = "4001"
		peerPID    = 4002
		failingPID = 4003

		oldStart, newStart = 1000, 2000
	)

	// Interval 1 gives the old process 2 s of the 3 s of CPU time and so 2/3 of the 50 J of active energy.
	const oldJoules = 33.33

	type attempt struct {
		setup func(f *fakeProcfs, zone *device.MockRaplZone)
		// wantErr is the error the interval must fail with, empty when it must produce a snapshot
		wantErr string
		// energy of the process on the reused PID, which only has its own last interval, and of the
		// unrelated process, which keeps accumulating, once the interval that produces a snapshot is done
		newJoules, peerJoules float64
	}

	takeover := func(f *fakeProcfs) {
		f.run(reusedPID, "new", 1, newStart) // a new process: all of its 1 s is new
		f.run(peerPID, "peer", 4, 500)       // 2 s of CPU time since the last interval
	}
	settled := func(f *fakeProcfs, zone *device.MockRaplZone) {
		f.run(reusedPID, "new", 3, newStart) // 2 s since the interval that saw it
		f.run(peerPID, "peer", 5, 500)       // 1 s
		f.remove(failingPID)
		zone.OnEnergy(300*Joule, nil)
	}

	tests := []struct {
		name     string
		attempts []attempt
	}{
		{
			name: "the takeover is seen by the interval that becomes the snapshot",
			attempts: []attempt{
				{
					setup: func(f *fakeProcfs, zone *device.MockRaplZone) {
						takeover(f)
						zone.OnEnergy(300*Joule, nil)
					},
					newJoules: 16.67, peerJoules: 50,
				},
			},
		},
		{
			name: "another process exits while the takeover is read",
			attempts: []attempt{
				{
					setup: func(f *fakeProcfs, _ *device.MockRaplZone) {
						takeover(f)
						f.vanishing(failingPID, "gone", 3000)
					},
					wantErr: "failed to get process comm",
				},
				{setup: settled, newJoules: 33.33, peerJoules: 33.33},
			},
		},
		{
			name: "the new process exits while the takeover is read",
			attempts: []attempt{
				{
					setup: func(f *fakeProcfs, _ *device.MockRaplZone) {
						f.vanishing(reusedPID, "new", newStart)
						f.run(peerPID, "peer", 4, 500)
					},
					wantErr: "failed to get process comm",
				},
				{
					setup: func(f *fakeProcfs, zone *device.MockRaplZone) {
						f.run(reusedPID, "new", 2, newStart) // the informer dropped the old process, so all 2 s are new
						f.run(peerPID, "peer", 5, 500)
						zone.OnEnergy(300*Joule, nil)
					},
					newJoules: 33.33, peerJoules: 33.33,
				},
			},
		},
		{
			name: "node power fails after the takeover was seen",
			attempts: []attempt{
				{
					setup: func(f *fakeProcfs, zone *device.MockRaplZone) {
						takeover(f)
						zone.OnEnergy(0, errors.New("energy counter unreadable"))
					},
					wantErr: "failed to calculate node power",
				},
				{setup: settled, newJoules: 33.33, peerJoules: 33.33},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeProcfs(t)
			f.tick()
			f.run(reusedPID, "old", 2, oldStart)
			f.run(peerPID, "peer", 1, 500)

			zone := device.NewMockRaplZone("package-0", 0, "/sys/class/powercap/intel-rapl/intel-rapl:0", 1000*Joule)
			zone.OnEnergy(100*Joule, nil)
			meter := &MockCPUPowerMeter{}
			meter.On("Zones").Return([]EnergyZone{zone}, nil)
			meter.On("PrimaryEnergyZone").Return(zone, nil)

			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
			clock := testingclock.NewFakeClock(time.Now())
			informer, err := resource.NewInformer(
				resource.WithProcFSPath(f.root), resource.WithLogger(logger), resource.WithClock(clock))
			require.NoError(t, err)
			require.NoError(t, informer.Init())

			pm := NewPowerMonitor(meter, WithResourceInformer(informer), WithClock(clock), WithLogger(logger))
			require.NoError(t, pm.Init())

			// interval 0 is the first reading, interval 1 gives the old process its energy
			require.NoError(t, pm.refreshSnapshot())

			f.tick()
			f.run(reusedPID, "old", 4, oldStart)
			f.run(peerPID, "peer", 2, 500)
			zone.OnEnergy(200*Joule, nil)
			clock.Step(time.Second)
			require.NoError(t, pm.refreshSnapshot())

			before := pm.snapshot.Load()
			require.Contains(t, before.Processes, reusedKey)
			require.Equal(t, "old", before.Processes[reusedKey].Comm)
			require.InDelta(t, oldJoules, before.Processes[reusedKey].Zones[zone].EnergyTotal.Joules(), 0.01)
			t.Logf("snapshot 1: pid %d comm=old %.2f J", reusedPID, before.Processes[reusedKey].Zones[zone].EnergyTotal.Joules())

			for _, a := range tc.attempts {
				f.tick()
				a.setup(f, zone)
				clock.Step(time.Second)
				err := pm.refreshSnapshot()

				if a.wantErr != "" {
					require.ErrorContains(t, err, a.wantErr)
					require.Same(t, before, pm.snapshot.Load(), "an interval that fails stores no snapshot")
					t.Logf("interval lost: %v", err)
					continue
				}
				require.NoError(t, err)
			}

			after := pm.snapshot.Load()
			require.Contains(t, after.Processes, reusedKey)
			current := after.Processes[reusedKey]
			t.Logf("snapshot 2: pid %d comm=%s %.2f J", reusedPID, current.Comm, current.Zones[zone].EnergyTotal.Joules())

			last := tc.attempts[len(tc.attempts)-1]
			assert.Equal(t, "new", current.Comm)
			assert.InDelta(t, last.newJoules, current.Zones[zone].EnergyTotal.Joules(), 0.01,
				"the new process on PID %d must not inherit the energy of the one that held it", reusedPID)

			peer := after.Processes[strconv.Itoa(peerPID)]
			require.NotNil(t, peer)
			t.Logf("snapshot 2: pid %d comm=peer %.2f J", peerPID, peer.Zones[zone].EnergyTotal.Joules())
			assert.InDelta(t, last.peerJoules, peer.Zones[zone].EnergyTotal.Joules(), 0.01,
				"a process that keeps its PID keeps accumulating energy")

			ended := pm.terminatedProcessesTracker.Items()
			if assert.Contains(t, ended, reusedKey, "the old process on PID %d must be reported as terminated", reusedPID) {
				t.Logf("terminated: pid %d comm=%s %.2f J", reusedPID, ended[reusedKey].Comm, ended[reusedKey].Zones[zone].EnergyTotal.Joules())
				assert.Equal(t, "old", ended[reusedKey].Comm)
				assert.InDelta(t, oldJoules, ended[reusedKey].Zones[zone].EnergyTotal.Joules(), 0.01)
			} else {
				t.Logf("terminated: none")
			}
			assert.Len(t, ended, 1)
			assert.Equal(t, ended, after.TerminatedProcesses)
		})
	}
}
