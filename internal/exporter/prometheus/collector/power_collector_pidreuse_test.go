// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sustainable-computing-io/kepler/config"
	"github.com/sustainable-computing-io/kepler/internal/device"
	"github.com/sustainable-computing-io/kepler/internal/monitor"
	"github.com/sustainable-computing-io/kepler/internal/resource"
)

func TestProcessCPUTimePIDReuse(t *testing.T) {
	for _, tt := range []struct {
		name                  string
		change                func(*monitor.Process)
		noRunning             bool
		replacementTerminated bool
	}{
		{name: "unchanged metadata"},
		{name: "different comm", change: func(p *monitor.Process) { p.Comm = "new-process" }},
		{name: "different executable", change: func(p *monitor.Process) { p.Exe = "/usr/bin/new-process" }},
		{name: "different type", change: func(p *monitor.Process) { p.Type = resource.ContainerProcess }},
		{name: "different container", change: func(p *monitor.Process) { p.ContainerID = "new-container" }},
		{name: "different VM", change: func(p *monitor.Process) { p.VirtualMachineID = "new-vm" }},
		{name: "terminated only", noRunning: true},
		{name: "both terminated with identical labels", noRunning: true, replacementTerminated: true},
		{
			name: "both terminated with different labels", noRunning: true, replacementTerminated: true,
			change: func(p *monitor.Process) { p.Exe = "/usr/bin/new-process" },
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			zone := device.NewMockRaplZone("package", 0, "/test/rapl", 1000*device.Joule)
			old := &monitor.Process{
				PID: 4001, StartTime: 100, Comm: "python3", Exe: "/usr/bin/python3",
				Type: resource.RegularProcess, CPUTotalTime: 20,
				Zones: monitor.ZoneUsageMap{zone: {EnergyTotal: 50 * device.Joule, Power: 5 * device.Watt}},
			}
			running := old.Clone()
			running.StartTime = 900
			running.CPUTotalTime = 1
			running.Zones[zone] = monitor.Usage{EnergyTotal: 10 * device.Joule, Power: device.Watt}
			if tt.change != nil {
				tt.change(running)
			}
			snapshot := monitor.NewSnapshot()
			snapshot.TerminatedProcesses["4001:100"] = old
			if !tt.noRunning {
				snapshot.Processes["4001"] = running
			}
			if tt.replacementTerminated {
				snapshot.TerminatedProcesses["4001:900"] = running
			}
			mockMonitor := NewMockPowerMonitor()
			mockMonitor.On("Snapshot").Return(snapshot, nil)
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			collector := NewPowerCollector(mockMonitor, "test-node", logger, config.MetricsLevelProcess)
			registry := prometheus.NewPedanticRegistry()
			require.NoError(t, registry.Register(collector))
			mockMonitor.TriggerUpdate()
			require.Eventually(t, collector.isReady, time.Second, time.Millisecond)

			families, err := registry.Gather()
			require.NoError(t, err)
			var cpuTimes []float64
			var terminatedEnergy, terminatedPower []float64
			for _, family := range families {
				for _, metric := range family.GetMetric() {
					assert.Equal(t, "4001", valueOfLabel(metric, "pid"))
					switch family.GetName() {
					case "kepler_process_cpu_seconds_total":
						cpuTimes = append(cpuTimes, metric.GetCounter().GetValue())
					case "kepler_process_cpu_joules_total":
						if valueOfLabel(metric, "state") == "terminated" {
							terminatedEnergy = append(terminatedEnergy, metric.GetCounter().GetValue())
						}
					case "kepler_process_cpu_watts":
						if valueOfLabel(metric, "state") == "terminated" {
							terminatedPower = append(terminatedPower, metric.GetGauge().GetValue())
						}
					}
				}
			}
			expectedTimes := []float64{1}
			if tt.noRunning && !tt.replacementTerminated {
				expectedTimes = []float64{20}
			} else if tt.change != nil {
				expectedTimes = append(expectedTimes, 20)
			}
			assert.ElementsMatch(t, expectedTimes, cpuTimes)
			expectedEnergy, expectedPower := []float64{50}, []float64{5}
			if tt.replacementTerminated {
				if tt.change == nil {
					expectedEnergy, expectedPower = []float64{60}, []float64{1}
				} else {
					expectedEnergy, expectedPower = []float64{50, 10}, []float64{5, 1}
				}
			}
			assert.ElementsMatch(t, expectedEnergy, terminatedEnergy)
			assert.ElementsMatch(t, expectedPower, terminatedPower)
			if !tt.noRunning {
				assertMetricLabelValues(t, registry, "kepler_process_cpu_joules_total",
					map[string]string{"pid": "4001", "state": "running"}, 10)
				assertMetricLabelValues(t, registry, "kepler_process_cpu_watts",
					map[string]string{"pid": "4001", "state": "running"}, 1)
			}
			mockMonitor.AssertExpectations(t)
		})
	}
}
