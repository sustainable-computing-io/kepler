// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// procForPID builds a mock process that looks like a container workload.
func procForPID(pid int, comm string, startTime uint64, cgroupPath string, environ []string) *MockProcInfo {
	proc := &MockProcInfo{}
	proc.On("PID").Return(pid)
	proc.On("Comm").Return(comm, nil)
	proc.On("Executable").Return("/usr/bin/"+comm, nil)
	proc.On("StartTime").Return(startTime, nil)
	proc.On("Cgroups").Return([]cGroup{{Path: cgroupPath}}, nil)
	proc.On("Environ").Return(environ, nil)
	proc.On("CmdLine").Return([]string{"/usr/bin/" + comm}, nil)
	proc.On("CPUTime").Return(float64(1), nil)
	return proc
}

func TestUpdateProcessCacheRecycledPID(t *testing.T) {
	const pid = 4242
	const comm = "python3"

	first := procForPID(pid, comm, 100,
		"/docker-ce82d94d69e1fbbc7feeb66930c69e9b96d9f151f594773e5d0e342741d15437",
		[]string{"CONTAINER_NAME=first", "KUBERNETES_NAMESPACE=ns-one", "KUBERNETES_POD_NAME=pod-one"})

	ri := &resourceInformer{
		procCache: map[int]*Process{},
	}

	p, err := ri.updateProcessCache(first)
	require.NoError(t, err)
	require.NotNil(t, p.Container)
	assert.Equal(t, "first", p.Container.Name)

	// Same PID, same comm, different process
	second := procForPID(pid, comm, 900,
		"/docker-aa11d94d69e1fbbc7feeb66930c69e9b96d9f151f594773e5d0e342741daaaaa",
		[]string{"CONTAINER_NAME=second", "KUBERNETES_NAMESPACE=ns-two", "KUBERNETES_POD_NAME=pod-two"})

	p, err = ri.updateProcessCache(second)
	require.NoError(t, err)
	require.NotNil(t, p.Container)
	assert.Equal(t, uint64(900), p.StartTime)
	assert.Equal(t, "second", p.Container.Name, "recycled PID kept the dead process's container")
}

func TestUpdateProcessCacheSameProcess(t *testing.T) {
	const pid = 4242

	ri := &resourceInformer{
		procCache: map[int]*Process{},
	}

	first := procForPID(pid, "python3", 100,
		"/docker-ce82d94d69e1fbbc7feeb66930c69e9b96d9f151f594773e5d0e342741d15437",
		[]string{"CONTAINER_NAME=first"})
	firstProc, err := ri.updateProcessCache(first)
	require.NoError(t, err)

	second := procForPID(pid, "python3", 100,
		"/docker-ce82d94d69e1fbbc7feeb66930c69e9b96d9f151f594773e5d0e342741d15437",
		[]string{"CONTAINER_NAME=first"})
	secondProc, err := ri.updateProcessCache(second)
	require.NoError(t, err)

	assert.Same(t, firstProc, secondProc, "an unchanged process should keep its cache entry")
}
