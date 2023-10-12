package metric

import (
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
	// AvailableBPFSWCounters holds a list of eBPF counters that might be collected
	AvailableBPFSWCounters []string
	// AvailableBPFHWCounters holds a list of hardware counters that might be collected
	AvailableBPFHWCounters []string
	// AvailableCGroupMetrics holds a list of cgroup metrics exposed by the cgroup that might be collected
	AvailableCGroupMetrics []string
	// AvailableKubeletMetrics holds a list of cgrpup metrics exposed by kubelet that might be collected
	AvailableKubeletMetrics []string
	// AvailableContainerKubeletMetrics holds a list of cgrpup metrics exposed by kubelet specific to container
	AvailableContainerKubeletMetrics []string
	// AvailableNodeKubeletMetrics holds a list of cgroup metrics exposed by kubelet specific to node
	AvailableNodeKubeletMetrics []string
	// CPUHardwareCounterEnabled defined if hardware counters should be accounted and exported
	CPUHardwareCounterEnabled = false
)

func InitAvailableParamAndMetrics() {
	AvailableBPFHWCounters = attacher.GetEnabledBPFHWCounters()
	AvailableBPFSWCounters = attacher.GetEnabledBPFSWCounters()
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
