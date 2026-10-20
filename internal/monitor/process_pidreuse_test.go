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
	"github.com/sustainable-computing-io/kepler/internal/device/gpu"
	"github.com/sustainable-computing-io/kepler/internal/resource"
	testingclock "k8s.io/utils/clock/testing"
)

func TestProcessPowerPIDReuse(t *testing.T) {
	statData, err := os.ReadFile("../resource/testdata/procfs/3456208/stat")
	require.NoError(t, err)

	for _, tt := range []struct {
		name           string
		startTime      uint64
		cpuTicks       uint64
		sameContainer  bool
		exitNext       bool
		failCollection bool
	}{
		{name: "same process", startTime: 100, cpuTicks: 3000},
		{name: "reused PID with changed metadata", startTime: 900, cpuTicks: 100},
		{name: "reused PID with unchanged metadata", startTime: 900, cpuTicks: 100, sameContainer: true},
		{name: "replacement exits with changed metadata", startTime: 900, cpuTicks: 100, exitNext: true},
		{name: "replacement exits with unchanged metadata", startTime: 900, cpuTicks: 100, sameContainer: true, exitNext: true},
		{name: "same process after failed collection", startTime: 100, cpuTicks: 3000, failCollection: true},
		{name: "reused PID after failed collection", startTime: 900, cpuTicks: 100, failCollection: true},
		{name: "replacement exits after failed collection", startTime: 900, cpuTicks: 100, sameContainer: true, exitNext: true, failCollection: true},
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
			if tt.failCollection {
				meter.On("Zones").Return([]EnergyZone{zone}, nil).Times(3)
				meter.On("Zones").Return([]EnergyZone(nil), assert.AnError).Once()
			}
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
			cpuTicks, nodeTicks := tt.cpuTicks, uint64(300)
			writeSample(cpuTicks, tt.startTime, nodeTicks, newID)
			if tt.failCollection {
				require.ErrorIs(t, monitor.refreshSnapshot(), assert.AnError)
				require.Same(t, previous, monitor.snapshot.Load())
				fakeClock.Step(time.Second)
				cpuTicks += 100
				nodeTicks += 100
				writeSample(cpuTicks, tt.startTime, nodeTicks, newID)
			}
			require.NoError(t, monitor.refreshSnapshot())
			if tt.failCollection {
				assert.Empty(t, informer.Processes().Terminated, "the reuse notification was consumed by the failed collection")
			}
			current := monitor.snapshot.Load()
			require.Contains(t, current.Processes, "4001")
			process := current.Processes["4001"]
			assert.Equal(t, newID, process.ContainerID)
			assert.Equal(t, float64(cpuTicks)/100, process.CPUTotalTime)
			expectedEnergy := 50 * Joule
			if tt.startTime == 100 {
				expectedEnergy += oldProcess.Zones[zone].EnergyTotal
				assert.Empty(t, current.TerminatedProcesses)
			} else {
				require.Contains(t, current.TerminatedProcesses, "4001:100")
				assert.Equal(t, oldProcess, current.TerminatedProcesses["4001:100"])
				assert.NotSame(t, oldProcess, current.TerminatedProcesses["4001:100"])
			}
			assert.Equal(t, expectedEnergy, process.Zones[zone].EnergyTotal)
			assert.Equal(t, 50*Joule, oldProcess.Zones[zone].EnergyTotal)
			assert.Equal(t, oldID, oldProcess.ContainerID)

			// Refresh without exporting: the replacement either keeps running or exits.
			fakeClock.Step(time.Second)
			writeSample(cpuTicks+200, tt.startTime, nodeTicks+100, newID)
			if tt.exitNext {
				require.NoError(t, os.RemoveAll(procDir))
			}
			require.NoError(t, monitor.refreshSnapshot())
			next := monitor.snapshot.Load()
			if tt.exitNext {
				assert.Empty(t, next.Processes)
				require.Len(t, next.TerminatedProcesses, 2)
				assert.Equal(t, process, next.TerminatedProcesses["4001:900"])
				assert.NotSame(t, process, next.TerminatedProcesses["4001:900"])
			} else {
				assert.Equal(t, expectedEnergy+50*Joule, next.Processes["4001"].Zones[zone].EnergyTotal)
			}
			if tt.startTime != 100 {
				assert.Equal(t, oldProcess, next.TerminatedProcesses["4001:100"])
			}
			zone.AssertExpectations(t)
			meter.AssertExpectations(t)
		})
	}
}

