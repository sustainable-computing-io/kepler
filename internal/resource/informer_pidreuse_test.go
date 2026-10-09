// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"fmt"
	"io/fs"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mockProcessForCache(t *testing.T, cpuStat procCPUStat, containerID string) *MockProcInfo {
	t.Helper()
	proc := &MockProcInfo{}
	proc.On("PID").Return(4001)
	proc.On("CPUStat").Return(cpuStat, nil).Once()
	proc.On("Comm").Return("python3", nil).Maybe()
	proc.On("Executable").Return("/usr/bin/python3", nil).Maybe()
	proc.On("Cgroups").Return([]cGroup{{Path: "/docker/" + containerID}}, nil).Maybe()
	proc.On("Environ").Return([]string{"CONTAINER_NAME=" + containerID}, nil).Maybe()
	proc.On("CmdLine").Return([]string{"/usr/bin/python3"}, nil).Maybe()
	return proc
}

func TestUpdateProcessCachePIDReuse(t *testing.T) {
	tests := []struct {
		name   string
		oldCPU float64
		newCPU float64
	}{
		{name: "lower CPU total", oldCPU: 20, newCPU: 1},
		{name: "lower CPU total to zero", oldCPU: 20, newCPU: 0},
		{name: "equal CPU totals", oldCPU: 1, newCPU: 1},
		{name: "zero CPU totals", oldCPU: 0, newCPU: 0},
		{name: "higher CPU total", oldCPU: 1, newCPU: 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldID, newID := strings.Repeat("a", 64), strings.Repeat("b", 64)
			oldProc := mockProcessForCache(t, procCPUStat{CPUTime: tt.oldCPU, StartTime: 100}, oldID)
			newProc := mockProcessForCache(t, procCPUStat{CPUTime: tt.newCPU, StartTime: 900}, newID)
			informer, err := NewInformer(WithProcReader(&MockProcReader{}))
			require.NoError(t, err)

			oldCached, err := informer.updateProcessCache(oldProc)
			require.NoError(t, err)
			require.NotNil(t, oldCached.Container)
			assert.Equal(t, oldID, oldCached.Container.ID)

			newCached, err := informer.updateProcessCache(newProc)
			require.NoError(t, err)
			require.NotNil(t, newCached.Container)
			assert.NotSame(t, oldCached, newCached)
			assert.Same(t, newCached, informer.procCache[4001])
			assert.Equal(t, newID, newCached.Container.ID)
			assert.Equal(t, tt.newCPU, newCached.CPUTotalTime)
			assert.Equal(t, tt.newCPU, newCached.CPUTimeDelta)
			assert.Equal(t, uint64(900), newCached.StartTime)
			assert.Equal(t, tt.oldCPU, oldCached.CPUTotalTime)
			assert.Equal(t, uint64(100), oldCached.StartTime)

			oldProc.AssertExpectations(t)
			newProc.AssertExpectations(t)
		})
	}
}

func TestUpdateProcessCachePIDReuseError(t *testing.T) {
	for _, method := range []string{"CPUStat", "Comm"} {
		t.Run(method, func(t *testing.T) {
			oldProc := mockProcessForCache(t, procCPUStat{CPUTime: 20, StartTime: 100}, strings.Repeat("a", 64))
			informer, err := NewInformer(WithProcReader(&MockProcReader{}))
			require.NoError(t, err)
			oldCached, err := informer.updateProcessCache(oldProc)
			require.NoError(t, err)

			newProc := &MockProcInfo{}
			newProc.On("PID").Return(4001)
			if method == "CPUStat" {
				newProc.On("CPUStat").Return(procCPUStat{}, assert.AnError).Once()
			} else {
				newProc.On("CPUStat").Return(procCPUStat{CPUTime: 1, StartTime: 900}, nil).Once()
				newProc.On("Comm").Return("", assert.AnError).Once()
			}

			newCached, err := informer.updateProcessCache(newProc)
			require.ErrorIs(t, err, assert.AnError)
			assert.Nil(t, newCached)
			assert.Same(t, oldCached, informer.procCache[4001])
			assert.Equal(t, float64(20), oldCached.CPUTotalTime)
			assert.Equal(t, uint64(100), oldCached.StartTime)
			oldProc.AssertExpectations(t)
			newProc.AssertExpectations(t)
		})
	}
}

