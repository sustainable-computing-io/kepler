// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/sustainable-computing-io/kepler/test/common"
)

func TestFindWorkloadPower_SumsWorkersNotParent(t *testing.T) {
	snapshot := &common.MetricsSnapshot{
		Families: map[string]*common.MetricFamily{
			"kepler_process_cpu_watts": {
				Name: "kepler_process_cpu_watts",
				Metrics: []common.Metric{
					{Name: "kepler_process_cpu_watts", Value: 0.0068, Labels: map[string]string{"pid": "100", "state": "running"}},
					{Name: "kepler_process_cpu_watts", Value: 1.2, Labels: map[string]string{"pid": "101", "state": "running"}},
					{Name: "kepler_process_cpu_watts", Value: 3.4, Labels: map[string]string{"pid": "102", "state": "running"}},
					{Name: "kepler_process_cpu_watts", Value: 9.9, Labels: map[string]string{"pid": "102", "state": "terminated"}},
				},
			},
		},
	}

	tests := []struct {
		name      string
		workers   []int
		wantPower float64
		wantFound bool
	}{
		{
			name:      "unequal workers ignore parent",
			workers:   []int{101, 102},
			wantPower: 4.6,
			wantFound: true,
		},
		{
			name:      "no workers",
			workers:   nil,
			wantPower: 0,
			wantFound: false,
		},
		{
			name:      "parent pid is not a worker",
			workers:   []int{101},
			wantPower: 1.2,
			wantFound: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := &Workload{workerPIDs: tc.workers}
			power, found := FindWorkloadPower(snapshot, w)
			assert.Equal(t, tc.wantFound, found)
			assert.InDelta(t, tc.wantPower, power, 1e-9)
		})
	}
}
