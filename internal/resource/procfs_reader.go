// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"fmt"

	"github.com/prometheus/procfs"
)

// cGroup holds only required cgroup info about the process
type cGroup struct {
	Path string // used to detect if a process is running in a container
}

// procInfo is an interface that wraps the necessary methods from procfs.Proc to be used by the resource service
type procInfo interface {
	PID() int
	Comm() (string, error)
	Executable() (string, error)
	Cgroups() ([]cGroup, error)
	Environ() ([]string, error)
	CmdLine() ([]string, error)
	CPUTime() (float64, error)
}

// procWrapper implements ProcInfo by wrapping procfs.Proc. This is needed because the procfs.Proc
// does not implement PID() as a method
type procWrapper struct {
	proc procfs.Proc
}

var _ procInfo = (*procWrapper)(nil)

func (p *procWrapper) PID() int {
	return p.proc.PID
}

func (p *procWrapper) Comm() (string, error) {
	return p.proc.Comm()
}

func (p *procWrapper) Executable() (string, error) {
	return p.proc.Executable()
}

func (p *procWrapper) Cgroups() ([]cGroup, error) {
	cgroupsData, err := p.proc.Cgroups()
	if err != nil {
		return nil, fmt.Errorf("failed to get process cgroups: %w", err)
	}

	cgroups := make([]cGroup, len(cgroupsData))
	for i, cg := range cgroupsData {
		cgroups[i] = cGroup{
			Path: cg.Path,
		}
	}
	return cgroups, nil
}

func (p *procWrapper) Environ() ([]string, error) {
	return p.proc.Environ()
}

func (p *procWrapper) CmdLine() ([]string, error) {
	return p.proc.CmdLine()
}

// userHZ is the number of clock ticks per second
// hardcoded just like in procfs
const userHZ = 100

func (p *procWrapper) CPUTime() (float64, error) {
	st, err := p.proc.Stat()
	if err != nil {
		return 0, err
	}

	return float64(st.STime+st.UTime) / userHZ, nil
}

// WrapProc wraps a procfs.Proc in a ProcInfo interface
func WrapProc(proc procfs.Proc) procInfo {
	return &procWrapper{proc: proc}
}

// Update the allProcReader interface to return our wrapped interface
type allProcReader interface {
	// AllProcs returns a list of all running processes
	AllProcs() ([]procInfo, error)

	// CPUUsageRatio returns the CPU usage ratio
	CPUUsageRatio() (float64, error)
}

// procFSReader is the default implementation of ProcReader using procfs
type procFSReader struct {
	fs       procfs.FS
	prevStat procfs.CPUStat
}

// CPUUsageRatio returns the CPU usage ratio as
// active over total, where active = total - (idle + iowait)
// and total = user + nice + system + idle + iowait + irq + softirq + steal
func (r *procFSReader) CPUUsageRatio() (float64, error) {
	current, err := r.fs.Stat()
	if err != nil {
		return 0, err
	}

	prev := r.prevStat
	r.prevStat = current.CPUTotal

	// first time, so return 0 usage ratio
	if prev == (procfs.CPUStat{}) {
		return 0, nil
	}

	curr := current.CPUTotal

	// find delta for all components
	dUser := curr.User - prev.User
	dNice := curr.Nice - prev.Nice
	dSystem := curr.System - prev.System
	dIdle := curr.Idle - prev.Idle
	dIowait := curr.Iowait - prev.Iowait
	dIRQ := curr.IRQ - prev.IRQ
	dSoftIRQ := curr.SoftIRQ - prev.SoftIRQ
	dSteal := curr.Steal - prev.Steal

	total := dUser + dNice + dSystem + dIdle + dIowait + dIRQ + dSoftIRQ + dSteal
	if total == 0 {
		return 0, nil
	}

	active := total - (dIdle + dIowait)
	ratio := active / total
	return ratio, nil
}

// AllProcs returns a list of all running processes
func (r *procFSReader) AllProcs() ([]procInfo, error) {
	procs, err := r.fs.AllProcs()
	if err != nil {
		return nil, err
	}

	ret := make([]procInfo, len(procs))
	for i, proc := range procs {
		ret[i] = WrapProc(proc)
	}
	return ret, nil
}

// NewProcFSReader creates a new ProcReader that reads from the specified procfs path
func NewProcFSReader(procfsPath string) (*procFSReader, error) {
	fs, err := procfs.NewFS(procfsPath)
	if err != nil {
		return nil, err
	}
	return &procFSReader{fs: fs}, nil
}
