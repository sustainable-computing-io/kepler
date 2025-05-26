// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"context"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/sustainable-computing-io/kepler/internal/device"
	"github.com/sustainable-computing-io/kepler/internal/resource"
)

// MockCPUPowerMeter is a mock implementation of CPUPowerMeter
type MockCPUPowerMeter struct {
	mock.Mock
}

func (m *MockCPUPowerMeter) Zones() ([]EnergyZone, error) {
	args := m.Called()
	return args.Get(0).([]EnergyZone), args.Error(1)
}

func (m *MockCPUPowerMeter) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockCPUPowerMeter) Run(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

type MockEnergyZone struct {
	mock.Mock
}

func (m *MockEnergyZone) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockEnergyZone) Index() int {
	args := m.Called()
	return args.Int(0)
}

func (m *MockEnergyZone) Path() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockEnergyZone) Energy() (Energy, error) {
	args := m.Called()
	return args.Get(0).(Energy), args.Error(1)
}

func (m *MockEnergyZone) MaxEnergy() Energy {
	args := m.Called()
	return args.Get(0).(Energy)
}

// MockResourceInformer is a mock implementation of resource.Informer
type MockResourceInformer struct {
	mock.Mock
}

func (m *MockResourceInformer) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockResourceInformer) Init() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockResourceInformer) Refresh() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockResourceInformer) Processes() *resource.Processes {
	args := m.Called()
	return args.Get(0).(*resource.Processes)
}

func (m *MockResourceInformer) Containers() *resource.Containers {
	args := m.Called()
	return args.Get(0).(*resource.Containers)
}

func (m *MockResourceInformer) VirtualMachines() *resource.VirtualMachines {
	args := m.Called()
	return args.Get(0).(*resource.VirtualMachines)
}

func (m *MockResourceInformer) Pods() *resource.Pods {
	args := m.Called()
	return args.Get(0).(*resource.Pods)
}

var _ resource.Informer = (*MockResourceInformer)(nil)

// Helper functions for creating test data

// CreateTestZones creates mock energy zones for testing
func CreateTestZones() []EnergyZone {
	pkg := device.NewMockRaplZone("package-0", 0, "/sys/class/powercap/intel-rapl/intel-rapl:0", 1000*Joule)
	core := device.NewMockRaplZone("core-0", 0, "/sys/class/powercap/intel-rapl/intel-rapl:0/intel-rapl:0:0", 500*Joule)
	return []EnergyZone{pkg, core}
}

// createNodeSnapshot creates a node snapshot with realistic power values
func createNodeSnapshot(zones []EnergyZone, timestamp time.Time) *Node {
	node := &Node{
		Timestamp: timestamp,
		Zones:     make(ZoneUsageMap),
	}

	for _, zone := range zones {
		node.Zones[zone] = &Usage{
			Absolute: 100 * Joule,
			Delta:    50 * Joule,
			Power:    50 * Watt,
		}
	}

	return node
}

// CreateTestResources creates test processes with container associations
func CreateTestResources() (*resource.Processes, *resource.Containers) {
	container1 := &resource.Container{
		ID:      "container-1",
		Name:    "test-container-1",
		Runtime: resource.DockerRuntime,
		// has proc 123 and 1231 running
	}

	container2 := &resource.Container{
		ID:      "container-2",
		Name:    "test-container-2",
		Runtime: resource.PodmanRuntime,
		// has proc 456 running
	}

	processes := &resource.Processes{
		NodeCPUTimeDelta: 100.0, // Total node CPU time delta
		Running: map[int]*resource.Process{
			123: {
				PID:          123,
				Comm:         "process1",
				Exe:          "/usr/bin/process1",
				CPUTotalTime: 100.0,
				CPUTimeDelta: 30.0, // 30% of total CPU time
				Container:    container1,
			},
			1231: {
				PID:          1231,
				Comm:         "process4",
				Exe:          "/usr/bin/process4",
				CPUTotalTime: 100.0,
				CPUTimeDelta: 10.0, // 10% of total CPU time
				Container:    container1,
			},
			456: {
				PID:          456,
				Comm:         "process2",
				Exe:          "/usr/bin/process2",
				CPUTotalTime: 200.0,
				CPUTimeDelta: 40.0, // 40% of total CPU time
				Container:    container2,
			},
			789: {
				PID:          789,
				Comm:         "process3",
				Exe:          "/usr/bin/process3",
				CPUTotalTime: 500.0,
				CPUTimeDelta: 20.0, // 20% of total CPU time
				Container:    nil,  // Not in a container
			},
		},
		Terminated: map[int]*resource.Process{},
	}
	// replicate what resource.Refresh() does
	container1.CPUTimeDelta = processes.Running[123].CPUTimeDelta + processes.Running[1231].CPUTimeDelta
	container2.CPUTimeDelta = processes.Running[456].CPUTimeDelta

	container1.CPUTotalTime = processes.Running[123].CPUTotalTime + processes.Running[1231].CPUTotalTime
	container2.CPUTotalTime = processes.Running[456].CPUTotalTime

	containers := &resource.Containers{
		NodeCPUTimeDelta: 100.0,
		Running: map[string]*resource.Container{
			container1.ID: container1,
			container2.ID: container2,
		},
		Terminated: map[string]*resource.Container{},
	}

	return processes, containers
}

// // CreateTestContainers creates test containers with CPU time deltas
// func CreateTestContainers() *resource.Containers {
// 	return &resource.Containers{
// 		NodeCPUTimeDelta: 80.0, // Container processes only (30+50)
// 		Running: map[string]*resource.Container{
// 			"container-1": {
// 				ID:           "container-1",
// 				Name:         "test-container-1",
// 				Runtime:      resource.DockerRuntime,
// 				CPUTotalTime: 15.0,
// 				CPUTimeDelta: 30.0,
// 			},
// 			"container-2": {
// 				ID:           "container-2",
// 				Name:         "test-container-2",
// 				Runtime:      resource.PodmanRuntime,
// 				CPUTotalTime: 25.0,
// 				CPUTimeDelta: 50.0,
// 			},
// 		},
// 		Terminated: map[string]*resource.Container{},
// 	}
// }
