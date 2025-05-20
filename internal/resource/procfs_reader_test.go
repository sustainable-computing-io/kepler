// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	testclock "k8s.io/utils/clock/testing"
)

func TestNewProcess(t *testing.T) {
	t.Run("Successfully create process", func(t *testing.T) {
		mockProc := new(MockProcInfo)
		mockProc.On("PID").Return(12345)
		mockProc.On("Comm").Return("test-process", nil)
		mockProc.On("Executable").Return("/usr/bin/test", nil)
		mockProc.On("Cgroups").Return([]cGroup{{Path: "/system.slice/test.service"}}, nil)
		mockProc.On("Environ").Return([]string{}, nil).Maybe()
		mockProc.On("CmdLine").Return([]string{"/bin/bash"}, nil).Maybe()
		mockProc.On("CPUTime").Return(float64(10.5), nil).Once()

		process, err := newProcess(mockProc)
		require.NoError(t, err)
		assert.NotNil(t, process)
		assert.Equal(t, 12345, process.PID)
		assert.Equal(t, "test-process", process.Comm)
		assert.Equal(t, "/usr/bin/test", process.Exe)
		assert.Equal(t, float64(10.5), process.CPUTotalTime)
		assert.Equal(t, float64(10.5), process.CPUTimeDelta)
		assert.Nil(t, process.Container) // Not a container process

		mockProc.AssertExpectations(t)
	})

	t.Run("Error getting Comm", func(t *testing.T) {
		mockProc := &MockProcInfo{}
		mockProc.On("PID").Return(12345)
		mockProc.On("Environ").Return([]string{}, nil).Maybe()
		mockProc.On("CmdLine").Return([]string{"/bin/bash"}, nil).Maybe()
		mockProc.On("Comm").Return("", assert.AnError)
		mockProc.On("CPUTime").Return(float64(10.5), nil).Once()

		process, err := newProcess(mockProc)
		assert.Error(t, err)
		assert.Nil(t, process)
		assert.ErrorContains(t, err, "failed to get process comm")

		mockProc.AssertExpectations(t)
	})

	t.Run("Error getting Executable", func(t *testing.T) {
		mockProc := &MockProcInfo{}
		mockProc.On("PID").Return(12345)
		mockProc.On("Comm").Return("test-process", nil)
		mockProc.On("Executable").Return("", errors.New("executable error"))
		mockProc.On("CPUTime").Return(float64(10.5), nil).Once()

		process, err := newProcess(mockProc)
		assert.Error(t, err)
		assert.Nil(t, process)
		assert.ErrorContains(t, err, "failed to get process executable")

		mockProc.AssertExpectations(t)
	})

	t.Run("Error getting Cgroups", func(t *testing.T) {
		mockProc := &MockProcInfo{}
		mockProc.On("PID").Return(12345)
		mockProc.On("Comm").Return("test-process", nil)
		mockProc.On("Executable").Return("/usr/bin/test", nil)
		mockProc.On("Cgroups").Return([]cGroup{}, errors.New("cgroups error"))
		mockProc.On("CPUTime").Return(float64(10.5), nil).Once()

		process, err := newProcess(mockProc)
		assert.Error(t, err)
		assert.Nil(t, process)
		assert.ErrorContains(t, err, "failed to get process cgroups")

		mockProc.AssertExpectations(t)
	})

	t.Run("Create container process", func(t *testing.T) {
		mockProc := &MockProcInfo{}
		mockProc.On("PID").Return(12345)
		mockProc.On("Comm").Return("container-process", nil)
		mockProc.On("Executable").Return("/usr/bin/container", nil)
		mockProc.On("CPUTime").Return(float64(10.5), nil)

		ctrID := "316de3e24617ffce955b712c990dd057e7088fc9720e578cb18d874aac72deb0"
		mockProc.On("Cgroups").Return([]cGroup{{Path: fmt.Sprintf("/sys/fs/cgroup/system.slice/docker-%s.scope", ctrID)}}, nil)
		mockProc.On("Environ").Return([]string{"CONTAINER_NAME=test-container"}, nil)

		process, err := newProcess(mockProc)
		require.NoError(t, err)
		require.NotNil(t, process)
		assert.Equal(t, 12345, process.PID)
		assert.Equal(t, "container-process", process.Comm)
		assert.Equal(t, "/usr/bin/container", process.Exe)

		require.NotNil(t, process.Container)
		assert.Equal(t, ctrID, process.Container.ID)
		assert.Equal(t, DockerRuntime, process.Container.Runtime)
		assert.Equal(t, "test-container", process.Container.Name)

		mockProc.AssertExpectations(t)
	})
}

