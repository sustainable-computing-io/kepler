// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package resource

type ProcessType string

const (
	UnknownProcess   ProcessType = ""
	RegularProcess   ProcessType = "regular"
	ContainerProcess ProcessType = "container"
	VMProcess        ProcessType = "vm"
)

type Process struct {
	// static
	PID  int
	Comm string
	Exe  string
	Type ProcessType

	Container      *Container
	VirtualMachine *VirtualMachine

	// Dynamic
	CPUTotalTime float64 // total cpu time used by the process
	CPUTimeDelta float64 // cpu time used by the process since last refresh
}

// Container represents metadata about a container
type Container struct {
	ID      string
	Name    string
	Runtime ContainerRuntime

	Pod *Pod

	// Resource usage tracking
	CPUTotalTime float64 // total cpu time used by the container so far
	CPUTimeDelta float64 // cpu time used by the container since last refresh
}

type ContainerRuntime string

const (
	UnknownRuntime    ContainerRuntime = "unknown"
	DockerRuntime     ContainerRuntime = "docker"
	ContainerDRuntime ContainerRuntime = "containerd"
	CrioRuntime       ContainerRuntime = "crio"
	PodmanRuntime     ContainerRuntime = "podman"
	KubePodsRuntime   ContainerRuntime = "kubernetes"
)

// Clone creates a deep copy of a Container
func (c *Container) Clone() *Container {
	if c == nil {
		return nil
	}

	clone := &Container{
		ID:      c.ID,
		Name:    c.Name,
		Runtime: c.Runtime,
	}

	return clone
}

// VirtualMachine represents metadata about a virtual machine
type VirtualMachine struct {
	ID         string
	Name       string
	Hypervisor Hypervisor

	// Resource usage tracking
	CPUTotalTime float64 // total cpu time used by the VM so far
	CPUTimeDelta float64 // cpu time used by the VM since last refresh
}

type Hypervisor string

const (
	UnknownHypervisor Hypervisor = "unknown"

	KVMHypervisor Hypervisor = "kvm"

	// TODO: add patterns for these hypervisors
	VirtualBoxHypervisor Hypervisor = "virtualbox"
	VMwareHypervisor     Hypervisor = "vmware"
	XenHypervisor        Hypervisor = "xen"
)

// Clone creates a deep copy of a VirtualMachine
func (vm *VirtualMachine) Clone() *VirtualMachine {
	if vm == nil {
		return nil
	}

	return &VirtualMachine{
		ID:         vm.ID,
		Name:       vm.Name,
		Hypervisor: vm.Hypervisor,
	}
}

type Pod struct {
	ID        string
	Name      string
	Namespace string

	// Resource usage tracking
	CPUTotalTime float64 // total cpu time used by the Pod so far
	CPUTimeDelta float64 // cpu time used by the Pod since last refresh
}

func (p *Pod) Clone() *Pod {
	if p == nil {
		return nil
	}
	return &Pod{
		ID:        p.ID,
		Name:      p.Name,
		Namespace: p.Namespace,
	}
}
