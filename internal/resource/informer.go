// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/sustainable-computing-io/kepler/internal/service"
	"k8s.io/utils/clock"
)

// Processes represents sets of running and terminated processes
type Processes struct {
	NodeCPUTimeDelta float64
	Running          map[int]*Process
	Terminated       map[int]*Process
}

// Containers represents sets of running and terminated containers
type Containers struct {
	NodeCPUTimeDelta float64
	Running          map[string]*Container
	Terminated       map[string]*Container
}

// VirtualMachines represents sets of running and terminated VMs
type VirtualMachines struct {
	NodeCPUTimeDelta float64
	Running          map[string]*VirtualMachine
	Terminated       map[string]*VirtualMachine
}

// Informer provides the interface for accessing process and container information
type Informer interface {
	service.Initializer
	// Refresh updates the internal state
	Refresh() error
	// Processes returns the current running and terminated processes
	Processes() *Processes
	// Containers returns the current running and terminated containers
	Containers() *Containers
	// VirtualMachines returns the current running and terminated VMs
	VirtualMachines() *VirtualMachines
}

// resourceInformer is the default implementation of the resource tracking service
type resourceInformer struct {
	logger *slog.Logger
	fs     allProcReader
	clock  clock.Clock

	// Process tracking
	procCache map[int]*Process
	processes *Processes

	// Container tracking
	containerCache map[string]*Container
	containers     *Containers

	// VM tracking
	vmCache map[string]*VirtualMachine
	vms     *VirtualMachines

	lastScanTime time.Time // Time of the last full scan
}

var _ Informer = (*resourceInformer)(nil)

// NewInformer creates a new ResourceInformer
func NewInformer(opts ...OptionFn) (*resourceInformer, error) {
	opt := defaultOptions()
	for _, fn := range opts {
		fn(opt)
	}

	if opt.procReader == nil && opt.procFSPath != "" {
		if pi, err := NewProcFSReader(opt.procFSPath); err != nil {
			return nil, fmt.Errorf("failed to create procfs reader: %w", err)
		} else {
			opt.procReader = pi
		}
	}

	if opt.procReader == nil {
		return nil, errors.New("no procfs reader specified")
	}

	return &resourceInformer{
		logger: opt.logger.With("service", "resource-informer"),
		fs:     opt.procReader,
		clock:  opt.clock,

		procCache: make(map[int]*Process),
		processes: &Processes{
			Running:    make(map[int]*Process),
			Terminated: make(map[int]*Process),
		},

		containerCache: make(map[string]*Container),
		containers: &Containers{
			Running:    make(map[string]*Container),
			Terminated: make(map[string]*Container),
		},

		vmCache: make(map[string]*VirtualMachine),
		vms: &VirtualMachines{
			Running:    make(map[string]*VirtualMachine),
			Terminated: make(map[string]*VirtualMachine),
		},
	}, nil
}

func (ri *resourceInformer) Name() string {
	return "resource-informer"
}

func (ri *resourceInformer) Init() error {
	// ensure we can access procfs
	_, err := ri.fs.AllProcs()
	if err != nil {
		return fmt.Errorf("failed to access procfs: %w", err)
	}

	ri.logger.Info("Resource informer initialized successfully")
	return nil
}

