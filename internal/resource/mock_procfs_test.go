// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"
	"math/rand"
	"strings"

	"github.com/stretchr/testify/mock"
	"github.com/sustainable-computing-io/kepler/internal/k8s/pod"
)

// MockProcInfo is a mock implementation of procInfo for testing
type MockProcInfo struct {
	mock.Mock
}

func (m *MockProcInfo) PID() int {
	args := m.Called()
	return args.Int(0)
}

func (m *MockProcInfo) Comm() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockProcInfo) Executable() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockProcInfo) Cgroups() ([]cGroup, error) {
	args := m.Called()
	return args.Get(0).([]cGroup), args.Error(1)
}

func (m *MockProcInfo) Environ() ([]string, error) {
	args := m.Called()
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockProcInfo) CmdLine() ([]string, error) {
	args := m.Called()
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockProcInfo) CPUTime() (float64, error) {
	args := m.Called()
	return args.Get(0).(float64), args.Error(1)
}

// MockProcReader is a mock implementation of procInformer for testing
type MockProcReader struct {
	mock.Mock
}

func (m *MockProcReader) AllProcs() ([]procInfo, error) {
	args := m.Called()
	return args.Get(0).([]procInfo), args.Error(1)
}

func (m *MockProcReader) CPUUsageRatio() (float64, error) {
	args := m.Called()
	return args.Get(0).(float64), args.Error(1)
}

func mockContainerIDAndPath(rt ContainerRuntime) (string, string) {
	containerPaths := map[ContainerRuntime]string{
		DockerRuntime:     "/docker/<id>",
		ContainerDRuntime: "/containerd/<id>",
		CrioRuntime:       "/crio/<id>",
		PodmanRuntime:     "0::/machine.slice/libpod-<id>.scope",
		KubePodsRuntime:   "/kubepods/pod<id>/<id>",
	}
	if _, ok := containerPaths[rt]; !ok {
		panic("unknown container runtime")
	}
	runes := []rune("abcdef0123456789")

	rand64 := make([]rune, 64)
	for i := range rand64 {
		rand64[i] = runes[rand.Intn(len(runes))]
	}
	id := string(rand64)
	return id, strings.ReplaceAll(containerPaths[rt], "<id>", id)
}

type mockPodInformer struct {
	mock.Mock
}

func (m *mockPodInformer) Init() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockPodInformer) Run(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockPodInformer) LookupByContainerID(containerID string) (*pod.ContainerInfo, bool, error) {
	args := m.Called(containerID)
	if podInfo, ok := args.Get(0).(*pod.ContainerInfo); ok {
		return podInfo, args.Bool(1), args.Error(2)
	}
	return nil, args.Bool(1), args.Error(2)
}

func (m *mockPodInformer) Name() string {
	args := m.Called()
	return args.String(0)
}
