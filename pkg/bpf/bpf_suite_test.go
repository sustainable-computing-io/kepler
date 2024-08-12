package bpf

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/cilium/ebpf/rlimit"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gmeasure"
	"golang.org/x/sys/unix"
)

func TestBpf(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bpf Suite")
}

var _ = Describe("BPF Exporter", func() {
	It("should send a page cache hit event", func() {
		// Remove resource limits for kernels <5.11.
		err := rlimit.RemoveMemlock()
		Expect(err).NotTo(HaveOccurred())

		// Load eBPF Specs
		specs, err := loadKepler()
		Expect(err).NotTo(HaveOccurred())

		var obj keplerObjects
		// Load eBPF objects
		err = specs.LoadAndAssign(&obj, nil)
		Expect(err).NotTo(HaveOccurred())

		out, err := obj.TestKeplerWritePageTrace.Run(&ebpf.RunOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(Equal(uint32(0)))

		// Read the event from the ring buffer
		rd, err := ringbuf.NewReader(obj.Rb)
		Expect(err).NotTo(HaveOccurred())
		defer rd.Close()

		var event keplerEvent
		record, err := rd.Read()
		Expect(err).NotTo(HaveOccurred())

		err = binary.Read(bytes.NewBuffer(record.RawSample), binary.NativeEndian, &event)
		Expect(err).NotTo(HaveOccurred())
		Expect(event.Pid).To(Equal(uint32(42)))
		Expect(event.Ts).To(BeNumerically(">", uint64(0)))
		Expect(event.EventType).To(Equal(uint64(keplerEventTypePAGE_CACHE_HIT)))
	})

	It("should send a process free event", func() {
		// Remove resource limits for kernels <5.11.
		err := rlimit.RemoveMemlock()
		Expect(err).NotTo(HaveOccurred())

		// Load eBPF Specs
		specs, err := loadKepler()
		Expect(err).NotTo(HaveOccurred())

		var obj keplerObjects
		// Load eBPF objects
		err = specs.LoadAndAssign(&obj, nil)
		Expect(err).NotTo(HaveOccurred())

		out, err := obj.TestKeplerSchedProcessFree.Run(&ebpf.RunOptions{
			Flags: uint32(1), // BPF_F_TEST_RUN_ON_CPU
			CPU:   uint32(0),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(Equal(uint32(0)))

		// Read the event from the ring buffer
		rd, err := ringbuf.NewReader(obj.Rb)
		Expect(err).NotTo(HaveOccurred())
		defer rd.Close()

		var event keplerEvent
		record, err := rd.Read()
		Expect(err).NotTo(HaveOccurred())

		err = binary.Read(bytes.NewBuffer(record.RawSample), binary.NativeEndian, &event)
		Expect(err).NotTo(HaveOccurred())
		Expect(event.Pid).To(Equal(uint32(42)))
		Expect(event.Ts).To(BeNumerically(">", uint64(0)))
		Expect(event.EventType).To(Equal(uint64(keplerEventTypeFREE)))
	})

	It("should increment the page hit counter efficiently", func() {
		experiment := gmeasure.NewExperiment("Increment the page hit counter")
		AddReportEntry(experiment.Name, experiment)
		// Remove resource limits for kernels <5.11.
		err := rlimit.RemoveMemlock()
		Expect(err).NotTo(HaveOccurred())

		// Load eBPF Specs
		specs, err := loadKepler()
		Expect(err).NotTo(HaveOccurred())

		var obj keplerObjects
		// Load eBPF objects
		err = specs.LoadAndAssign(&obj, nil)
		Expect(err).NotTo(HaveOccurred())

		experiment.Sample(func(idx int) {
			experiment.MeasureDuration("page hit counter increment", func() {
				out, err := obj.TestKeplerWritePageTrace.Run(&ebpf.RunOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(out).To(Equal(uint32(0)))
			}, gmeasure.Precision(time.Nanosecond))
		}, gmeasure.SamplingConfig{N: 1000000, Duration: 10 * time.Second})
	})

	It("collects hardware counter metrics for sched_switch events", Label("perf_event"), func() {
		// Remove resource limits for kernels <5.11.
		err := rlimit.RemoveMemlock()
		Expect(err).NotTo(HaveOccurred())

		// Load eBPF Specs
		specs, err := loadKepler()
		Expect(err).NotTo(HaveOccurred())

		var obj keplerObjects
		// Load eBPF objects
		err = specs.LoadAndAssign(&obj, nil)
		Expect(err).NotTo(HaveOccurred())

		perfEvents, err := createTestHardwarePerfEvents(
			obj.CpuInstructionsEventReader,
			obj.CpuCyclesEventReader,
			obj.CacheMissEventReader,
		)
		Expect(err).NotTo(HaveOccurred())
		defer func() {
			for _, fd := range perfEvents {
				unix.Close(fd)
			}
		}()

		out, err := obj.TestKeplerSchedSwitchTrace.Run(&ebpf.RunOptions{
			Flags: uint32(1), // BPF_F_TEST_RUN_ON_CPU
			CPU:   uint32(0),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(Equal(uint32(0)))

		// Read the event from the ring buffer
		rd, err := ringbuf.NewReader(obj.Rb)
		Expect(err).NotTo(HaveOccurred())
		defer rd.Close()

		var event keplerEvent
		record := new(ringbuf.Record)

		err = rd.ReadInto(record)
		Expect(err).NotTo(HaveOccurred())

		err = binary.Read(bytes.NewBuffer(record.RawSample), binary.NativeEndian, &event)
		Expect(err).NotTo(HaveOccurred())
		Expect(event.Pid).To(Equal(uint32(43)))
		Expect(event.Tid).To(Equal(uint32(43)))
		Expect(event.Ts).To(BeNumerically(">", uint64(0)))
		Expect(event.EventType).To(Equal(uint64(keplerEventTypeSCHED_SWITCH)))
		Expect(event.CpuCycles).To(BeNumerically(">", uint64(0)))
		Expect(event.CpuInstr).To(BeNumerically(">", uint64(0)))
		Expect(event.CacheMiss).To(BeNumerically(">", uint64(0)))
		Expect(event.OffcpuPid).To(Equal(uint32(42)))
		Expect(event.OffcpuTid).To(Equal(uint32(42)))
	})

	It("collects metrics for sched_switch events when no hardware events are enabled", Label("perf_event"), func() {
		// Remove resource limits for kernels <5.11.
		err := rlimit.RemoveMemlock()
		Expect(err).NotTo(HaveOccurred())

		// Load eBPF Specs
		specs, err := loadKepler()
		Expect(err).NotTo(HaveOccurred())

		err = specs.RewriteConstants(map[string]interface{}{
			"HW": int32(-1),
		})
		Expect(err).NotTo(HaveOccurred())

		var obj keplerObjects
		// Load eBPF objects
		err = specs.LoadAndAssign(&obj, nil)
		Expect(err).NotTo(HaveOccurred())

		out, err := obj.TestKeplerSchedSwitchTrace.Run(&ebpf.RunOptions{
			Flags: uint32(1), // BPF_F_TEST_RUN_ON_CPU
			CPU:   uint32(0),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(Equal(uint32(0)))

		// Read the event from the ring buffer
		rd, err := ringbuf.NewReader(obj.Rb)
		Expect(err).NotTo(HaveOccurred())
		defer rd.Close()

		var event keplerEvent
		record := new(ringbuf.Record)

		err = rd.ReadInto(record)
		Expect(err).NotTo(HaveOccurred())

		err = binary.Read(bytes.NewBuffer(record.RawSample), binary.NativeEndian, &event)
		Expect(err).NotTo(HaveOccurred())
		Expect(event.Pid).To(Equal(uint32(43)))
		Expect(event.Tid).To(Equal(uint32(43)))
		Expect(event.Ts).To(BeNumerically(">", uint64(0)))
		Expect(event.EventType).To(Equal(uint64(keplerEventTypeSCHED_SWITCH)))
		Expect(event.CpuCycles).To(BeNumerically("==", uint64(0)))
		Expect(event.CpuInstr).To(BeNumerically("==", uint64(0)))
		Expect(event.CacheMiss).To(BeNumerically("==", uint64(0)))
		Expect(event.OffcpuPid).To(Equal(uint32(42)))
		Expect(event.OffcpuTid).To(Equal(uint32(42)))
	})

	It("efficiently collects hardware counter metrics for sched_switch events", Label("perf_event"), func() {
		experiment := gmeasure.NewExperiment("sched_switch tracepoint")
		AddReportEntry(experiment.Name, experiment)
		// Remove resource limits for kernels <5.11.
		err := rlimit.RemoveMemlock()
		Expect(err).NotTo(HaveOccurred())

		// Load eBPF Specs
		specs, err := loadKepler()
		Expect(err).NotTo(HaveOccurred())

		var obj keplerObjects
		// Load eBPF objects
		err = specs.LoadAndAssign(&obj, nil)
		Expect(err).NotTo(HaveOccurred())

		perfEvents, err := createTestHardwarePerfEvents(
			obj.CpuInstructionsEventReader,
			obj.CpuCyclesEventReader,
			obj.CacheMissEventReader,
		)
		Expect(err).NotTo(HaveOccurred())
		defer func() {
			for _, fd := range perfEvents {
				unix.Close(fd)
			}
		}()
		experiment.Sample(func(idx int) {
			experiment.MeasureDuration("sched_switch tracepoint", func() {
				runSchedSwitchTracepoint(&obj)
			}, gmeasure.Precision(time.Nanosecond))
			Expect(err).NotTo(HaveOccurred())
		}, gmeasure.SamplingConfig{N: 1000000, Duration: 10 * time.Second})
	})
})

func runSchedSwitchTracepoint(obj *keplerObjects) {
	out, err := obj.TestKeplerSchedSwitchTrace.Run(&ebpf.RunOptions{
		Flags: uint32(1), // BPF_F_TEST_RUN_ON_CPU
		CPU:   uint32(0),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(out).To(Equal(uint32(0)))
}

// This function is used to create hardware perf events for CPU cycles, instructions and cache misses.
// Instead of using hardware perf events, we use the software perf event for testing purposes.
func createTestHardwarePerfEvents(cpuCyclesMap, cpuInstructionsMap, cacheMissMap *ebpf.Map) ([]int, error) {
	cpuCyclesFd, err := unixOpenPerfEvent(unix.PERF_TYPE_SOFTWARE, unix.PERF_COUNT_SW_CPU_CLOCK, 1)
	if err != nil {
		return nil, err
	}
	err = cpuCyclesMap.Update(uint32(0), uint32(cpuCyclesFd[0]), ebpf.UpdateAny)
	if err != nil {
		return nil, err
	}

	cpuInstructionsFd, err := unixOpenPerfEvent(unix.PERF_TYPE_SOFTWARE, unix.PERF_COUNT_SW_CPU_CLOCK, 1)
	if err != nil {
		return nil, err
	}
	err = cpuInstructionsMap.Update(uint32(0), uint32(cpuInstructionsFd[0]), ebpf.UpdateAny)
	if err != nil {
		return nil, err
	}

	cacheMissFd, err := unixOpenPerfEvent(unix.PERF_TYPE_SOFTWARE, unix.PERF_COUNT_SW_CPU_CLOCK, 1)
	if err != nil {
		return nil, err
	}
	err = cacheMissMap.Update(uint32(0), uint32(cacheMissFd[0]), ebpf.UpdateAny)
	if err != nil {
		return nil, err
	}

	return []int{cpuCyclesFd[0], cpuInstructionsFd[0], cacheMissFd[0]}, nil
}
