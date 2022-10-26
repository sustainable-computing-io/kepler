package metric

import (
	"fmt"
	"math"

	"k8s.io/klog/v2"

	"github.com/sustainable-computing-io/kepler/pkg/attacher"
	"github.com/sustainable-computing-io/kepler/pkg/cgroup"
	"github.com/sustainable-computing-io/kepler/pkg/config"
)

const (
	freqMetricLabel = config.CPUFrequency

	// TO-DO: merge to cgroup stat
	ByteReadLabel    = config.BytesReadIO
	ByteWriteLabel   = config.BytesWriteIO
	blockDeviceLabel = config.BlockDevicesIO

	CPUTimeLabel = config.CPUTime

	CurrPrefix = "curr_"
	AggrPrefix = "total_"
)

var (
	// AvailableCounters holds a list of hardware counters that might be collected
	AvailableCounters []string = attacher.GetEnabledCounters()
	// AvailableCgroupMetrics holds a list of cgroup metrics exposed by the cgroup that might be collected
	AvailableCgroupMetrics []string = cgroup.GetAvailableCgroupMetrics()
	// AvailableKubeletMetrics holds a list of cgrpup metrics exposed by kubelet that might be collected
	AvailableKubeletMetrics []string = cgroup.GetAvailableKubeletMetrics()

	// CPUHardwareCounterEnabled defined if hardware counters should be accounted and exported
	CPUHardwareCounterEnabled bool = false
	// CPUHardwareCounterEnabled = isCounterStatEnabled(attacher.CPUInstructionLabel)
)

type UInt64Stat struct {
	Curr     uint64
	Aggr     uint64
	PrevCurr uint64
}

func (s UInt64Stat) String() string {
	return fmt.Sprintf("%d (%d)", s.Curr, s.Aggr)
}

// ResetCurr resets current value and keep previous curr value for filling negative value
func (s *UInt64Stat) ResetCurr() {
	s.PrevCurr = s.Curr
	s.Curr = uint64(0)
}

// AddNewCurr adds new read current value (e.g., from bpf table that is reset, computed delta energy)
func (s *UInt64Stat) AddNewCurr(newCurr uint64) error {
	s.Curr += newCurr
	if math.MaxUint64-newCurr < s.Aggr {
		// overflow
		s.Aggr = s.Curr
		return fmt.Errorf("Aggr value overflow %d < %d, reset", s.Aggr+newCurr, s.Aggr)
	}
	s.Aggr += newCurr
	return nil
}

// SetNewAggr set new read aggregated value (e.g., from cgroup, energy files)
func (s *UInt64Stat) SetNewAggr(newAggr uint64) error {
	oldAggr := s.Aggr
	s.Aggr = newAggr
	if newAggr < oldAggr {
		// overflow: set to prev value
		s.Curr = s.PrevCurr
		return fmt.Errorf("Aggr value overflow %d < %d", newAggr, oldAggr)
	}
	if oldAggr == 0 {
		// new value
		s.Curr = 0
	} else {
		s.Curr = newAggr - oldAggr
	}
	return nil
}

// UInt64StatCollection keeps a collection of UInt64Stat
type UInt64StatCollection struct {
	Stat map[string]*UInt64Stat
}

func (s *UInt64StatCollection) AddAggrStat(key string, newAggr uint64) {
	if _, found := s.Stat[key]; !found {
		s.Stat[key] = &UInt64Stat{}
	}
	if err := s.Stat[key].SetNewAggr(newAggr); err != nil {
		klog.V(3).Infoln(err)
	}
}

func (s *UInt64StatCollection) AddCurrStat(key string, newCurr uint64) {
	if _, found := s.Stat[key]; !found {
		s.Stat[key] = &UInt64Stat{}
	}
	if err := s.Stat[key].AddNewCurr(newCurr); err != nil {
		klog.V(3).Infoln(err)
	}
}

func (s *UInt64StatCollection) Curr() uint64 {
	sum := uint64(0)
	for _, stat := range s.Stat {
		sum += stat.Curr
	}
	return sum
}

func (s *UInt64StatCollection) Aggr() uint64 {
	sum := uint64(0)
	for _, stat := range s.Stat {
		sum += stat.Aggr
	}
	return sum
}

func (s *UInt64StatCollection) ResetCurr() {
	for _, stat := range s.Stat {
		stat.ResetCurr()
	}
}

func (s UInt64StatCollection) String() string {
	return fmt.Sprintf("%d (%d)", s.Curr(), s.Aggr())
}
