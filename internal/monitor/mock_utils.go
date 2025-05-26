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

func (m *MockResourceInformer) SetupTestResources(tr *TestResource) {
	if tr.Processes != nil {
		m.On("Processes").Return(tr.Processes, nil)
	}
	if tr.Containers != nil {
		m.On("Containers").Return(tr.Containers, nil)
	}
	if tr.VirtualMachines != nil {
		m.On("VirtualMachines").Return(tr.VirtualMachines, nil)
	}
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

type TestResource struct {
	Processes       *resource.Processes
	Containers      *resource.Containers
	VirtualMachines *resource.VirtualMachines
}

// CreateTestResources creates test processes with container associations
func CreateTestResources() *TestResource {
	//  VMs
	vm1 := &resource.VirtualMachine{
		ID:         "vm-1",
		Name:       "test-vm-1",
		Hypervisor: resource.KVMHypervisor,
	}

	vm2 := &resource.VirtualMachine{
		ID:         "vm-2",
		Name:       "test-vm-2",
		Hypervisor: resource.KVMHypervisor,
	}

	// Create containers
	container1 := &resource.Container{
		ID:      "container-1",
		Name:    "test-container-1",
		Runtime: resource.DockerRuntime,
	}

	container2 := &resource.Container{
		ID:      "container-2",
		Name:    "test-container-2",
		Runtime: resource.PodmanRuntime,
	}

	processes := &resource.Processes{
		NodeCPUTimeDelta: 100.0, // Total node CPU time delta
		Running: map[int]*resource.Process{
			123: {
				PID:          123,
				Comm:         "process1",
				Exe:          "/usr/bin/process1",
				CPUTotalTime: 100.0,
				CPUTimeDelta: 30.0, // 30% of total CPU time | cum: 30
				Container:    container1,
				Type:         resource.ContainerProcess,
			},
			1231: {
				PID:          1231,
				Comm:         "process4",
				Exe:          "/usr/bin/process4",
				CPUTotalTime: 100.0,
				CPUTimeDelta: 10.0, // 10% | cum: 40
				Container:    container1,
				Type:         resource.ContainerProcess,
			},
			456: {
				PID:          456,
				Comm:         "process2",
				Exe:          "/usr/bin/process2",
				CPUTotalTime: 200.0,
				CPUTimeDelta: 20.0, // 20% | cum: 60
				Container:    container2,
				Type:         resource.ContainerProcess,
			},
			789: {
				PID:          789,
				Comm:         "process3",
				Exe:          "/usr/bin/process3",
				CPUTotalTime: 500.0,
				CPUTimeDelta: 15.0, // 15% | cum: 75
				Type:         resource.RegularProcess,
			},
			// VM processes
			1001: {
				PID:            1001,
				Comm:           "qemu-vm1",
				Exe:            "/usr/bin/qemu-system-x86_64",
				CPUTotalTime:   300.0,
				CPUTimeDelta:   20.0, // 20% | cum: 95
				VirtualMachine: vm1,
				Type:           resource.VMProcess,
			},
			1002: {
				PID:            1002,
				Comm:           "qemu-vm2",
				Exe:            "/usr/bin/qemu-system-x86_64",
				CPUTotalTime:   200.0,
				CPUTimeDelta:   5.0, // 5%  | cum: 100
				VirtualMachine: vm2,
				Type:           resource.VMProcess,
			},
		},
		Terminated: map[int]*resource.Process{},
	}

	// Calculate container CPU times from their processes
	container1.CPUTimeDelta = processes.Running[123].CPUTimeDelta + processes.Running[1231].CPUTimeDelta // 40%
	container2.CPUTimeDelta = processes.Running[456].CPUTimeDelta                                        // 20%

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

	// Calculate VM CPU times from their processes
	vm1.CPUTimeDelta = processes.Running[1001].CPUTimeDelta // 20%
	vm2.CPUTimeDelta = processes.Running[1002].CPUTimeDelta // 5%

	vm1.CPUTotalTime = processes.Running[1001].CPUTotalTime
	vm2.CPUTotalTime = processes.Running[1002].CPUTotalTime

	virtualMachines := &resource.VirtualMachines{
		NodeCPUTimeDelta: 100.0,
		Running: map[string]*resource.VirtualMachine{
			vm1.ID: vm1,
			vm2.ID: vm2,
		},
		Terminated: map[string]*resource.VirtualMachine{},
	}

	return &TestResource{processes, containers, virtualMachines}
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
