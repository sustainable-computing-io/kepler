package metric

import (
	"fmt"
	"math"

	"k8s.io/klog/v2"

	"github.com/sustainable-computing-io/kepler/pkg/bpfassets/attacher"
	"github.com/sustainable-computing-io/kepler/pkg/cgroup"
	"github.com/sustainable-computing-io/kepler/pkg/config"
)

const (
	// TO-DO: merge to cgroup stat
	ByteReadLabel    = config.BytesReadIO
	ByteWriteLabel   = config.BytesWriteIO
	blockDeviceLabel = config.BlockDevicesIO

	DeltaPrefix = "curr_"
	AggrPrefix  = "total_"
)

var (
	// AvailableEBPFCounters holds a list of eBPF counters that might be collected
	AvailableEBPFCounters []string
	// AvailableHWCounters holds a list of hardware counters that might be collected
	AvailableHWCounters []string
	// AvailableCGroupMetrics holds a list of cgroup metrics exposed by the cgroup that might be collected
	AvailableCGroupMetrics []string
	// AvailableKubeletMetrics holds a list of cgrpup metrics exposed by kubelet that might be collected
	AvailableKubeletMetrics []string

	// CPUHardwareCounterEnabled defined if hardware counters should be accounted and exported
	CPUHardwareCounterEnabled = false
)

func InitAvailableParamAndMetrics() {
	AvailableHWCounters = attacher.GetEnabledHWCounters()
	AvailableEBPFCounters = attacher.GetEnabledBPFCounters()
	AvailableCGroupMetrics = []string{
		config.CgroupfsMemory, config.CgroupfsKernelMemory, config.CgroupfsTCPMemory,
		config.CgroupfsCPU, config.CgroupfsSystemCPU, config.CgroupfsUserCPU,
		config.CgroupfsReadIO, config.CgroupfsWriteIO, config.BlockDevicesIO,
	}
	AvailableKubeletMetrics = cgroup.GetAvailableKubeletMetrics()
	CPUHardwareCounterEnabled = isCounterStatEnabled(attacher.CPUInstructionLabel)

	// defined in utils to init metrics
	setEnabledMetrics()
}

type UInt64Stat struct {
	Aggr  uint64
	Delta uint64
}

func (s UInt64Stat) String() string {
	return fmt.Sprintf("%d (%d)", s.Delta, s.Aggr)
}

// ResetDelta resets current value
func (s *UInt64Stat) ResetDeltaValues() {
	s.Delta = uint64(0)
}

// AddNewDelta sum a new read delta value (e.g., from bpf table that is reset, computed delta energy)
func (s *UInt64Stat) AddNewDelta(newDelta uint64) error {
	return s.SetNewDeltaValue(newDelta, true)
}

// SetNewDelta replace the delta value with a new read delta value (e.g., from bpf table that is reset, computed delta energy)
func (s *UInt64Stat) SetNewDelta(newDelta uint64) error {
	return s.SetNewDeltaValue(newDelta, false)
}

// SetNewDeltaValue sum or replace the delta value with a new read delta value
func (s *UInt64Stat) SetNewDeltaValue(newDelta uint64, sum bool) error {
	if newDelta == 0 {
		// if a counter has overflowed we skip it
		return nil
	}
	if sum {
		// sum is used to accumulate metrics from different processes
		s.Delta += newDelta
	} else {
		s.Delta = newDelta
	}
	s.Aggr += newDelta
	// verify overflow
	if s.Aggr == math.MaxUint64 {
		// we must set the the value to 0 when overflow, so that prometheus will handle it
		s.Aggr = 0
		return fmt.Errorf("the aggregated value has overflowed")
	}
	return nil
}

// SetNewAggr set new read aggregated value (e.g., from cgroup, energy files)
func (s *UInt64Stat) SetNewAggr(newAggr uint64) error {
	if newAggr == 0 {
		// if a counter has overflowed we skip it
		return nil
	}
	if newAggr == s.Aggr {
		// if a counter has not changed, we skip it
		return nil
	}
	// verify aggregated value overflow
	if newAggr == math.MaxUint64 {
		// we must set the the value to 0 when overflow, so that prometheus will handle it
		s.Aggr = 0
		return fmt.Errorf("the aggregated value has overflowed")
	}

	oldAggr := s.Aggr
	s.Aggr = newAggr

	if (oldAggr > 0) && (newAggr > oldAggr) {
		s.Delta = newAggr - oldAggr
	}
	return nil
}

// UInt64StatCollection keeps a collection of UInt64Stat
type UInt64StatCollection struct {
	Stat map[string]*UInt64Stat
}

func (s *UInt64StatCollection) SetAggrStat(key string, newAggr uint64) {
	if _, found := s.Stat[key]; !found {
		s.Stat[key] = &UInt64Stat{}
	}
	if err := s.Stat[key].SetNewAggr(newAggr); err != nil {
		klog.V(3).Infoln(err)
	}
}

func (s *UInt64StatCollection) AddDeltaStat(key string, newDelta uint64) {
	if _, found := s.Stat[key]; !found {
		s.Stat[key] = &UInt64Stat{}
	}
	if err := s.Stat[key].AddNewDelta(newDelta); err != nil {
		klog.V(3).Infoln(err)
	}
}
func (s *UInt64StatCollection) SetDeltaStat(key string, newDelta uint64) {
	if _, found := s.Stat[key]; !found {
		s.Stat[key] = &UInt64Stat{}
	}
	if err := s.Stat[key].SetNewDelta(newDelta); err != nil {
		klog.V(3).Infoln(err)
	}
}

// SumAllDeltaValues aggregates the delta metrics of all sources (i.e., stat keys)
func (s *UInt64StatCollection) SumAllDeltaValues() uint64 {
	sum := uint64(0)
	for _, stat := range s.Stat {
		sum += stat.Delta
	}
	return sum
}

// SumAllAggrValues aggregates the aggregated metrics of all sources (i.e., stat keys)
func (s *UInt64StatCollection) SumAllAggrValues() uint64 {
	sum := uint64(0)
	for _, stat := range s.Stat {
		sum += stat.Aggr
	}
	return sum
}

func (s *UInt64StatCollection) ResetDeltaValues() {
	for _, stat := range s.Stat {
		stat.ResetDeltaValues()
	}
}

func (s UInt64StatCollection) String() string {
	return fmt.Sprintf("%d (%d)", s.SumAllDeltaValues(), s.SumAllAggrValues())
}
