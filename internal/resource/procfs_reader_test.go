// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/prometheus/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sustainable-computing-io/kepler/internal/k8s/pod"
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
		mockProc.On("CmdLine").Return([]string{"/usr/bin/test", "this", "out"}, nil).Maybe()
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
		mockProc.On("CmdLine").Return([]string{"/usr/bin/container"}, nil)
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
		mockProc.On("CmdLine").Return([]string{"/bin/bash"}, nil)
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
		mockProcFS.On("CPUUsageRatio").Return(float64(0.25), nil).Once()
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

		// Check Node information
		node := informer.Node()
		require.NotNil(t, node)
		assert.Equal(t, float64(0.25), node.CPUUsageRatio)
		assert.Equal(t, float64(10.5), node.ProcessTotalCPUTimeDelta)

		// Check containers (none in this test)
		containers := informer.Containers()
		require.NotNil(t, containers)
		assert.Len(t, containers.Running, 0)
		assert.Len(t, containers.Terminated, 0)

		// For second Refresh - same process with increased CPU time
		mockProc.On("CPUTime").Return(float64(15.0), nil).Once()
		mockProcFS.On("AllProcs").Return([]procInfo{mockProc}, nil).Once()
		mockProcFS.On("CPUUsageRatio").Return(float64(0.35), nil).Once()

		err = informer.Refresh()
		require.NoError(t, err)

		// Check update after second refresh
		processes = informer.Processes()
		assert.Equal(t, float64(15.0), processes.Running[12345].CPUTotalTime)
		assert.Equal(t, float64(4.5), processes.Running[12345].CPUTimeDelta) // 15.0 - 10.5 = 4.5

		// Check updated Node information
		node = informer.Node()
		require.NotNil(t, node)
		assert.Equal(t, float64(0.35), node.CPUUsageRatio)
		assert.Equal(t, float64(4.5), node.ProcessTotalCPUTimeDelta)

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
		mockInformer.On("CPUUsageRatio").Return(float64(0.1), nil).Once()

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

		// Check Node information
		node := informer.Node()
		require.NotNil(t, node)
		assert.Equal(t, float64(0.1), node.CPUUsageRatio)
		assert.Equal(t, float64(15.0), node.ProcessTotalCPUTimeDelta) // 5.0 + 10.0 = 15.0

		// Second refresh - process 2 is gone
		mockProc1.On("CPUTime").Return(float64(7.5), nil)
		mockInformer.On("AllProcs").Return([]procInfo{mockProc1}, nil).Once()
		mockInformer.On("CPUUsageRatio").Return(float64(0.15), nil).Once()

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

		// Check updated Node information
		node = informer.Node()
		require.NotNil(t, node)
		assert.Equal(t, float64(0.15), node.CPUUsageRatio)
		assert.Equal(t, float64(2.5), node.ProcessTotalCPUTimeDelta) // Only running process delta

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
		mockProc.On("CmdLine").Return([]string{"/bin/container-app", "-with", "args"}, nil)
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
		mockInformer.On("CPUUsageRatio").Return(float64(0.3), nil).Once()
		err = informer.Refresh()
		require.NoError(t, err)

		// Check Node information
		node := informer.Node()
		require.NotNil(t, node)
		assert.Equal(t, float64(0.3), node.CPUUsageRatio)
		assert.Equal(t, float64(3.0), node.ProcessTotalCPUTimeDelta)

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

		// For second Refresh - increased CPU time
		mockProc.On("CPUTime").Return(float64(5.0), nil).Once()
		mockInformer.On("AllProcs").Return([]procInfo{mockProc}, nil).Once()
		mockInformer.On("CPUUsageRatio").Return(float64(0.45), nil).Once()

		// Second refresh
		err = informer.Refresh()
		require.NoError(t, err)

		// Check container after second refresh
		containers = informer.Containers()
		assert.Equal(t, float64(5.0), containers.Running[ctnrID].CPUTotalTime)
		assert.Equal(t, float64(2.0), containers.Running[ctnrID].CPUTimeDelta)

		// Check updated Node information
		node = informer.Node()
		require.NotNil(t, node)
		assert.Equal(t, float64(0.45), node.CPUUsageRatio)
		assert.Equal(t, float64(2.0), node.ProcessTotalCPUTimeDelta)

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
		mockProc.On("CmdLine").Return([]string{"/bin/container-app", "-with", "args"}, nil)
		cntrID, cgroupPath := mockContainerIDAndPath(PodmanRuntime)
		mockProc.On("Cgroups").Return([]cGroup{{Path: cgroupPath}}, nil)
		mockProc.On("Environ").Return([]string{"CONTAINER_NAME=test-container"}, nil)
		mockProc.On("CPUTime").Return(float64(8.0), nil)

		// For Init
		mockInformer.On("AllProcs").Return([]procInfo{mockProc}, nil).Once()

		// For first Refresh
		mockInformer.On("AllProcs").Return([]procInfo{mockProc}, nil).Once()
		mockInformer.On("CPUUsageRatio").Return(float64(0.0), nil).Once()

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
		mockInformer.On("CPUUsageRatio").Return(float64(0.3), nil).Once()

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

		// For Refresh - return error from AllProcs but still need CPUUsageRatio for refreshNode
		mockInformer.On("AllProcs").Return([]procInfo{}, errors.New("procfs error")).Once()
		mockInformer.On("CPUUsageRatio").Return(0.5, nil).Once()

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