func TestResourceInformer(t *testing.T) {
	t.Run("Basic functionality", func(t *testing.T) {
		mockProc := &MockProcInfo{}
		mockProc.On("PID").Return(12345)
		mockProc.On("Comm").Return("test-process", nil)
		mockProc.On("Executable").Return("/usr/bin/test", nil)
		mockProc.On("Cgroups").Return([]cGroup{{Path: "/system.slice/test.service"}}, nil)
		mockProc.On("Environ").Return([]string{}, nil).Maybe()
		mockProc.On("CmdLine").Return([]string{"/bin/bash"}, nil).Maybe()
		mockProc.On("CPUTime").Return(float64(10.5), nil).Once()

		// AllProcs calls
		mockProcFS := &MockProcReader{}
		fakeClock := testclock.NewFakeClock(time.Now())

		informer, err := NewInformer(
			WithProcReader(mockProcFS),
			WithClock(fakeClock),
		)
		require.NoError(t, err)
		require.NotNil(t, informer)

		// Initialize
		mockProcFS.On("AllProcs").Return([]procInfo{mockProc}, nil).Once() // first
		err = informer.Init()
		require.NoError(t, err)

		// First refresh
		mockProcFS.On("AllProcs").Return([]procInfo{mockProc}, nil).Once() // first
		err = informer.Refresh()
		require.NoError(t, err)

		// Check processes
		processes := informer.Processes()
		require.NotNil(t, processes)
		assert.Len(t, processes.Running, 1)
		assert.Len(t, processes.Terminated, 0)
		assert.Equal(t, 12345, processes.Running[12345].PID)
		assert.Equal(t, "test-process", processes.Running[12345].Comm)
		assert.Equal(t, float64(10.5), processes.Running[12345].CPUTotalTime)
		assert.Equal(t, float64(10.5), processes.Running[12345].CPUTimeDelta) // First time, delta equals total

		// Check containers (none in this test)
		containers := informer.Containers()
		require.NotNil(t, containers)
		assert.Len(t, containers.Running, 0)
		assert.Len(t, containers.Terminated, 0)

		// For second Refresh - same process with increased CPU time
		mockProc.On("CPUTime").Return(float64(15.0), nil).Once()
		mockProcFS.On("AllProcs").Return([]procInfo{mockProc}, nil).Once()

		err = informer.Refresh()
		require.NoError(t, err)

		// Check update after second refresh
		processes = informer.Processes()
		assert.Equal(t, float64(15.0), processes.Running[12345].CPUTotalTime)
		assert.Equal(t, float64(4.5), processes.Running[12345].CPUTimeDelta) // 15.0 - 10.5 = 4.5
		assert.Equal(t, float64(4.5), processes.NodeCPUTimeDelta)            // Total delta

		mockProcFS.AssertExpectations(t)
		mockProc.AssertExpectations(t)
	})

	t.Run("Process termination", func(t *testing.T) {
		mockInformer := &MockProcReader{}
		fakeClock := testclock.NewFakeClock(time.Now())

		// Create two processes for first refresh
		mockProc1 := &MockProcInfo{}
		mockProc1.On("PID").Return(1001)
		mockProc1.On("Comm").Return("process-1", nil)
		mockProc1.On("Executable").Return("/bin/process1", nil)
		mockProc1.On("Cgroups").Return([]cGroup{{Path: "/system.slice/process1.service"}}, nil)
		mockProc1.On("CPUTime").Return(float64(5.0), nil).Once()
		mockProc1.On("Environ").Return([]string{}, nil).Maybe()
		mockProc1.On("CmdLine").Return([]string{"/bin/process1"}, nil).Maybe()

		mockProc2 := new(MockProcInfo)
		mockProc2.On("PID").Return(1002)
		mockProc2.On("Comm").Return("process-2", nil)
		mockProc2.On("Executable").Return("/bin/process2", nil)
		mockProc2.On("Cgroups").Return([]cGroup{{Path: "/system.slice/process2.service"}}, nil)
		mockProc2.On("CPUTime").Return(float64(10.0), nil).Once()
		mockProc2.On("Environ").Return([]string{}, nil).Maybe()
		mockProc2.On("CmdLine").Return([]string{"/bin/process2"}, nil).Maybe()

		// For Init
		mockInformer.On("AllProcs").Return([]procInfo{mockProc1, mockProc2}, nil).Once()

		// For first Refresh
		mockInformer.On("AllProcs").Return([]procInfo{mockProc1, mockProc2}, nil).Once()

		informer, err := NewInformer(
			WithProcReader(mockInformer),
			WithClock(fakeClock),
		)
		require.NoError(t, err)

		err = informer.Init()
		require.NoError(t, err)

		err = informer.Refresh()
		require.NoError(t, err)

		// Verify both processes are running
		processes := informer.Processes()
		assert.Len(t, processes.Running, 2)
		assert.Len(t, processes.Terminated, 0)

		// Second refresh - process 2 is gone
		mockProc1.On("CPUTime").Return(float64(7.5), nil)
		mockInformer.On("AllProcs").Return([]procInfo{mockProc1}, nil).Once()

		// Second refresh
		err = informer.Refresh()
		require.NoError(t, err)

		// Check processes after second refresh
		processes = informer.Processes()
		assert.Len(t, processes.Running, 1)
		assert.Len(t, processes.Terminated, 1)
		assert.Contains(t, processes.Running, 1001)
		assert.Contains(t, processes.Terminated, 1002)
		assert.Equal(t, "process-2", processes.Terminated[1002].Comm)

		// Check CPU time delta
		assert.Equal(t, float64(2.5), processes.Running[1001].CPUTimeDelta) // 7.5 - 5.0 = 2.5
		assert.Equal(t, float64(2.5), processes.NodeCPUTimeDelta)           // Should match the only running process's delta

		mockInformer.AssertExpectations(t)
		mockProc1.AssertExpectations(t)
		mockProc2.AssertExpectations(t)
	})

	t.Run("Container detect", func(t *testing.T) {
		mockInformer := &MockProcReader{}
		fakeClock := testclock.NewFakeClock(time.Now())

		// Create a container process
		mockProc := &MockProcInfo{}
		mockProc.On("PID").Return(2001)
		mockProc.On("Comm").Return("container-proc", nil)
		mockProc.On("Executable").Return("/bin/container-app", nil)
		mockProc.On("Environ").Return([]string{
			"CONTAINER_NAME=test-container",
		}, nil)

		ctnrID, cgPath := mockContainerIDAndPath(PodmanRuntime)
		mockProc.On("Cgroups").Return([]cGroup{{Path: cgPath}}, nil).Once()

		mockProc.On("CPUTime").Return(float64(3.0), nil).Once()

		informer, err := NewInformer(
			WithProcReader(mockInformer),
			WithClock(fakeClock),
		)
		require.NoError(t, err)

		// Initialize
		mockInformer.On("AllProcs").Return([]procInfo{mockProc}, nil).Once()
		err = informer.Init()
		require.NoError(t, err)

		// First refresh
		mockInformer.On("AllProcs").Return([]procInfo{mockProc}, nil).Once()
		err = informer.Refresh()
		require.NoError(t, err)

		// Verify process is tracked
		processes := informer.Processes()
		assert.Len(t, processes.Running, 1)
		assert.Contains(t, processes.Running, 2001)
		require.NotNil(t, processes.Running[2001].Container, "failed to find container for %s", cgPath)
		assert.Equal(t, "test-container", processes.Running[2001].Container.Name)
		assert.Equal(t, ctnrID, processes.Running[2001].Container.ID, "failed to find container id from %s", cgPath)

		// Verify container is tracked
		containers := informer.Containers()
		assert.Len(t, containers.Running, 1)
		assert.Contains(t, containers.Running, ctnrID)
		c := containers.Running[ctnrID]
		assert.Equal(t, "test-container", c.Name)
		assert.Equal(t, PodmanRuntime, c.Runtime)
		assert.Equal(t, float64(3.0), c.CPUTimeDelta)
		assert.Equal(t, float64(3.0), containers.NodeCPUTimeDelta)

		// For second Refresh - increased CPU time
		mockProc.On("CPUTime").Return(float64(5.0), nil).Once()
		mockInformer.On("AllProcs").Return([]procInfo{mockProc}, nil).Once()

		// Second refresh
		err = informer.Refresh()
		require.NoError(t, err)

		// Check container after second refresh
		containers = informer.Containers()
		assert.Equal(t, float64(5.0), containers.Running[ctnrID].CPUTotalTime)
		assert.Equal(t, float64(2.0), containers.Running[ctnrID].CPUTimeDelta)
		assert.Equal(t, float64(2.0), processes.NodeCPUTimeDelta) // Delta should be 2.0

		mockInformer.AssertExpectations(t)
		mockProc.AssertExpectations(t)
	})

	t.Run("Container termination", func(t *testing.T) {
		mockInformer := new(MockProcReader)
		fakeClock := testclock.NewFakeClock(time.Now())

		// Create a container process
		mockProc := new(MockProcInfo)
		mockProc.On("PID").Return(3001)
		mockProc.On("Comm").Return("container-app", nil)
		mockProc.On("Executable").Return("/bin/container-app", nil)
		cntrID, cgroupPath := mockContainerIDAndPath(PodmanRuntime)
		mockProc.On("Cgroups").Return([]cGroup{{Path: cgroupPath}}, nil)
		mockProc.On("Environ").Return([]string{"CONTAINER_NAME=test-container"}, nil)
		mockProc.On("CPUTime").Return(float64(8.0), nil)

		// For Init
		mockInformer.On("AllProcs").Return([]procInfo{mockProc}, nil).Once()

		// For first Refresh
		mockInformer.On("AllProcs").Return([]procInfo{mockProc}, nil).Once()

		informer, err := NewInformer(
			WithProcReader(mockInformer),
			WithClock(fakeClock),
		)
		require.NoError(t, err)

		// Initialize
		err = informer.Init()
		require.NoError(t, err)

		// First refresh
		err = informer.Refresh()
		require.NoError(t, err)

		// Verify container is tracked
		containers := informer.Containers()
		assert.Len(t, containers.Running, 1)
		assert.Contains(t, containers.Running, cntrID)

		// Second refresh - container process is gone
		mockInformer.On("AllProcs").Return([]procInfo{}, nil).Once()

		// Move clock forward
		fakeClock.Step(1000 * 1000 * 1000) // 1 second

		// Second refresh
		err = informer.Refresh()
		require.NoError(t, err)

		// Check container after second refresh
		containers = informer.Containers()
		assert.Len(t, containers.Running, 0)
		assert.Len(t, containers.Terminated, 1)
		assert.Contains(t, containers.Terminated, cntrID)
		assert.Equal(t, "test-container", containers.Terminated[cntrID].Name)

		// Check processes
		processes := informer.Processes()
		assert.Len(t, processes.Running, 0)
		assert.Len(t, processes.Terminated, 1)
		assert.Contains(t, processes.Terminated, 3001)

		mockInformer.AssertExpectations(t)
		mockProc.AssertExpectations(t)
	})

	t.Run("Refresh error", func(t *testing.T) {
		mockInformer := new(MockProcReader)
		fakeClock := testclock.NewFakeClock(time.Now())

		// For Init
		mockInformer.On("AllProcs").Return([]procInfo{}, nil).Once()

		// For Refresh - return error
		mockInformer.On("AllProcs").Return([]procInfo{}, errors.New("procfs error")).Once()

		informer, err := NewInformer(
			WithProcReader(mockInformer),
			WithClock(fakeClock),
		)
		require.NoError(t, err)

		// Initialize
		err = informer.Init()
		require.NoError(t, err)

		// Refresh should return error
		err = informer.Refresh()
		assert.Error(t, err)
		assert.ErrorContains(t, err, "failed to get processes")

		mockInformer.AssertExpectations(t)
	})
}

