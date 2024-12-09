package types

import (
	"fmt"
	"math"
	"sync/atomic"

	"k8s.io/klog/v2"
)

type UInt64Stat struct {
	aggr  atomic.Uint64
	delta atomic.Uint64
}

func NewUInt64Stat(aggr, delta uint64) *UInt64Stat {
	stat := UInt64Stat{}
	stat.aggr.Store(aggr)
	stat.delta.Store(delta)
	return &stat
}

func (s *UInt64Stat) String() string {
	return fmt.Sprintf("%d (%d)", s.delta.Load(), s.aggr.Load())
}

// ResetDelta resets current value
func (s *UInt64Stat) ResetDeltaValues() {
	s.delta.Swap(uint64(0))
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
	if s.aggr.Load() >= (math.MaxUint64 - newDelta) {
		// we must set the value to 0 when overflow, so that prometheus will handle it
		s.aggr.Swap(0)
		return fmt.Errorf("the aggregated value has overflowed")
	}
	if sum {
		// sum is used to accumulate metrics from different processes
		s.delta.Add(newDelta)
	} else {
		s.delta.Swap(newDelta)
	}
	// update aggregated value
	s.aggr.Add(newDelta)
	return nil
}

// SetNewAggr set new read aggregated value (e.g., from cgroup, energy files)
func (s *UInt64Stat) SetNewAggr(newAggr uint64) error {
	currAggr := s.aggr.Load()
	if newAggr == 0 || newAggr == currAggr {
		// if a counter has not changed, we skip it
		return nil
	}
	// verify aggregated value overflow
	if newAggr == math.MaxUint64 {
		// we must set the value to 0 when overflow, so that prometheus will handle it
		s.aggr.Swap(0)
		return fmt.Errorf("the aggregated value has overflowed")
	}
	if (currAggr > 0) && (newAggr > currAggr) {
		s.delta.Swap(newAggr - currAggr)
	}
	s.aggr.Swap(newAggr)
	return nil
}

func (s *UInt64Stat) GetDelta() uint64 {
	return s.delta.Load()
}

func (s *UInt64Stat) GetAggr() uint64 {
	return s.aggr.Load()
}

func NewUInt64StatCollection() UInt64StatCollection {
	return make(map[string]*UInt64Stat)
}

type UInt64StatCollection map[string]*UInt64Stat

func (s UInt64StatCollection) SetAggrStat(key string, newAggr uint64) {
	if instance, found := s[key]; !found {
		s[key] = NewUInt64Stat(newAggr, 0)
	} else {
		if err := instance.SetNewAggr(newAggr); err != nil {
			klog.V(3).Infoln(err)
		}
	}
}

func (s UInt64StatCollection) AddDeltaStat(key string, newDelta uint64) {
	if instance, found := s[key]; !found {
		s[key] = NewUInt64Stat(newDelta, newDelta)
	} else {
		if err := instance.AddNewDelta(newDelta); err != nil {
			klog.V(3).Infoln(err)
		}
	}
}
func (s UInt64StatCollection) SetDeltaStat(key string, newDelta uint64) {
	if instance, found := s[key]; !found {
		s[key] = NewUInt64Stat(newDelta, newDelta)
	} else {
		if err := instance.SetNewDelta(newDelta); err != nil {
			klog.V(3).Infoln(err)
		}
	}
}

// SumAllDeltaValues aggregates the delta metrics of all sources (i.e., stat keys)
func (s UInt64StatCollection) SumAllDeltaValues() uint64 {
	sum := uint64(0)
	for _, stat := range s {
		sum += stat.GetDelta()
	}
	return sum
}

// SumAllAggrValues aggregates the aggregated metrics of all sources (i.e., stat keys)
func (s UInt64StatCollection) SumAllAggrValues() uint64 {
	sum := uint64(0)
	for _, stat := range s {
		sum += stat.GetAggr()
	}
	return sum
}

func (s UInt64StatCollection) ResetDeltaValues() {
	for _, stat := range s {
		stat.ResetDeltaValues()
	}
}

func (s UInt64StatCollection) String() string {
	return fmt.Sprintf("%d (%d)", s.SumAllDeltaValues(), s.SumAllAggrValues())
}
