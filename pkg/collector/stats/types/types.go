package types

import (
	"fmt"
	"math"

	"k8s.io/klog/v2"
)

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

// AddNewDelta add a new read (process) delta value (e.g., from bpf table that is reset, computed delta energy)
func (s *UInt64Stat) AddNewDelta(newDelta uint64) error {
	return s.SetNewDeltaValue(newDelta, true)
}

// SetNewDelta replace the delta value with a new (node) read delta value (e.g., from bpf table that is reset, computed delta energy)
func (s *UInt64Stat) SetNewDelta(newDelta uint64) error {
	return s.SetNewDeltaValue(newDelta, false)
}

// SetNewDeltaValue sum or replace the delta value with a new read delta value
func (s *UInt64Stat) SetNewDeltaValue(newDelta uint64, sum bool) error {
	if newDelta == 0 {
		// if a counter has overflowed we skip it
		return nil
	}
	// verify overflow
	if s.Aggr >= (math.MaxUint64 - newDelta) {
		// we must set the the value to 0 when overflow, so that prometheus will handle it
		s.Aggr = 0
		return fmt.Errorf("the aggregated value has overflowed")
	}
	if sum {
		// sum is used to accumulate metrics from different processes
		s.Delta += newDelta
	} else {
		s.Delta = newDelta
	}
	s.Aggr += newDelta
	return nil
}

// SetNewAggr set new read aggregated value (e.g., from cgroup, energy files)
func (s *UInt64Stat) SetNewAggr(newAggr uint64) error {
	if newAggr == 0 || newAggr == s.Aggr {
		// if a counter has not changed, we skip it
		return nil
	}
	// verify aggregated value overflow
	if newAggr == math.MaxUint64 {
		// we must set the the value to 0 when overflow, so that prometheus will handle it
		s.Aggr = 0
		return fmt.Errorf("the aggregated value has overflowed")
	}
	if (s.Aggr > 0) && (newAggr > s.Aggr) {
		s.Delta = newAggr - s.Aggr
	}
	s.Aggr = newAggr
	return nil
}

func (s *UInt64Stat) GetDelta() uint64 {
	return s.Delta
}

func (s *UInt64Stat) GetAggr() uint64 {
	return s.Aggr
}

func NewUInt64StatCollection() *UInt64StatCollection {
	return &UInt64StatCollection{
		Stat: make(map[string]*UInt64Stat),
	}
}

// UInt64StatCollection keeps a collection of UInt64Stat
type UInt64StatCollection struct {
	Stat map[string]*UInt64Stat
}

func (s *UInt64StatCollection) SetAggrStat(key string, newAggr uint64) {
	if instance, found := s.Stat[key]; !found {
		s.Stat[key] = &UInt64Stat{
			Aggr:  newAggr,
			Delta: 0,
		}
	} else {
		if err := instance.SetNewAggr(newAggr); err != nil {
			klog.V(3).Infoln(err)
		}
	}
}

func (s *UInt64StatCollection) AddDeltaStat(key string, newDelta uint64) {
	if instance, found := s.Stat[key]; !found {
		s.Stat[key] = &UInt64Stat{
			Aggr:  newDelta,
			Delta: newDelta,
		}
	} else {
		if err := instance.AddNewDelta(newDelta); err != nil {
			klog.V(3).Infoln(err)
		}
	}
}
func (s *UInt64StatCollection) SetDeltaStat(key string, newDelta uint64) {
	if instance, found := s.Stat[key]; !found {
		s.Stat[key] = &UInt64Stat{
			Aggr:  newDelta,
			Delta: newDelta,
		}
	} else {
		if err := instance.SetNewDelta(newDelta); err != nil {
			klog.V(3).Infoln(err)
		}
	}
}

// SumAllDeltaValues aggregates the delta metrics of all sources (i.e., stat keys)
func (s *UInt64StatCollection) SumAllDeltaValues() uint64 {
	sum := uint64(0)
	if s == nil {
		klog.V(3).Info(" s  == nil")
		return sum
	}
	for _, stat := range s.Stat {
		if stat != nil {
			sum += stat.Delta
		} else {
			klog.V(3).Info(" Error retrieving stat")
		}
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
