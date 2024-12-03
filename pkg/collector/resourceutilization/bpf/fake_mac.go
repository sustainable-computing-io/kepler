//go:build darwin
// +build darwin

package bpf

import (
	"github.com/sustainable-computing-io/kepler/pkg/bpf"
	"github.com/sustainable-computing-io/kepler/pkg/collector/stats"
)

func UpdateProcessBPFMetrics(bpfExporter bpf.Exporter, processStats map[uint64]*stats.ProcessStats) {

}
