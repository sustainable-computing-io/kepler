// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package fakeworkload

import (
	"path/filepath"
	"strings"

	"github.com/prometheus/procfs"
	"github.com/sustainable-computing-io/kepler/internal/device/gpu/nvidia"
)

var _ nvidia.PIDSource = (*stressNGGPUWorkload)(nil)

// stressNGGPUWorkload finds stress-ng worker PIDs whose cwd matches workloadDir.
// It reads host.procfs and ignores stress-ng processes in other directories.
type stressNGGPUWorkload struct {
	fs          procfs.FS
	workloadDir string
}

// New returns a PIDSource that reports stress-ng workers under procfsPath whose
// cwd equals workloadDir. An empty workloadDir yields no PIDs (idle).
func New(procfsPath, workloadDir string) (nvidia.PIDSource, error) {
	fs, err := procfs.NewFS(procfsPath)
	if err != nil {
		return nil, err
	}
	resolved := ""
	if workloadDir != "" {
		resolved, err = filepath.Abs(workloadDir)
		if err != nil {
			return nil, err
		}
		resolved, err = filepath.EvalSymlinks(resolved)
		if err != nil {
			return nil, err
		}
		resolved = filepath.Clean(resolved)
	}
	return &stressNGGPUWorkload{fs: fs, workloadDir: resolved}, nil
}

func (s *stressNGGPUWorkload) PIDs() []uint32 {
	if s == nil || s.workloadDir == "" {
		return nil
	}

	procs, err := s.fs.AllProcs()
	if err != nil {
		return nil
	}

	var pids []uint32
	for _, proc := range procs {
		comm, err := proc.Comm()
		if err != nil || !isStressNGWorker(comm) {
			continue
		}
		cwd, err := proc.Cwd()
		if err != nil {
			continue
		}
		if filepath.Clean(cwd) != s.workloadDir {
			continue
		}
		pids = append(pids, uint32(proc.PID))
	}
	return pids
}

func isStressNGWorker(comm string) bool {
	return strings.HasPrefix(strings.TrimSpace(comm), "stress-ng-")
}
