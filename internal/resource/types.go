// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package resource

type Process struct {
	// static
	PID  int
	Comm string
	Exe  string

	Container *Container

	// Dynamic
	CPUTotalTime float64 // total cpu time used by the process
	CPUTimeDelta float64 // cpu time used by the process since last refresh
}

// Container represents metadata about a container
type Container struct {
	ID      string
	Name    string
	Runtime ContainerRuntime

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