func (ri *resourceInformer) Refresh() error {
	started := ri.clock.Now()

	procs, err := ri.fs.AllProcs()
	if err != nil {
		return fmt.Errorf("failed to get processes: %w", err)
	}

	// construct current running processes and containers
	procsRunning := make(map[int]*Process, len(procs))
	containersRunning := make(map[string]*Container)
	vmsRunning := make(map[string]*VirtualMachine)

	// Refresh process cache and update running processes
	var refreshErrs error
	for _, p := range procs {
		pid := p.PID()
		// start by updating the process
		proc, err := ri.updateProcessCache(p)
		if err != nil {
			if os.IsNotExist(err) {
				ri.logger.Debug("Process not found", "pid", pid)
				continue
			}

			ri.logger.Debug("Failed to get process info", "pid", pid, "error", err)
			refreshErrs = errors.Join(refreshErrs, err)
			continue
		}
		procsRunning[pid] = proc

		switch proc.Type {
		case ContainerProcess:
			c := proc.Container
			_, seen := containersRunning[c.ID]
			// reset CPU Time of the container if it is getting added to the running list for the first time
			// in the subsequent iteration, the CPUTimeDelta should be incremented by process's CPUTimeDelta
			resetCPUTime := !seen
			containersRunning[c.ID] = ri.updateContainerCache(proc, resetCPUTime)

		case VMProcess:
			vm := proc.VirtualMachine
			vmsRunning[vm.ID] = ri.updateVMCache(proc)
		}

	}

	// Find terminated processes
	nodeCPUDelta := float64(0)
	procsTerminated := make(map[int]*Process)
	for pid, proc := range ri.procCache {
		if _, isRunning := procsRunning[pid]; isRunning {
			nodeCPUDelta += proc.CPUTimeDelta
			continue
		}
		procsTerminated[pid] = proc
		delete(ri.procCache, pid)
	}
	// Update tracking structures
	ri.processes.NodeCPUTimeDelta = nodeCPUDelta
	ri.processes.Running = procsRunning
	ri.processes.Terminated = procsTerminated

	// Find terminated containers
	totalContainerDelta := float64(0)
	containersTerminated := make(map[string]*Container)
	for id, container := range ri.containerCache {
		if _, isRunning := containersRunning[id]; isRunning {
			totalContainerDelta += container.CPUTimeDelta
			continue
		}
		containersTerminated[id] = container
		delete(ri.containerCache, id)
	}
	ri.containers.NodeCPUTimeDelta = nodeCPUDelta
	ri.containers.Running = containersRunning
	ri.containers.Terminated = containersTerminated

	// Find terminated VMs
	vmsTerminated := make(map[string]*VirtualMachine)
	for id, vm := range ri.vmCache {
		if _, isRunning := vmsRunning[id]; isRunning {
			continue
		}
		vmsTerminated[id] = vm
		delete(ri.vmCache, id)
	}

	ri.vms.NodeCPUTimeDelta = nodeCPUDelta
	ri.vms.Running = vmsRunning
	ri.vms.Terminated = vmsTerminated

	now := ri.clock.Now()
	ri.lastScanTime = now
	duration := now.Sub(started)

	ri.logger.Debug("Resource information collected",
		"process.running", len(procsRunning),
		"process.terminated", len(procsTerminated),
		"container.running", len(containersRunning),
		"container.terminated", len(containersTerminated),
		"vm.running", len(vmsRunning),
		"vm.terminated", len(vmsTerminated),
		"duration", duration)

	return refreshErrs
}

func (ri *resourceInformer) Processes() *Processes {
	return ri.processes
}

func (ri *resourceInformer) Containers() *Containers {
	return ri.containers
}

func (ri *resourceInformer) VirtualMachines() *VirtualMachines {
	return ri.vms
}

// Add VM cache update method
func (ri *resourceInformer) updateVMCache(proc *Process) *VirtualMachine {
	vm := proc.VirtualMachine
	if vm == nil {
		panic(fmt.Sprintf("process %d of type %s (%s)  has is nil virtual machine", proc.PID, proc.Type, proc.Comm))
	}

	cached, exists := ri.vmCache[vm.ID]
	if !exists {
		cached = vm.Clone()
		ri.vmCache[vm.ID] = cached
	}

	cached.CPUTimeDelta = proc.CPUTimeDelta
	cached.CPUTotalTime = proc.CPUTotalTime

	return cached
}

