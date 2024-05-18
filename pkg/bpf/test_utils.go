package bpf

import (
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"k8s.io/apimachinery/pkg/util/sets"
)

type mockExporter struct {
	softwareCounters sets.Set[string]
	hardwareCounters sets.Set[string]
}

func DefaultSupportedMetrics() SupportedMetrics {
	return SupportedMetrics{
		HardwareCounters: defaultHardwareCounters(),
		SoftwareCounters: defaultSoftwareCounters(),
	}
}

func defaultHardwareCounters() sets.Set[string] {
	return sets.New(config.CPUCycle, config.CPUInstruction, config.CacheMiss, config.TaskClock)
}

func defaultSoftwareCounters() sets.Set[string] {
	swCounters := sets.New(config.CPUTime, config.PageCacheHit)
	if config.ExposeIRQCounterMetrics {
		swCounters.Insert(config.IRQNetTXLabel, config.IRQNetRXLabel, config.IRQBlockLabel)
	}
	return swCounters
}

func NewMockExporter(bpfSupportedMetrics SupportedMetrics) Exporter {
	return &mockExporter{
		softwareCounters: bpfSupportedMetrics.SoftwareCounters.Clone(),
		hardwareCounters: bpfSupportedMetrics.HardwareCounters.Clone(),
	}
}

func (m *mockExporter) SupportedMetrics() SupportedMetrics {
	return SupportedMetrics{
		HardwareCounters: m.hardwareCounters,
		SoftwareCounters: m.softwareCounters,
	}
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
