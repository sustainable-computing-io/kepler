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
	return sets.New(config.BPFHwCounters()...)
}

func defaultSoftwareCounters() sets.Set[string] {
	swCounters := sets.New(config.BPFSwCounters()...)
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

func (m *mockExporter) CollectProcesses() ([]ProcessMetrics, error) {
	return []ProcessMetrics{
		{
			CgroupId:       0,
			Pid:            0,
			ProcessRunTime: 0,
			CpuCycles:      0,
			CpuInstr:       0,
			CacheMiss:      0,
			PageCacheHit:   0,
			VecNr:          [10]uint16{},
			Comm:           [16]int8{},
		},
	}, nil
}
