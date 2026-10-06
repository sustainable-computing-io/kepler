// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sustainable-computing-io/kepler/internal/resource"
	testingclock "k8s.io/utils/clock/testing"
)

func TestProcessPowerPIDReuse(t *testing.T) {
	statData, err := os.ReadFile("../resource/testdata/procfs/3456208/stat")
	require.NoError(t, err)

	for _, tt := range []struct {
		name          string
		startTime     uint64
		cpuTicks      uint64
		sameContainer bool
	}{
		{name: "same process", startTime: 100, cpuTicks: 3000},
		{name: "reused PID with changed metadata", startTime: 900, cpuTicks: 100},
		{name: "reused PID with unchanged metadata", startTime: 900, cpuTicks: 100, sameContainer: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			procDir := filepath.Join(root, "4001")
			require.NoError(t, os.Mkdir(procDir, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(procDir, "comm"), []byte("python3\n"), 0o644))
			require.NoError(t, os.Symlink("/usr/bin/python3", filepath.Join(procDir, "exe")))
			require.NoError(t, os.WriteFile(filepath.Join(procDir, "cmdline"), []byte("/usr/bin/python3\x00"), 0o644))
			require.NoError(t, os.WriteFile(filepath.Join(procDir, "environ"), []byte("CONTAINER_NAME=worker\x00"), 0o644))

			fields := strings.Fields(string(statData))
			fields[0] = "4001"
			fields[14] = "0"
			writeSample := func(cpuTicks, startTime, nodeTicks uint64, containerID string) {
				t.Helper()
				fields[13] = strconv.FormatUint(cpuTicks, 10)
				fields[21] = strconv.FormatUint(startTime, 10)
				require.NoError(t, os.WriteFile(filepath.Join(procDir, "stat"), []byte(strings.Join(fields, " ")+"\n"), 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(procDir, "cgroup"), []byte("0::/docker/"+containerID+"\n"), 0o644))
				nodeStat := fmt.Sprintf("cpu %d 0 0 %d 0 0 0 0 0 0\nbtime 0\n", nodeTicks, nodeTicks)
				require.NoError(t, os.WriteFile(filepath.Join(root, "stat"), []byte(nodeStat), 0o644))
			}

			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			informer, err := resource.NewInformer(resource.WithProcFSPath(root), resource.WithLogger(logger))
			require.NoError(t, err)
			zone := &MockEnergyZone{}
			zone.On("Name").Return("package").Maybe()
			zone.On("MaxEnergy").Return(1000 * Joule)
			zone.On("Power").Return(Power(0), nil)
			for _, energy := range []Energy{100 * Joule, 200 * Joule, 300 * Joule, 400 * Joule} {
				zone.On("Energy").Return(energy, nil).Once()
			}
			meter := &MockCPUPowerMeter{}
			meter.On("Zones").Return([]EnergyZone{zone}, nil)
			meter.On("PrimaryEnergyZone").Return(zone, nil)
			fakeClock := testingclock.NewFakeClock(time.Now())
			monitor := NewPowerMonitor(meter,
				WithResourceInformer(informer), WithLogger(logger), WithClock(fakeClock),
				WithMaxTerminated(10), WithMinTerminatedEnergyThreshold(0))
			t.Cleanup(func() { require.NoError(t, monitor.Shutdown()) })
			require.NoError(t, monitor.Init())

			oldID, newID := strings.Repeat("a", 64), strings.Repeat("b", 64)
			writeSample(1000, 100, 100, oldID)
			require.NoError(t, monitor.refreshSnapshot())
			fakeClock.Step(time.Second)
			writeSample(2000, 100, 200, oldID)
			require.NoError(t, monitor.refreshSnapshot())
			previous := monitor.snapshot.Load()
			oldProcess := previous.Processes["4001"]
			require.Equal(t, 50*Joule, oldProcess.Zones[zone].EnergyTotal)

			if tt.startTime == 100 || tt.sameContainer {
				newID = oldID
			}
			fakeClock.Step(time.Second)
			writeSample(tt.cpuTicks, tt.startTime, 300, newID)
			require.NoError(t, monitor.refreshSnapshot())
			current := monitor.snapshot.Load()
			require.Contains(t, current.Processes, "4001")
			process := current.Processes["4001"]
			assert.Equal(t, newID, process.ContainerID)
			assert.Equal(t, float64(tt.cpuTicks)/100, process.CPUTotalTime)
			expectedEnergy := 50 * Joule
			if tt.startTime == 100 {
				expectedEnergy += oldProcess.Zones[zone].EnergyTotal
				assert.Empty(t, current.TerminatedProcesses)
			} else {
				require.Contains(t, current.TerminatedProcesses, "4001")
				assert.Equal(t, oldProcess, current.TerminatedProcesses["4001"])
				assert.NotSame(t, oldProcess, current.TerminatedProcesses["4001"])
			}
			assert.Equal(t, expectedEnergy, process.Zones[zone].EnergyTotal)
			assert.Equal(t, 50*Joule, oldProcess.Zones[zone].EnergyTotal)
			assert.Equal(t, oldID, oldProcess.ContainerID)

			// The next sample belongs to the replacement and must accumulate its energy.
			fakeClock.Step(time.Second)
			writeSample(tt.cpuTicks+200, tt.startTime, 400, newID)
			require.NoError(t, monitor.refreshSnapshot())
			next := monitor.snapshot.Load()
			assert.Equal(t, expectedEnergy+50*Joule, next.Processes["4001"].Zones[zone].EnergyTotal)
			if tt.startTime != 100 {
				assert.Equal(t, oldProcess, next.TerminatedProcesses["4001"])
			}
			zone.AssertExpectations(t)
			meter.AssertExpectations(t)
		})
	}
}