// updateProcessCache updates the process cache with the latest information and returns the updated process
func (ri *resourceInformer) updateProcessCache(proc procInfo) (*Process, error) {
	pid := proc.PID()

	if cached, exists := ri.procCache[pid]; exists {
		err := populateProcessFields(cached, proc)
		return cached, err
	}

	newProc, err := newProcess(proc)
	if err != nil {
		return nil, err
	}

	ri.procCache[pid] = newProc
	return newProc, nil
}

func (ri *resourceInformer) updateContainerCache(proc *Process, resetCPUTime bool) *Container {
	c := proc.Container
	if c == nil {
		return nil
	}

	cached, exists := ri.containerCache[c.ID]
	if !exists {
		cached = c.Clone()
		ri.containerCache[c.ID] = cached
	}

	if resetCPUTime {
		cached.CPUTimeDelta = 0
	}

	cached.CPUTimeDelta += proc.CPUTimeDelta
	cached.CPUTotalTime += proc.CPUTimeDelta

	return cached
}

// Update populateProcessFields to handle process types
func populateProcessFields(p *Process, proc procInfo) error {
	cpuTotalTime, err := proc.CPUTime()
	if err != nil {
		return err
	}

	p.CPUTimeDelta = cpuTotalTime - p.CPUTotalTime
	p.CPUTotalTime = cpuTotalTime

	// ignore already processed processes with close to 0 CPU time usage
	if newProc := p.Comm == ""; !newProc && p.CPUTimeDelta <= 1e-12 {
		return nil
	}

	comm, err := proc.Comm()
	if err != nil {
		return fmt.Errorf("failed to get process comm: %w", err)
	}
	commChanged := comm != p.Comm
	p.Comm = comm

	exe, err := proc.Executable()
	if err != nil {
		return fmt.Errorf("failed to get process executable: %w", err)
	}
	p.Exe = exe

	// Determine process type and associated container/VM only if not already set
	if p.Type == UnknownProcess || commChanged {
		info, err := computeTypeInfoFromProc(proc)
		if err != nil {
			return fmt.Errorf("failed to detect process type: %w", err)
		}

		p.Type = info.Type
		p.Container = info.Container
		p.VirtualMachine = info.VM
	}

	return nil
}

type ProcessTypeInfo struct {
	Type      ProcessType
	Container *Container
	VM        *VirtualMachine
}

func computeTypeInfoFromProc(proc procInfo) (*ProcessTypeInfo, error) {
	// detect process type in parallel
	type result struct {
		container *Container
		vm        *VirtualMachine
		err       error
	}

	// Using buffered channels to prevent goroutine from blocking
	containerCh := make(chan result, 1)
	vmCh := make(chan result, 1)

	go func() {
		defer close(containerCh)
		container, err := containerInfoFromProc(proc)
		containerCh <- result{container: container, err: err}
	}()

	go func() {
		defer close(vmCh)
		vm, err := vmInfoFromProc(proc)
		vmCh <- result{vm: vm, err: err}
	}()

	// Wait for both to complete
	ctnrResult := <-containerCh
	vmResult := <-vmCh

	switch {
	case ctnrResult.err == nil && ctnrResult.container != nil:
		return &ProcessTypeInfo{Type: ContainerProcess, Container: ctnrResult.container}, nil

	case vmResult.err == nil && vmResult.vm != nil:
		return &ProcessTypeInfo{Type: VMProcess, VM: vmResult.vm}, nil

	case ctnrResult.err == nil && vmResult.err == nil:
		return &ProcessTypeInfo{Type: RegularProcess}, errors.Join(ctnrResult.err, vmResult.err)

	default:
		return nil, errors.Join(ctnrResult.err, vmResult.err)
	}
}

// newProcess creates a new Process with static information filled in
func newProcess(proc procInfo) (*Process, error) {
	p := &Process{
		PID: proc.PID(),
	}

	if err := populateProcessFields(p, proc); err != nil {
		return nil, err
	}

	return p, nil
}