// Test for the procfs fixture to ensure the test fixture directory is available
// and to test the integration with procfs package
func TestProcWrapper(t *testing.T) {
	fs, err := procfs.NewFS("./testdata/procfs")
	if err != nil {
		t.Skip("Skipping test due to missing procfs fixtures:", err)
	}

	// Get a process from fixtures
	pid := 3456208
	proc, err := fs.Proc(pid)
	require.NoError(t, err)

	// Wrap the process
	wrapper := WrapProc(proc)

	// Test methods
	assert.Equal(t, pid, wrapper.PID())

	comm, err := wrapper.Comm()
	require.NoError(t, err)
	assert.Equal(t, "prometheus", comm) // From procfs test fixtures

	exe, err := wrapper.Executable()
	require.NoError(t, err)
	// NOTE: The test fixtures don't have the full path and
	// prometheus is only an empty file under fake-root, softlinked
	// as exe in the proc/3456208 directory
	assert.Contains(t, exe, "/usr/bin/prometheus")

	cgroups, err := wrapper.Cgroups()
	require.NoError(t, err)
	assert.NotEmpty(t, cgroups)

	cpuTime, err := wrapper.CPUTime()
	require.NoError(t, err)
	assert.Greater(t, cpuTime, float64(0))
}

// Test for the procfs fixture to ensure the test fixture directory is available
// and to test the integration with procfs package
func TestProcFSReader(t *testing.T) {
	informer, err := NewProcFSReader("./testdata/procfs")
	require.NoError(t, err)

	// Test AllProcs
	procs, err := informer.AllProcs()
	require.NoError(t, err)
	assert.Len(t, procs, 5)
}

