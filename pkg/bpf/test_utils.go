package bpf

import "github.com/sustainable-computing-io/kepler/pkg/config"

type mockAttacher struct {
	hardwareCountersEnabled bool
}

func NewMockAttacher(hardwareCountersEnabled bool) Attacher {
	return &mockAttacher{
		hardwareCountersEnabled: hardwareCountersEnabled,
	}
}

func (m *mockAttacher) HardwareCountersEnabled() bool {
	return m.hardwareCountersEnabled
}

func (m *mockAttacher) Detach() {}

func (m *mockAttacher) CollectProcesses() ([]ProcessBPFMetrics, error) {
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

func (m *mockAttacher) CollectCPUFreq() (map[int32]uint64, error) {
	return map[int32]uint64{0: 0}, nil
}

func (m *mockAttacher) GetEnabledBPFHWCounters() []string {
	if !m.hardwareCountersEnabled {
		return []string{}
	}
	return []string{config.CPUCycle, config.CPUInstruction, config.CacheMiss, config.TaskClock}
}

func (m *mockAttacher) GetEnabledBPFSWCounters() []string {
	swCounters := []string{config.CPUTime, config.TaskClock, config.PageCacheHit}
	if config.ExposeIRQCounterMetrics {
		swCounters = append(swCounters, SoftIRQEvents...)
	}
	return swCounters
}