func TestRefresh_PodInformer(t *testing.T) {
	t.Run("Uses podInformer successfully", func(t *testing.T) {
		mockProc := &MockProcInfo{}
		mockProc.On("PID").Return(123)
		mockProc.On("Comm").Return("test-process", nil)
		mockProc.On("CmdLine").Return([]string{"/usr/bin/test", "--arg1"}, nil).Once()
		mockProc.On("Executable").Return("/usr/bin/test", nil)
		containerID, cgPath := mockContainerIDAndPath(DockerRuntime)
		mockProc.On("Cgroups").Return([]cGroup{{Path: cgPath}}, nil)
		mockProc.On("CPUTime").Return(10.0, nil).Once()
		mockProc.On("Environ").Return([]string{"CONTAINER_NAME=my-container"}, nil)

		mockProcFS := &MockProcReader{}
		mockProcFS.On("AllProcs").Return([]procInfo{mockProc}, nil).Twice()
		mockProcFS.On("CPUUsageRatio").Return(0.5, nil).Once()

		mockPodInformer := new(mockPodInformer)
		mockPodInformer.On("LookupByContainerID", containerID).Return(
			&pod.ContainerInfo{
				PodID:         "pod123",
				PodName:       "mypod",
				Namespace:     "default",
				ContainerName: "my-container",
			}, true, nil,
		)

		informer, err := NewInformer(WithProcReader(mockProcFS), WithPodInformer(mockPodInformer))
		require.NoError(t, err)
		err = informer.Init()
		require.NoError(t, err)
		err = informer.Refresh()
		require.NoError(t, err)

		pods := informer.Pods()
		assert.Len(t, pods.Running, 1)
		assert.Equal(t, "mypod", pods.Running["pod123"].Name)

		mockPodInformer.AssertExpectations(t)
		mockProcFS.AssertExpectations(t)
		mockProc.AssertExpectations(t)
	})
	t.Run("podInformer returns ErrNoPod", func(t *testing.T) {
		mockProc := &MockProcInfo{}
		mockProc.On("PID").Return(456)
		mockProc.On("Comm").Return("container-process", nil)
		mockProc.On("Executable").Return("/usr/bin/container-exec", nil)
		mockProc.On("CPUTime").Return(10.0, nil).Once()
		mockProc.On("Environ").Return([]string{"CONTAINER_NAME=my-container"}, nil)
		mockProc.On("CmdLine").Return([]string{"/usr/bin/container-exec"}, nil).Once()

		containerID, cgPath := mockContainerIDAndPath(DockerRuntime)
		mockProc.On("Cgroups").Return([]cGroup{{Path: cgPath}}, nil)

		mockProcFS := &MockProcReader{}
		mockProcFS.On("AllProcs").Return([]procInfo{mockProc}, nil).Twice()
		mockProcFS.On("CPUUsageRatio").Return(0.5, nil).Once()

		mockPodInformer := new(mockPodInformer)
		mockPodInformer.On("LookupByContainerID", containerID).Return(nil, false, nil)

		informer, err := NewInformer(
			WithProcReader(mockProcFS),
			WithPodInformer(mockPodInformer),
		)
		require.NoError(t, err)

		err = informer.Init()
		require.NoError(t, err)

		err = informer.Refresh()
		require.NoError(t, err)

		pods := informer.Pods()
		assert.Empty(t, pods.Running)
		assert.Contains(t, pods.ContainersNoPod, containerID)

		mockPodInformer.AssertExpectations(t)
		mockProcFS.AssertExpectations(t)
		mockProc.AssertExpectations(t)
	})
	t.Run("podInformer returns a general error", func(t *testing.T) {
		mockProc := &MockProcInfo{}
		mockProc.On("PID").Return(789)
		mockProc.On("Comm").Return("container-process", nil)
		mockProc.On("Executable").Return("/usr/bin/container-exec", nil)
		mockProc.On("CPUTime").Return(10.0, nil).Once()
		mockProc.On("Environ").Return([]string{"CONTAINER_NAME=my-container"}, nil)
		mockProc.On("CmdLine").Return([]string{"/usr/bin/container-exec"}, nil).Once()

		containerID, cgPath := mockContainerIDAndPath(DockerRuntime)
		mockProc.On("Cgroups").Return([]cGroup{{Path: cgPath}}, nil)

		mockProcFS := &MockProcReader{}
		mockProcFS.On("AllProcs").Return([]procInfo{mockProc}, nil).Twice()
		mockProcFS.On("CPUUsageRatio").Return(0.5, nil).Once()

		podError := errors.New("general error")
		mockPodInformer := new(mockPodInformer)
		mockPodInformer.On("LookupByContainerID", containerID).Return(nil, false, podError)

		informer, err := NewInformer(
			WithProcReader(mockProcFS),
			WithPodInformer(mockPodInformer),
		)
		require.NoError(t, err)

		err = informer.Init()
		require.NoError(t, err)

		err = informer.Refresh()
		require.ErrorContains(t, err, "failed to get pod for container")

		// even if podInformer has general errors, informer should continue gracefully
		pods := informer.Pods()
		assert.Empty(t, pods.Running)
		assert.NotContains(t, pods.ContainersNoPod, containerID, "Container should not be added to ContainersNoPod on general errors")

		mockPodInformer.AssertExpectations(t)
		mockProcFS.AssertExpectations(t)
		mockProc.AssertExpectations(t)
	})
}