// Test for the procfs fixture to ensure the test fixture directory is available
// and to test the integration with procfs package
func TestProcFSReaderWithInformer(t *testing.T) {
	informer, err := NewInformer(WithProcFSPath("./testdata/procfs"))
	require.NoError(t, err)

	// Test AllProcs
	err = informer.Refresh()
	assert.NoError(t, err)

	processes := informer.Processes()
	assert.Len(t, processes.Running, 5)
	assert.Len(t, processes.Terminated, 0)

	containers := informer.Containers()
	// prometheus container and podman container
	assert.Len(t, containers.Running, 2)
	assert.Len(t, containers.Terminated, 0)

	runtimes := []ContainerRuntime{}
	for _, c := range containers.Running {
		runtimes = append(runtimes, c.Runtime)
	}

	assert.Contains(t, runtimes, PodmanRuntime)
	assert.Contains(t, runtimes, DockerRuntime)

	// go through all procs and count processes
	// by controller-runtime

	containerProcs := map[ContainerRuntime]int{}

	for _, p := range processes.Running {
		rt := UnknownRuntime
		if p.Container != nil {
			rt = p.Container.Runtime
		}
		containerProcs[rt]++
	}
	assert.Equal(t, 1, containerProcs[DockerRuntime])
	assert.Equal(t, 1, containerProcs[UnknownRuntime])
	assert.Equal(t, 3, containerProcs[PodmanRuntime])
}