func TestRefreshPIDReuse(t *testing.T) {
	oldID, newID := strings.Repeat("a", 64), strings.Repeat("b", 64)
	oldProc := mockProcessForCache(t, procCPUStat{CPUTime: 1, StartTime: 100}, oldID)
	newProc := mockProcessForCache(t, procCPUStat{CPUTime: 4, StartTime: 900}, newID)
	nextProc := mockProcessForCache(t, procCPUStat{CPUTime: 6, StartTime: 900}, newID)
	reader := &MockProcReader{}
	reader.On("AllProcs").Return([]procInfo{oldProc}, nil).Once()
	reader.On("AllProcs").Return([]procInfo{newProc}, nil).Once()
	reader.On("AllProcs").Return([]procInfo{nextProc}, nil).Once()
	reader.On("CPUUsageRatio").Return(float64(0.2), nil).Times(3)
	informer, err := NewInformer(WithProcReader(reader))
	require.NoError(t, err)

	require.NoError(t, informer.Refresh())
	require.Contains(t, informer.Containers().Running, oldID)
	firstCached := informer.Processes().Running[4001]

	require.NoError(t, informer.Refresh())
	require.Contains(t, informer.Processes().Running, 4001)
	process := informer.Processes().Running[4001]
	assert.NotSame(t, firstCached, process)
	require.Contains(t, informer.Processes().Terminated, 4001)
	assert.Same(t, firstCached, informer.Processes().Terminated[4001])
	assert.Equal(t, uint64(100), firstCached.StartTime)
	assert.Equal(t, float64(1), firstCached.CPUTotalTime)
	assert.Same(t, process, informer.procCache[4001])
	assert.Equal(t, float64(4), process.CPUTimeDelta)
	assert.Equal(t, float64(4), informer.Node().ProcessTotalCPUTimeDelta)
	require.Contains(t, informer.Containers().Running, newID)
	assert.Equal(t, float64(4), informer.Containers().Running[newID].CPUTimeDelta)
	assert.Equal(t, float64(4), informer.Containers().Running[newID].CPUTotalTime)
	assert.NotContains(t, informer.Containers().Running, oldID)
	assert.Contains(t, informer.Containers().Terminated, oldID)

	require.NoError(t, informer.Refresh())
	assert.Same(t, process, informer.Processes().Running[4001])
	assert.Equal(t, float64(2), process.CPUTimeDelta)
	assert.Empty(t, informer.Processes().Terminated)

	oldProc.AssertExpectations(t)
	newProc.AssertExpectations(t)
	nextProc.AssertExpectations(t)
	reader.AssertExpectations(t)
}

func TestRefreshProcessesMissingProcess(t *testing.T) {
	missing := &fs.PathError{Op: "open", Path: "/proc/4001/stat", Err: fs.ErrNotExist}
	for _, tt := range []struct {
		name string
		err  error
	}{
		{name: "raw error", err: missing},
		{name: "wrapped error", err: fmt.Errorf("failed to read stat: %w", missing)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			proc := &MockProcInfo{}
			proc.On("PID").Return(4001)
			proc.On("CPUStat").Return(procCPUStat{}, tt.err).Once()
			reader := &MockProcReader{}
			reader.On("AllProcs").Return([]procInfo{proc}, nil).Once()
			informer, err := NewInformer(WithProcReader(reader))
			require.NoError(t, err)

			_, _, err = informer.refreshProcesses()
			require.NoError(t, err)
			assert.Empty(t, informer.Processes().Running)
			proc.AssertExpectations(t)
			reader.AssertExpectations(t)
		})
	}
}
