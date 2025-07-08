// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"context"
	"testing"
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

func (m *MockCPUPowerMeter) PrimaryEnergyZone() (EnergyZone, error) {
	args := m.Called()
	return args.Get(0).(EnergyZone), args.Error(1)
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

func (m *MockResourceInformer) SetExpectations(t *testing.T, tr *TestResource) {
	t.Helper()
	if tr.Node != nil {
		m.On("Node").Return(tr.Node, nil)
	}
	if tr.Processes != nil {
		m.On("Processes").Return(tr.Processes, nil)
	}
	if tr.Containers != nil {
		m.On("Containers").Return(tr.Containers, nil)
	}
	if tr.VirtualMachines != nil {
		m.On("VirtualMachines").Return(tr.VirtualMachines, nil)
	}
	if tr.Pods != nil {
		m.On("Pods").Return(tr.Pods, nil)
	}
	t.Cleanup(func() {
		m.ExpectedCalls = nil
	})
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

func (m *MockResourceInformer) Node() *resource.Node {
	args := m.Called()
	return args.Get(0).(*resource.Node)
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
func createNodeSnapshot(zones []EnergyZone, timestamp time.Time, usageRatio float64) *Node {
	node := &Node{
		Timestamp:  timestamp,
		UsageRatio: usageRatio,
		Zones:      make(NodeZoneUsageMap),
	}

	for _, zone := range zones {
		node.Zones[zone] = NodeUsage{
			EnergyTotal:       200 * Joule,
			activeEnergy:      Energy(usageRatio * float64(100*Joule)),
			ActiveEnergyTotal: Energy(usageRatio * float64(100*Joule)),
			IdleEnergyTotal:   Energy((1 - usageRatio) * float64(100*Joule)),

			Power:       50 * Watt,
			ActivePower: Power(usageRatio * float64(50*Watt)),
			IdlePower:   Power((1 - usageRatio) * float64(50*Watt)),
		}
	}

	return node
}

type TestResource struct {
	Node            *resource.Node
	Processes       *resource.Processes
	Containers      *resource.Containers
	VirtualMachines *resource.VirtualMachines
	Pods            *resource.Pods
}

type resourceOpts struct {
	nodeCpuUsage     float64
	nodeCpuTimeDelta float64
	omit             map[testResourceType]bool
}

type resOptFn func(*resourceOpts)

type testResourceType int

const (
	testNode testResourceType = iota
	testProcesses
	testContainers
	testVMs
	testPods
)

func createOnly(rs ...testResourceType) resOptFn {
	return func(opts *resourceOpts) {
		opts.omit = map[testResourceType]bool{
			testNode:       true,
			testProcesses:  true,
			testContainers: true,
			testVMs:        true,
			testPods:       true,
		}

		for _, r := range rs {
			opts.omit[r] = false
		}
	}
}

func withNodeCpuUsage(usage float64) resOptFn {
	return func(opts *resourceOpts) {
		opts.nodeCpuUsage = usage
	}
}

func withNodeCpuTimeDelta(delta float64) resOptFn {
	return func(opts *resourceOpts) {
		opts.nodeCpuTimeDelta = delta
	}
}

// CreateTestResources creates test processes with container associations
func CreateTestResources(opts ...resOptFn) *TestResource {
	opt := resourceOpts{
		nodeCpuUsage:     0.5,
		nodeCpuTimeDelta: 200.0,
		omit:             map[testResourceType]bool{},
	}

	for _, apply := range opts {
		apply(&opt)
	}

	node := &resource.Node{
		CPUUsageRatio:            opt.nodeCpuUsage,
		ProcessTotalCPUTimeDelta: opt.nodeCpuTimeDelta,
	}

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

	pod1 := &resource.Pod{
		ID:        "pod-id-1",
		Name:      "pod-name-1",
		Namespace: "namespace=1",
	}

	// Create containers
	container1 := &resource.Container{
		ID:      "container-1",
		Name:    "test-container-1",
		Runtime: resource.DockerRuntime,
		Pod:     pod1,
	}

	container2 := &resource.Container{
		ID:      "container-2",
		Name:    "test-container-2",
		Runtime: resource.PodmanRuntime,
	}

	processes := &resource.Processes{
		Running: map[int]*resource.Process{
			123: {
				PID:          123,
				Comm:         "process1",
				Exe:          "/usr/bin/process1",
				CPUTotalTime: 100.0,
				CPUTimeDelta: 0.3 * node.ProcessTotalCPUTimeDelta, // 30% of total CPU time | cum: 30
				Container:    container1,
				Type:         resource.ContainerProcess,
			},
			1231: {
				PID:          1231,
				Comm:         "process4",
				Exe:          "/usr/bin/process4",
				CPUTotalTime: 100.0,
				CPUTimeDelta: 0.1 * node.ProcessTotalCPUTimeDelta, // 10% | cum: 40
				Container:    container1,
				Type:         resource.ContainerProcess,
			},
			456: {
				PID:          456,
				Comm:         "process2",
				Exe:          "/usr/bin/process2",
				CPUTotalTime: 200.0,
				CPUTimeDelta: 0.20 * node.ProcessTotalCPUTimeDelta, // 20% | cum: 60
				Container:    container2,
				Type:         resource.ContainerProcess,
			},
			789: {
				PID:          789,
				Comm:         "process3",
				Exe:          "/usr/bin/process3",
				CPUTotalTime: 500.0,
				CPUTimeDelta: 0.15 * node.ProcessTotalCPUTimeDelta, // 15% | cum: 75
				Type:         resource.RegularProcess,
			},
			// VM processes
			1001: {
				PID:            1001,
				Comm:           "qemu-vm1",
				Exe:            "/usr/bin/qemu-system-x86_64",
				CPUTotalTime:   300.0,
				CPUTimeDelta:   0.20 * node.ProcessTotalCPUTimeDelta, // 20% | cum: 95
				VirtualMachine: vm1,
				Type:           resource.VMProcess,
			},
			1002: {
				PID:            1002,
				Comm:           "qemu-vm2",
				Exe:            "/usr/bin/qemu-system-x86_64",
				CPUTotalTime:   200.0,
				CPUTimeDelta:   0.05 * node.ProcessTotalCPUTimeDelta, // 5%  | cum: 100
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

	vms := &resource.VirtualMachines{
		Running: map[string]*resource.VirtualMachine{
			vm1.ID: vm1,
			vm2.ID: vm2,
		},
		Terminated: map[string]*resource.VirtualMachine{},
	}
	pod1.CPUTimeDelta = container1.CPUTimeDelta

	pods := &resource.Pods{
		Running: map[string]*resource.Pod{
			pod1.ID: pod1,
		},
		Terminated: map[string]*resource.Pod{},
	}
	if opt.omit[testNode] {
		node = nil
	}
	if opt.omit[testProcesses] {
		processes = nil
	}

	if opt.omit[testContainers] {
		containers = nil
	}
	if opt.omit[testVMs] {
		vms = nil
	}
	if opt.omit[testPods] {
		pods = nil
	}

	return &TestResource{node, processes, containers, vms, pods}
}