func TestProcessUpdateAfterRefresh(t *testing.T) {
	mockInformer := &MockProcReader{}
	fakeClock := testclock.NewFakeClock(time.Now())

	// Initial process state
	mockProc := &MockProcInfo{}
	mockProc.On("PID").Return(1001)
	mockProc.On("Comm").Return("process-initial", nil).Once()
	mockProc.On("Executable").Return("/bin/process-initial", nil).Once()
	mockProc.On("Cgroups").Return([]cGroup{{Path: "/system.slice/process.service"}}, nil).Once()
	mockProc.On("CPUTime").Return(float64(5.0), nil).Once()
	mockProc.On("Environ").Return([]string{}, nil).Maybe()
	mockProc.On("CmdLine").Return([]string{"/bin/process-initial"}, nil).Maybe()

	// For Init
	mockInformer.On("AllProcs").Return([]procInfo{mockProc}, nil).Once()

	// For first Refresh
	mockInformer.On("AllProcs").Return([]procInfo{mockProc}, nil).Once()

	informer, err := NewInformer(
		WithProcReader(mockInformer),
		WithClock(fakeClock),
	)
	require.NoError(t, err)

	// Initialize and first refresh
	err = informer.Init()
	require.NoError(t, err)

	err = informer.Refresh()
	require.NoError(t, err)

	// Verify initial state
	processes := informer.Processes()
	assert.Equal(t, "process-initial", processes.Running[1001].Comm)
	assert.Equal(t, "/bin/process-initial", processes.Running[1001].Exe)
	assert.Equal(t, float64(5.0), processes.Running[1001].CPUTotalTime)

	// Second refresh - process has changed comm and executable, with significant CPU time
	mockProc.On("Comm").Return("process-updated", nil).Once()
	mockProc.On("Executable").Return("/bin/process-updated", nil).Once()
	mockProc.On("Cgroups").Return([]cGroup{{Path: "/system.slice/process.service"}}, nil).Once()
	mockProc.On("CPUTime").Return(float64(7.0), nil).Once() // 2.0 delta

	mockInformer.On("AllProcs").Return([]procInfo{mockProc}, nil).Once()

	// Second refresh
	err = informer.Refresh()
	require.NoError(t, err)

	// Verify changes were applied
	processes = informer.Processes()
	assert.Equal(t, "process-updated", processes.Running[1001].Comm)
	assert.Equal(t, "/bin/process-updated", processes.Running[1001].Exe)
	assert.Equal(t, float64(7.0), processes.Running[1001].CPUTotalTime)
	assert.Equal(t, float64(2.0), processes.Running[1001].CPUTimeDelta)

	// Third refresh - process changes again but with negligible CPU time delta
	mockProc.On("CPUTime").Return(float64(7.0000000000001), nil).Once() // Very small delta (1e-13)
	mockInformer.On("AllProcs").Return([]procInfo{mockProc}, nil).Once()

	// Third refresh
	err = informer.Refresh()
	require.NoError(t, err)

	// Verify process wasn't updated due to negligible CPU time
	processes = informer.Processes()
	assert.Equal(t, "process-updated", processes.Running[1001].Comm,
		"Process with negligible CPU time delta should not be updated")
	assert.Equal(t, "/bin/process-updated", processes.Running[1001].Exe,
		"Process with negligible CPU time delta should not be updated")
	assert.InDelta(t, 7.0000000000001, processes.Running[1001].CPUTotalTime, 1e-10)
	assert.InDelta(t, 1e-13, processes.Running[1001].CPUTimeDelta, 1e-10)

	mockInformer.AssertExpectations(t)
	mockProc.AssertExpectations(t)
}

