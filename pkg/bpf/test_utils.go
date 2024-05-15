package bpf

import "github.com/sustainable-computing-io/kepler/pkg/config"

type mockExporter struct {
	hardwareCountersEnabled bool
}

func NewMockExporter(hardwareCountersEnabled bool) Exporter {
	return &mockExporter{
		hardwareCountersEnabled: hardwareCountersEnabled,
	}
}

func (m *mockExporter) HardwareCountersEnabled() bool {
	return m.hardwareCountersEnabled
}

func (m *mockExporter) Detach() {}

func (m *mockExporter) CollectProcesses() ([]ProcessBPFMetrics, error) {
	return []ProcessBPFMetrics{
		{
			CGroupID:       0,
			ThreadPID:      0,
			PID:            0,
			ProcessRunTime: 0,
			TaskClockTime:  0,
			CPUCycles:      0,
			CPUInstr:       0,
			CacheMisses:    0,
			PageCacheHit:   0,
			VecNR:          [10]uint16{},
			Command:        [16]byte{},
		},
	}, nil
}

func (m *mockExporter) CollectCPUFreq() (map[int32]uint64, error) {
	return map[int32]uint64{0: 0}, nil
}

func (m *mockExporter) GetEnabledBPFHWCounters() []string {
	if !m.hardwareCountersEnabled {
		return []string{}
	}
	return []string{config.CPUCycle, config.CPUInstruction, config.CacheMiss, config.TaskClock}
}

func (m *mockExporter) GetEnabledBPFSWCounters() []string {
	swCounters := []string{config.CPUTime, config.TaskClock, config.PageCacheHit}
	if config.ExposeIRQCounterMetrics {
		swCounters = append(swCounters, SoftIRQEvents...)
	}
	return swCounters
}