func TestLookupByContainerID_UpdatesContainerName(t *testing.T) {
	t.Run("Container name from podInfo updates container cache", func(t *testing.T) {
		mockProc := &MockProcInfo{}
		mockProc.On("PID").Return(5001)
		mockProc.On("Comm").Return("app-container", nil)
		mockProc.On("Executable").Return("/app/server", nil)
		mockProc.On("CPUTime").Return(15.0, nil).Once()
		mockProc.On("Environ").Return([]string{}, nil) // No CONTAINER_NAME in env
		mockProc.On("CmdLine").Return([]string{"/app/server", "--port=8080"}, nil)

		// Create container with Docker runtime
		containerID, cgPath := mockContainerIDAndPath(DockerRuntime)
		mockProc.On("Cgroups").Return([]cGroup{{Path: cgPath}}, nil)

		mockProcFS := &MockProcReader{}
		mockProcFS.On("AllProcs").Return([]procInfo{mockProc}, nil).Twice()
		mockProcFS.On("CPUUsageRatio").Return(0.4, nil).Once()

		// Mock pod informer that returns container name from pod info
		mockPodInformer := new(mockPodInformer)
		mockPodInformer.On("LookupByContainerID", containerID).Return(
			&pod.ContainerInfo{
				PodID:         "pod-12345",
				PodName:       "test-app-pod",
				Namespace:     "production",
				ContainerName: "app-container-from-pod", // Container name comes from pod status
			}, true, nil,
		)

		informer, err := NewInformer(
			WithProcReader(mockProcFS),
			WithPodInformer(mockPodInformer),
		)
		require.NoError(t, err)

		err = informer.Init()
		require.NoError(t, err)

		err = informer.Refresh()
		require.NoError(t, err)

		// Verify container name is updated from podInfo, not from environment
		containers := informer.Containers()
		require.Len(t, containers.Running, 1)
		container := containers.Running[containerID]
		require.NotNil(t, container)

		// Container name should come from pod info, not environment
		assert.Equal(t, "app-container-from-pod", container.Name,
			"Container name should be set from podInfo.LookupByContainerID")
		assert.Equal(t, containerID, container.ID)
		assert.Equal(t, DockerRuntime, container.Runtime)

		// Verify pod information is also set
		pods := informer.Pods()
		require.Len(t, pods.Running, 1)
		podInstance := pods.Running["pod-12345"]
		require.NotNil(t, podInstance)
		assert.Equal(t, "test-app-pod", podInstance.Name)
		assert.Equal(t, "production", podInstance.Namespace)

		// Verify the container has reference to the pod
		assert.NotNil(t, container.Pod)
		assert.Equal(t, "pod-12345", container.Pod.ID)
		assert.Equal(t, "test-app-pod", container.Pod.Name)
		assert.Equal(t, "production", container.Pod.Namespace)

		mockPodInformer.AssertExpectations(t)
		mockProcFS.AssertExpectations(t)
		mockProc.AssertExpectations(t)
	})

	t.Run("Container name prioritizes podInfo over environment", func(t *testing.T) {
		mockProc := &MockProcInfo{}
		mockProc.On("PID").Return(5002)
		mockProc.On("Comm").Return("web-app", nil)
		mockProc.On("Executable").Return("/usr/bin/nginx", nil)
		mockProc.On("CPUTime").Return(8.5, nil).Once()
		mockProc.On("Environ").Return([]string{"CONTAINER_NAME=nginx-from-env"}, nil)
		mockProc.On("CmdLine").Return([]string{"/usr/bin/nginx", "-g", "daemon off;"}, nil)

		containerID, cgPath := mockContainerIDAndPath(ContainerDRuntime)
		mockProc.On("Cgroups").Return([]cGroup{{Path: cgPath}}, nil)

		mockProcFS := &MockProcReader{}
		mockProcFS.On("AllProcs").Return([]procInfo{mockProc}, nil).Twice()
		mockProcFS.On("CPUUsageRatio").Return(0.2, nil).Once()

		// Pod informer returns different name than environment
		mockPodInformer := new(mockPodInformer)
		mockPodInformer.On("LookupByContainerID", containerID).Return(
			&pod.ContainerInfo{
				PodID:         "web-pod-67890",
				PodName:       "web-server",
				Namespace:     "default",
				ContainerName: "nginx-from-pod", // Different from environment name
			}, true, nil,
		)

		informer, err := NewInformer(
			WithProcReader(mockProcFS),
			WithPodInformer(mockPodInformer),
		)
		require.NoError(t, err)

		err = informer.Init()
		require.NoError(t, err)

		err = informer.Refresh()
		require.NoError(t, err)

		// Verify container name comes from podInfo, not environment
		containers := informer.Containers()
		require.Len(t, containers.Running, 1)
		container := containers.Running[containerID]
		require.NotNil(t, container)

		// Should use pod name, not environment name
		assert.Equal(t, "nginx-from-pod", container.Name,
			"Container name should prioritize podInfo over environment variables")
		assert.NotEqual(t, "nginx-from-env", container.Name,
			"Should not use environment container name when podInfo is available")

		mockPodInformer.AssertExpectations(t)
		mockProcFS.AssertExpectations(t)
		mockProc.AssertExpectations(t)
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
	assert.Len(t, procs, 6) // 1 regular, 4 containers, 1 vm
}

// Test for the procfs fixture to ensure the test fixture directory is available
// and to test the integration with procfs package
func TestProcFSReaderWithInformer(t *testing.T) {
	informer, err := NewInformer(WithProcFSPath("./testdata/procfs"))
	require.NoError(t, err)

	// Test AllProcs
	err = informer.Refresh()
	require.NoError(t, err)

	assert.Equal(t, informer.Node().CPUUsageRatio, 0.0)

	processes := informer.Processes()
	assert.Len(t, processes.Running, 6)
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
	// by controller-runtime, and Hypervisor

	containerProcs := map[ContainerRuntime]int{}
	vmProcs := map[Hypervisor]int{}
	for _, p := range processes.Running {
		if p.Container != nil {
			containerProcs[p.Container.Runtime]++
		}

		if vm := p.VirtualMachine; vm != nil {
			vmProcs[vm.Hypervisor]++
		}

	}
	assert.Equal(t, 0, containerProcs[UnknownRuntime])
	assert.Equal(t, 1, containerProcs[DockerRuntime])
	assert.Equal(t, 3, containerProcs[PodmanRuntime])
	assert.Equal(t, 1, vmProcs[KVMHypervisor])

	vms := informer.VirtualMachines()
	assert.Len(t, vms.Running, 1)
	assert.Len(t, vms.Terminated, 0)
	vmID := "df12672f-fedb-4f6f-9d51-0166868835fb"
	assert.Contains(t, vms.Running, vmID)
	assert.Equal(t, vms.Running[vmID].Hypervisor, KVMHypervisor)
}

func TestProcessUpdateAfterRefresh(t *testing.T) {
	mockInformer := &MockProcReader{}
	fakeClock := testclock.NewFakeClock(time.Now())

	const (
		procCPUTime = 5.0
	)

	// Initial process state
	mockProc := &MockProcInfo{}
	mockProc.On("PID").Return(1001)
	mockProc.On("Comm").Return("process-initial", nil).Once()
	mockProc.On("Executable").Return("/bin/process-initial", nil).Once()
	mockProc.On("Cgroups").Return([]cGroup{{Path: "/system.slice/process.service"}}, nil).Once()
	mockProc.On("CPUTime").Return(procCPUTime, nil).Once()
	mockProc.On("Environ").Return([]string{}, nil).Maybe()
	mockProc.On("CmdLine").Return([]string{"/bin/process-initial"}, nil).Once()

	// For Init
	mockInformer.On("AllProcs").Return([]procInfo{mockProc}, nil).Once()

	// For first Refresh
	mockInformer.On("AllProcs").Return([]procInfo{mockProc}, nil).Once()
	mockInformer.On("CPUUsageRatio").Return(float64(0.0), nil).Once()

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
	node := informer.Node()
	assert.Equal(t, float64(0.0), node.CPUUsageRatio)
	assert.Equal(t, procCPUTime, node.ProcessTotalCPUTimeDelta)

	processes := informer.Processes()
	assert.Equal(t, "process-initial", processes.Running[1001].Comm)
	assert.Equal(t, "/bin/process-initial", processes.Running[1001].Exe)
	assert.Equal(t, float64(5.0), processes.Running[1001].CPUTotalTime)

	// Second refresh - process has changed comm and executable, with significant CPU time
	mockProc.On("Comm").Return("process-updated", nil).Once()
	mockProc.On("CmdLine").Return([]string{"/bin/process-updated"}, nil).Once()
	mockProc.On("Cgroups").Return([]cGroup{{Path: "/system.slice/process.service"}}, nil).Once()
	mockProc.On("Executable").Return("/bin/process-updated", nil).Once()
	mockProc.On("CPUTime").Return(float64(7.0), nil).Once() // 2.0 delta

	mockInformer.On("AllProcs").Return([]procInfo{mockProc}, nil).Once()
	mockInformer.On("CPUUsageRatio").Return(0.3, nil).Once()

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
	mockInformer.On("CPUUsageRatio").Return(0.3, nil).Once()
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
	mockProcFS := &MockProcReader{}
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
	mockProcFS.On("AllProcs").Return([]procInfo{mockProc}, nil).Once()

	// For first Refresh
	mockProcFS.On("AllProcs").Return([]procInfo{mockProc}, nil).Once()
	mockProcFS.On("CPUUsageRatio").Return(float64(0.0), nil).Once()

	informer, err := NewInformer(
		WithProcReader(mockProcFS),
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

	mockProcFS.On("AllProcs").Return([]procInfo{mockProc}, nil).Once()
	mockProcFS.On("CPUUsageRatio").Return(float64(0.5), nil).Once()
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

	mockProcFS.AssertExpectations(t)
	mockProc.AssertExpectations(t)
}

func TestProcFSReaderCPUUsageRatio(t *testing.T) {
	t.Run("First call returns zero usage", func(t *testing.T) {
		// Create a mock reader with no previous stats
		reader, err := NewProcFSReader("./testdata/procfs")
		require.NoError(t, err)

		ratio, err := reader.CPUUsageRatio()
		require.NoError(t, err)
		assert.Equal(t, float64(0), ratio, "First call should return 0 usage ratio")

		// magic numbers copied from testdata/procfs/stat
		// cpu  8608833 7605 4179891 1295036209 426072 15697167 1285624 0 5327346 0
		assert.Equal(t, 86088.33, reader.prevStat.User, "should read user time from procfs")
		assert.Equal(t, 41798.91, reader.prevStat.System, "should read system time from procfs")
		assert.Equal(t, 12950362.09, reader.prevStat.Idle, "should read idle time from procfs")
	})

	t.Run("Second call calculates correct ratio", func(t *testing.T) {
		reader, err := NewProcFSReader("./testdata/procfs")
		require.NoError(t, err)

		ratio, err := reader.CPUUsageRatio()
		require.NoError(t, err)
		assert.Equal(t, float64(0), ratio, "First call should return 0 usage ratio")
		// magic numbers copied from testdata/procfs/stat
		// cpu  8608833 7605 4179891 1295036209 426072 15697167 1285624 0 5327346 0
		assert.Equal(t, 86088.33, reader.prevStat.User, "should read user time from procfs")
		assert.Equal(t, 41798.91, reader.prevStat.System, "should read system time from procfs")
		assert.Equal(t, 12950362.09, reader.prevStat.Idle, "should read idle time from procfs")

		read := reader.prevStat
		reader.prevStat = procfs.CPUStat{
			User:    read.User - 500,
			Nice:    read.Nice - 100,
			System:  read.System - 300,
			Idle:    read.Idle - 650,
			Iowait:  read.Iowait - 50,
			IRQ:     read.IRQ - 25,
			SoftIRQ: read.SoftIRQ - 75,
			Steal:   read.Steal - 50,
		}

		// total = 500 + 100 + 300 + 650 + 50 + 25 + 75 + 50 = 1750
		// active = total - idle (idle + iowait) = 1750 - 700 = 1050
		// ratio = active / total = 1050/1750 = 0.6

		ratio, err = reader.CPUUsageRatio()
		assert.NoError(t, err, "should not error on second call")
		assert.InDelta(t, 0.6, ratio, 0.0001, "Second call should calculate correct ratio")
		assert.Equal(t, 86088.33, reader.prevStat.User, "should read user time from procfs")
		assert.Equal(t, 41798.91, reader.prevStat.System, "should read system time from procfs")
		assert.Equal(t, 12950362.09, reader.prevStat.Idle, "should read idle time from procfs")
	})
}

func TestResourceInformerCreation(t *testing.T) {
	t.Run("Name method returns service name", func(t *testing.T) {
		mockProcFS := &MockProcReader{}
		informer, err := NewInformer(WithProcReader(mockProcFS))
		require.NoError(t, err)

		assert.Equal(t, "resource-informer", informer.Name())
	})

	t.Run("NewInformer with nil procReader", func(t *testing.T) {
		_, err := NewInformer()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no procfs reader specified")
	})

	t.Run("NewInformer with procFSPath creates reader", func(t *testing.T) {
		_, err := NewInformer(WithProcFSPath("/proc"))
		assert.NoError(t, err)
	})

	t.Run("NewInformer with invalid procFSPath", func(t *testing.T) {
		_, err := NewInformer(WithProcFSPath("/invalid/path"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create procfs reader")
	})

	t.Run("WithLogger option function", func(t *testing.T) {
		logger := slog.Default()
		mockProcFS := &MockProcReader{}

		informer, err := NewInformer(
			WithProcReader(mockProcFS),
			WithLogger(logger),
		)
		require.NoError(t, err)
		assert.NotNil(t, informer)
	})
}

func TestResourceInformer_InitRefreshErr(t *testing.T) {
	t.Run("Init with failing procfs access", func(t *testing.T) {
		mockProcFS := &MockProcReader{}
		mockProcFS.On("AllProcs").Return([]procInfo(nil), errors.New("procfs access denied"))

		informer, err := NewInformer(WithProcReader(mockProcFS))
		require.NoError(t, err)

		err = informer.Init()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to access procfs")

		mockProcFS.AssertExpectations(t)
	})

	t.Run("refreshNode with CPUUsageRatio error", func(t *testing.T) {
		mockProcFS := &MockProcReader{}
		mockProcFS.On("AllProcs").Return([]procInfo{}, nil).Twice() // Once for Init, once for Refresh
		mockProcFS.On("CPUUsageRatio").Return(0.0, errors.New("cpu stat error"))

		informer, err := NewInformer(WithProcReader(mockProcFS))
		require.NoError(t, err)

		err = informer.Init()
		require.NoError(t, err)

		err = informer.Refresh()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get procfs usage")

		mockProcFS.AssertExpectations(t)
	})

	t.Run("updateVMCache with nil VM panic", func(t *testing.T) {
		mockProcFS := &MockProcReader{}
		informer, err := NewInformer(WithProcReader(mockProcFS))
		require.NoError(t, err)

		proc := &Process{
			PID:            123,
			Type:           VMProcess,
			VirtualMachine: nil, // This should cause panic
		}

		assert.Panics(t, func() {
			informer.updateVMCache(proc)
		})
	})
}

func TestNewProcFSReaderErrors(t *testing.T) {
	t.Run("NewProcFSReader with invalid path", func(t *testing.T) {
		_, err := NewProcFSReader("/invalid/nonexistent/path")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "could not read")
	})

	t.Run("NewProcFSReader with valid path", func(t *testing.T) {
		reader, err := NewProcFSReader("/proc")
		assert.NoError(t, err)
		assert.NotNil(t, reader)
	})
}

func TestProcWrapperErrors(t *testing.T) {
	t.Run("Cgroups with read error", func(t *testing.T) {
		mockProc := &MockProcInfo{}

		// Mock Cgroups to return error
		mockProc.On("Cgroups").Return([]cGroup(nil), errors.New("cgroup read error"))

		cgroups, err := mockProc.Cgroups()
		assert.Error(t, err)
		assert.Nil(t, cgroups)
		assert.Contains(t, err.Error(), "cgroup read error")

		mockProc.AssertExpectations(t)
	})

	t.Run("CPUTime with read error", func(t *testing.T) {
		mockProc := &MockProcInfo{}

		// Mock CPUTime to return error
		mockProc.On("CPUTime").Return(float64(0), errors.New("stat read error"))

		cpuTime, err := mockProc.CPUTime()
		assert.Error(t, err)
		assert.Equal(t, float64(0), cpuTime)
		assert.Contains(t, err.Error(), "stat read error")

		mockProc.AssertExpectations(t)
	})
}

func TestRefreshConcurrency(t *testing.T) {
	// container for pod dependency testing
	mockProc1 := &MockProcInfo{}
	mockProc1.On("PID").Return(2001)
	mockProc1.On("Comm").Return("container-proc", nil)
	mockProc1.On("Executable").Return("/bin/container-app", nil)
	mockProc1.On("CmdLine").Return([]string{"/bin/container-app"}, nil)
	mockProc1.On("Environ").Return([]string{"CONTAINER_NAME=test-container"}, nil)
	ctnrID, cgPath := mockContainerIDAndPath(PodmanRuntime)
	mockProc1.On("Cgroups").Return([]cGroup{{Path: cgPath}}, nil)
	mockProc1.On("CPUTime").Return(float64(3.0), nil)

	// VM process
	mockProc2 := &MockProcInfo{}
	mockProc2.On("PID").Return(3001)
	mockProc2.On("Comm").Return("qemu-system-x86_64", nil)
	mockProc2.On("Executable").Return("/usr/bin/qemu-system-x86_64", nil)
	mockProc2.On("CmdLine").Return([]string{
		"/usr/bin/qemu-system-x86_64",
		"-uuid", "550e8400-e29b-41d4-a716-446655440000",
		"-name", "test-vm",
	}, nil)
	mockProc2.On("Environ").Return([]string{}, nil).Maybe()
	mockProc2.On("Cgroups").Return([]cGroup{{Path: "/system.slice/libvirt.service"}}, nil)
	mockProc2.On("CPUTime").Return(float64(2.0), nil)

	// Regular process
	mockProc3 := &MockProcInfo{}
	mockProc3.On("PID").Return(1001)
	mockProc3.On("Comm").Return("regular-proc", nil)
	mockProc3.On("Executable").Return("/bin/regular", nil)
	mockProc3.On("Cgroups").Return([]cGroup{{Path: "/system.slice/regular.service"}}, nil)
	mockProc3.On("CPUTime").Return(float64(1.0), nil)
	mockProc3.On("Environ").Return([]string{}, nil).Maybe()
	mockProc3.On("CmdLine").Return([]string{"/bin/regular"}, nil).Maybe()

	mockInformer := &MockProcReader{}
	mockInformer.On("AllProcs").Return([]procInfo{}, nil).Once()
	mockInformer.On("AllProcs").Return([]procInfo{mockProc1, mockProc2, mockProc3}, nil).Once()
	mockInformer.On("CPUUsageRatio").Return(float64(0.1), nil).Once()

	// Mock pod informer to test pod dependency on containers
	mockPodInformer := new(mockPodInformer)
	mockPodInformer.On("LookupByContainerID", ctnrID).Return(
		&pod.ContainerInfo{
			PodID:         "pod123",
			PodName:       "mypod",
			Namespace:     "default",
			ContainerName: "my-container",
		}, true, nil,
	)

	informer, err := NewInformer(
		WithProcReader(mockInformer),
		WithPodInformer(mockPodInformer),
	)
	require.NoError(t, err)

	err = informer.Init()
	require.NoError(t, err)

	// Single Refresh call should work without races
	// Goroutine 1: containers → pods (sequential within goroutine)
	// Goroutine 2: VMs (independent)
	// Goroutine 3: node (independent)
	err = informer.Refresh()
	require.NoError(t, err)

	// Verify all workload types are processed correctly
	processes := informer.Processes()
	containers := informer.Containers()
	vms := informer.VirtualMachines()
	pods := informer.Pods()
	node := informer.Node()

	// Should have 3 processes (1 regular, 1 container, 1 VM)
	assert.Len(t, processes.Running, 3, "Expected 3 running processes")

	// Should have 1 container
	assert.Len(t, containers.Running, 1, "Expected 1 running container")
	assert.Contains(t, containers.Running, ctnrID, "Expected container to be present")

	// Should have 1 VM
	assert.Len(t, vms.Running, 1, "Expected 1 running VM")
	assert.Contains(t, vms.Running, "550e8400-e29b-41d4-a716-446655440000", "Expected VM to be present")

	// Should have 1 pod (depends on container from stage 1)
	assert.Len(t, pods.Running, 1, "Expected 1 running pod")
	assert.Contains(t, pods.Running, "pod123", "Expected pod to be present")

	// Node should have aggregated CPU data from all processes
	assert.NotNil(t, node, "Node should not be nil")
	assert.Equal(t, float64(0.1), node.CPUUsageRatio, "CPU usage ratio should be set")
	expectedTotalCPU := float64(6.0) // 3.0 + 2.0 + 1.0
	assert.Equal(t, expectedTotalCPU, node.ProcessTotalCPUTimeDelta, "Total CPU delta should be sum of all processes")

	mockInformer.AssertExpectations(t)
	mockPodInformer.AssertExpectations(t)
	mockProc1.AssertExpectations(t)
	mockProc2.AssertExpectations(t)
	mockProc3.AssertExpectations(t)
}
