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

func (m *mockExporter) Start(results chan<- []*ProcessBPFMetrics, stop <-chan struct{}) {}

func (m *mockExporter) CollectCPUFreq() (map[int32]uint64, error) {
	return map[int32]uint64{0: 0}, nil
}
