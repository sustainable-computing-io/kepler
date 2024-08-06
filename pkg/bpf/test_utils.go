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
	return sets.New(config.CPUCycle, config.CPUInstruction, config.CacheMiss)
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

func (m *mockExporter) Start(<-chan struct{}) error {
	return nil
}

func (m *mockExporter) SupportedMetrics() SupportedMetrics {
	return SupportedMetrics{
		HardwareCounters: m.hardwareCounters,
		SoftwareCounters: m.softwareCounters,
	}
}

func (m *mockExporter) Detach() {}

func (m *mockExporter) CollectProcesses() (ProcessMetricsCollection, error) {
	return ProcessMetricsCollection{
		Metrics: []ProcessMetrics{
			{
				CGroupID:        0,
				Pid:             0,
				ProcessRunTime:  0,
				CPUCyles:        0,
				CPUInstructions: 0,
				CacheMiss:       0,
				PageCacheHit:    0,
				NetTxIRQ:        0,
				NetRxIRQ:        0,
				NetBlockIRQ:     0,
			},
		},
		FreedPIDs: []int{0},
	}, nil
}