func TestProcessPowerPIDReuseGPU(t *testing.T) {
	for _, tt := range []struct {
		name         string
		startTime    uint64
		containerID  string
		watts        float64
		notification bool
		podNotFound  bool
		exitNext     bool
		exportNext   bool
	}{
		{name: "same process", startTime: 100, containerID: "old"},
		{name: "same process with GPU", startTime: 100, containerID: "old", watts: 10},
		{name: "pod discovered after container", startTime: 100, containerID: "old", watts: 10, podNotFound: true},
		{name: "same process changes container", startTime: 100, containerID: "new", watts: 10},
		{name: "reused PID with new GPU energy", startTime: 900, containerID: "new", watts: 10, notification: true},
		{name: "reused PID with new GPU energy in the same container", startTime: 900, containerID: "old", watts: 10, notification: true},
		{name: "reused PID in the same container after missed termination", startTime: 900, containerID: "old", watts: 10},
		{name: "replacement exits with unchanged metadata", startTime: 900, containerID: "old", watts: 10, notification: true, exitNext: true},
		{name: "export clears retained lifetimes", startTime: 900, containerID: "old", watts: 10, notification: true, exportNext: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			zone := CreateTestZones()[0]
			node := &resource.Node{ProcessTotalCPUTimeDelta: 1}
			old := &Process{
				PID: 4001, StartTime: 100, Comm: "worker", Exe: "/usr/bin/worker",
				ContainerID: "old", GPUEnergyTotal: 100 * Joule,
				Zones: ZoneUsageMap{zone: {EnergyTotal: 50 * Joule}},
			}
			previous := NewSnapshot()
			previous.Node.Timestamp = time.Unix(1_000, 0)
			previous.Processes["4001"] = old
			previous.Containers["old"] = &Container{
				ID: "old", PodID: "pod", GPUEnergyTotal: old.GPUEnergyTotal, Zones: ZoneUsageMap{},
			}
			previous.Pods["pod"] = &Pod{ID: "pod", GPUEnergyTotal: old.GPUEnergyTotal, Zones: ZoneUsageMap{}}
			if tt.podNotFound {
				previous.Containers["old"].PodID = ""
				delete(previous.Pods, "pod")
			}
			before := previous.Clone()
			pod := &resource.Pod{ID: "pod"}
			container := &resource.Container{ID: tt.containerID, Pod: pod}

			next := NewSnapshot()
			next.Node.Timestamp = previous.Node.Timestamp.Add(5 * time.Second)
			running := &resource.Process{
				PID: 4001, StartTime: tt.startTime, Comm: old.Comm, Exe: old.Exe,
				Container: container,
			}
			procs := &resource.Processes{
				Running:    map[int]*resource.Process{4001: running},
				Terminated: map[int]*resource.Process{},
			}
			if tt.notification {
				procs.Terminated[4001] = &resource.Process{PID: 4001, StartTime: 100}
			}
			containers := &resource.Containers{
				Running:    map[string]*resource.Container{tt.containerID: container},
				Terminated: map[string]*resource.Container{},
			}
			if tt.containerID != old.ContainerID {
				containers.Terminated[old.ContainerID] = &resource.Container{ID: old.ContainerID, Pod: pod}
			}
			pods := &resource.Pods{
				Running: map[string]*resource.Pod{pod.ID: pod}, Terminated: map[string]*resource.Pod{},
			}

			informer := &MockResourceInformer{}
			informer.On("Processes").Return(procs).Once()
			informer.On("Node").Return(node).Times(3)
			informer.On("Containers").Return(containers).Once()
			informer.On("Pods").Return(pods).Twice()
			meter := &MockGPUPowerMeter{}
			meter.On("GetProcessPower").Return(map[uint32]float64{4001: tt.watts}, nil).Once()
			meter.On("GetProcessPower").Return(map[uint32]float64{}, nil).Once()
			meter.On("Devices").Return([]gpu.GPUDevice{})

			monitor := &PowerMonitor{
				logger:                      logger,
				resources:                   informer,
				gpuMeters:                   []gpu.GPUPowerMeter{meter},
				terminatedProcessesTracker:  NewTerminatedResourceTracker[*Process](zone, 10, 0, logger),
				terminatedContainersTracker: NewTerminatedResourceTracker[*Container](zone, 10, 0, logger),
				terminatedPodsTracker:       NewTerminatedResourceTracker[*Pod](zone, 10, 0, logger),
			}

			require.NoError(t, monitor.calculateProcessPower(previous, next))
			require.NoError(t, monitor.calculateContainerPower(previous, next))
			require.NoError(t, monitor.calculatePodPower(previous, next))

			currentInterval := Energy(tt.watts * 5 * float64(Joule))
			wantRunning := currentInterval
			wantTerminated := 100 * Joule
			if tt.startTime == old.StartTime {
				wantRunning += old.GPUEnergyTotal
				wantTerminated = 0
			}

			require.Contains(t, next.Processes, "4001")
			assert.Equal(t, wantRunning, next.Processes["4001"].GPUEnergyTotal)
			var terminated Energy
			for _, proc := range next.TerminatedProcessesForMetrics() {
				terminated += proc.GPUEnergyTotal
			}
			assert.Equal(t, wantTerminated, terminated)
			wantContainer := currentInterval
			if tt.containerID == old.ContainerID {
				wantContainer += old.GPUEnergyTotal
			}
			wantPod := old.GPUEnergyTotal + currentInterval
			assert.Equal(t, wantContainer, next.Containers[tt.containerID].GPUEnergyTotal)
			assert.Equal(t, wantPod, next.Pods[pod.ID].GPUEnergyTotal)
			assert.Equal(t, tt.watts, next.Containers[tt.containerID].GPUPower)
			assert.Equal(t, tt.watts, next.Pods[pod.ID].GPUPower)
			assert.Equal(t, before, previous, "the previous snapshot is immutable")

			following := &resource.Processes{
				Running: procs.Running, Terminated: map[int]*resource.Process{},
			}
			if tt.exitNext {
				following.Running = map[int]*resource.Process{}
				following.Terminated[4001] = &resource.Process{PID: 4001, StartTime: tt.startTime}
			}
			informer.On("Processes").Return(following).Once()
			informer.On("Node").Return(node).Times(3)
			informer.On("Containers").Return(&resource.Containers{
				Running: containers.Running, Terminated: map[string]*resource.Container{},
			}).Once()
			monitor.exported.Store(tt.exportNext)
			after := NewSnapshot()
			after.Node.Timestamp = next.Node.Timestamp.Add(5 * time.Second)
			nextBefore := next.Clone()
			require.NoError(t, monitor.calculateProcessPower(next, after))
			require.NoError(t, monitor.calculateContainerPower(next, after))
			require.NoError(t, monitor.calculatePodPower(next, after))
			assert.Equal(t, wantContainer, after.Containers[tt.containerID].GPUEnergyTotal)
			assert.Equal(t, wantPod, after.Pods[pod.ID].GPUEnergyTotal)
			assert.Zero(t, after.Containers[tt.containerID].GPUPower)
			assert.Zero(t, after.Pods[pod.ID].GPUPower)
			assert.Equal(t, nextBefore, next, "aggregation must preserve the previous snapshot")

			if tt.exitNext {
				assert.Empty(t, after.Processes)
				grouped := after.TerminatedProcessesForMetrics()
				require.Len(t, grouped, 1)
				for _, proc := range grouped {
					assert.Equal(t, old.GPUEnergyTotal+currentInterval, proc.GPUEnergyTotal)
					assert.Equal(t, tt.watts, proc.GPUPower)
				}
			} else {
				assert.Equal(t, wantRunning, after.Processes["4001"].GPUEnergyTotal)
			}
			if tt.exportNext {
				assert.Empty(t, after.TerminatedProcesses)
			}

			meter.AssertExpectations(t)
			informer.AssertExpectations(t)
		})
	}
}