func TestZeroCPUTimeProcess(t *testing.T) {
	mockInformer := &MockProcReader{}
	fakeClock := testclock.NewFakeClock(time.Now())

	// Initial creation of process (new process)
	mockProc := &MockProcInfo{}
	mockProc.On("PID").Return(1001).Times(5) // Called multiple times
	mockProc.On("Comm").Return("zero-cpu-process", nil).Once()
	mockProc.On("Executable").Return("/bin/zero-cpu-process", nil).Once()
	mockProc.On("Cgroups").Return([]cGroup{{Path: "/system.slice/process.service"}}, nil).Once()
	mockProc.On("CPUTime").Return(float64(0.0), nil).Once()
	mockProc.On("Environ").Return([]string{}, nil).Maybe()
	mockProc.On("CmdLine").Return([]string{"/bin/zero-cpu-process"}, nil).Maybe()

	// For Init
	mockInformer.On("AllProcs").Return([]procInfo{mockProc}, nil).Once()

	// For first Refresh
	mockInformer.On("AllProcs").Return([]procInfo{mockProc}, nil).Once()

	informer, err := NewInformer(
		WithProcReader(mockInformer),
		WithClock(fakeClock),
	)
	require.NoError(t, err)

	// Initialize and first refresh
	err = informer.Init()
	require.NoError(t, err)

	err = informer.Refresh()
	require.NoError(t, err)

	// Verify initial state (should be created even with zero CPU time)
	processes := informer.Processes()
	assert.Equal(t, "zero-cpu-process", processes.Running[1001].Comm)
	assert.Equal(t, "/bin/zero-cpu-process", processes.Running[1001].Exe)
	assert.Equal(t, float64(0.0), processes.Running[1001].CPUTotalTime)
	assert.Equal(t, float64(0.0), processes.Running[1001].CPUTimeDelta)

	// Second refresh - process with close to 0 CPU delta and should not update process fields
	mockProc.On("CPUTime").Return(float64(1e-14), nil).Once() // Still zero

	mockInformer.On("AllProcs").Return([]procInfo{mockProc}, nil).Once()

	// Second refresh
	err = informer.Refresh()
	require.NoError(t, err)

	// Verify process wasn't updated due to zero CPU time delta
	processes = informer.Processes()
	assert.Equal(t, "zero-cpu-process", processes.Running[1001].Comm,
		"Process with zero CPU time delta should not be updated")
	assert.Equal(t, "/bin/zero-cpu-process", processes.Running[1001].Exe,
		"Process with zero CPU time delta should not be updated")
	assert.Equal(t, float64(1e-14), processes.Running[1001].CPUTotalTime)
	assert.Equal(t, float64(1e-14), processes.Running[1001].CPUTimeDelta)

	mockInformer.AssertExpectations(t)
	mockProc.AssertExpectations(t)
}
