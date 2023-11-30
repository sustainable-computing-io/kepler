/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package stats

import (
	"fmt"
)

var (
	// ProcessMetricNames holds the list of names of the container metric
	ProcessMetricNames []string
	// ProcessFeaturesNames holds all the feature name of the container stats. This is specific for the machine-learning based models.
	ProcessFeaturesNames []string
)

type ProcessStats struct {
	Stats
	PID         uint64
	CGroupID    uint64
	ContainerID string
	VMID        string
	Command     string
	IdleCounter int
}

// NewProcessStats creates a new ProcessStats instance
func NewProcessStats(pid, cGroupID uint64, containerID, vmID, command string) *ProcessStats {
	p := &ProcessStats{
		PID:         pid,
		CGroupID:    cGroupID,
		ContainerID: containerID,
		VMID:        vmID,
		Command:     command,
		Stats:       *NewStats(),
	}
	return p
}

// ResetDeltaValues reset all delta values to 0
func (p *ProcessStats) ResetDeltaValues() {
	p.Stats.ResetDeltaValues()
	// if the metrics are not updated, this counter will increment, otherwise it will be set to 0
	p.IdleCounter += 1
}

func (p *ProcessStats) String() string {
	return fmt.Sprintf("energy from process pid: %d, containerID: %s, comm: %s\n"+
		"%v\n", p.PID, p.ContainerID, p.Command, p.Stats.String(),
	)
}
