//go:build !darwin
// +build !darwin

package bpftest

import (
	"fmt"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/cilium/ebpf"
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
	It("should increment the page cache hit counter", func() {
		// Remove resource limits for kernels <5.11.
		err := rlimit.RemoveMemlock()
		Expect(err).NotTo(HaveOccurred())

		// Load eBPF Specs
		specs, err := loadTest()
		Expect(err).NotTo(HaveOccurred())

		err = specs.RewriteConstants(map[string]interface{}{
			"TEST": int32(1),
		})
		Expect(err).NotTo(HaveOccurred())

		var obj testObjects
		// Load eBPF objects
		err = specs.LoadAndAssign(&obj, nil)
		Expect(err).NotTo(HaveOccurred())

		key := uint32(0)

		err = obj.Processes.Put(key, testProcessMetricsT{
			CgroupId:       0,
			Pid:            0,
			ProcessRunTime: 0,
			CpuCycles:      0,
			CpuInstr:       0,
			CacheMiss:      0,
			PageCacheHit:   0,
			VecNr:          [10]uint16{},
			Comm:           [16]int8{},
		})
		Expect(err).NotTo(HaveOccurred())

		out, err := obj.TestKeplerWritePageTrace.Run(&ebpf.RunOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(Equal(uint32(0)))

		// Read the page cache hit counter
		var res testProcessMetricsT
		err = obj.Processes.Lookup(key, &res)
		Expect(err).NotTo(HaveOccurred())

		Expect(res.PageCacheHit).To(BeNumerically("==", uint64(1)))

		err = obj.Processes.Delete(key)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should register a new process if one doesn't exist", func() {
		// Remove resource limits for kernels <5.11.
		err := rlimit.RemoveMemlock()
		Expect(err).NotTo(HaveOccurred())

		// Load eBPF Specs
		specs, err := loadTest()
		Expect(err).NotTo(HaveOccurred())

		err = specs.RewriteConstants(map[string]interface{}{
			"TEST": int32(1),
		})
		Expect(err).NotTo(HaveOccurred())

		var obj testObjects
		// Load eBPF objects
		err = specs.LoadAndAssign(&obj, nil)
		Expect(err).NotTo(HaveOccurred())

		out, err := obj.TestRegisterNewProcessIfNotExist.Run(&ebpf.RunOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(BeNumerically("==", uint32(0)))

		// Read the page cache hit counter
		var res testProcessMetricsT
		key := uint32(42) // Kernel TGID
		err = obj.Processes.Lookup(key, &res)
		Expect(err).NotTo(HaveOccurred())

		Expect(res.Pid).To(BeNumerically("==", uint64(42)))

		err = obj.Processes.Delete(key)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should increment the page hit counter efficiently", func() {
		experiment := gmeasure.NewExperiment("Increment the page hit counter")
		AddReportEntry(experiment.Name, experiment)
		// Remove resource limits for kernels <5.11.
		err := rlimit.RemoveMemlock()
		Expect(err).NotTo(HaveOccurred())

		// Load eBPF Specs
		specs, err := loadTest()
		Expect(err).NotTo(HaveOccurred())

		err = specs.RewriteConstants(map[string]interface{}{
			"TEST": int32(1),
		})
		Expect(err).NotTo(HaveOccurred())

		var obj testObjects
		// Load eBPF objects
		err = specs.LoadAndAssign(&obj, nil)
		Expect(err).NotTo(HaveOccurred())

		key := uint32(0)

		err = obj.Processes.Put(key, testProcessMetricsT{
			CgroupId:       0,
			Pid:            0,
			ProcessRunTime: 0,
			CpuCycles:      0,
			CpuInstr:       0,
			CacheMiss:      0,
			PageCacheHit:   0,
			VecNr:          [10]uint16{},
			Comm:           [16]int8{},
		})
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
		specs, err := loadTest()
		Expect(err).NotTo(HaveOccurred())

		err = specs.RewriteConstants(map[string]interface{}{
			"TEST": int32(1),
			"HW":   int32(1),
		})
		Expect(err).NotTo(HaveOccurred())

		var obj testObjects
		// Load eBPF objects
		err = specs.LoadAndAssign(&obj, nil)
		Expect(err).NotTo(HaveOccurred())

		perfEvents, err := createHardwarePerfEvents(
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

		// Register TGID 42 - This would be done by register_new_process_if_not_exist
		// when we get a sched_switch event for a new process
		key := uint32(42)
		nsecs := getNSecs()
		err = obj.Processes.Put(key, testProcessMetricsT{
			CgroupId:       0,
			Pid:            42,
			ProcessRunTime: nsecs,
			CpuCycles:      0,
			CpuInstr:       0,
			CacheMiss:      0,
			PageCacheHit:   0,
			VecNr:          [10]uint16{},
			Comm:           [16]int8{},
		})
		Expect(err).NotTo(HaveOccurred())
		err = obj.PidTimeMap.Put(key, nsecs)
		Expect(err).NotTo(HaveOccurred())

		out, err := obj.TestKeplerSchedSwitchTrace.Run(&ebpf.RunOptions{
			Flags: uint32(1), // BPF_F_TEST_RUN_ON_CPU
			CPU:   uint32(0),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(Equal(uint32(0)))

		var res testProcessMetricsT
		err = obj.Processes.Lookup(key, &res)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.CpuCycles).To(BeNumerically(">", uint64(0)))
		Expect(res.CpuInstr).To(BeNumerically(">", uint64(0)))
		Expect(res.CacheMiss).To(BeNumerically(">", uint64(0)))

		err = obj.Processes.Delete(key)
		Expect(err).NotTo(HaveOccurred())
	})

	It("collects metrics for sched_switch events when no hardware events are enabled", Label("perf_event"), func() {
		// Remove resource limits for kernels <5.11.
		err := rlimit.RemoveMemlock()
		Expect(err).NotTo(HaveOccurred())

		// Load eBPF Specs
		specs, err := loadTest()
		Expect(err).NotTo(HaveOccurred())

		err = specs.RewriteConstants(map[string]interface{}{
			"TEST": int32(1),
			"HW":   int32(0),
		})
		Expect(err).NotTo(HaveOccurred())

		var obj testObjects
		// Load eBPF objects
		err = specs.LoadAndAssign(&obj, nil)
		Expect(err).NotTo(HaveOccurred())

		// Register TGID 42 - This would be done by register_new_process_if_not_exist
		// when we get a sched_switch event for a new process
		key := uint32(42)
		nsecs := getNSecs()
		err = obj.Processes.Put(key, testProcessMetricsT{
			CgroupId:       0,
			Pid:            42,
			ProcessRunTime: nsecs,
			CpuCycles:      0,
			CpuInstr:       0,
			CacheMiss:      0,
			PageCacheHit:   0,
			VecNr:          [10]uint16{},
			Comm:           [16]int8{},
		})
		Expect(err).NotTo(HaveOccurred())
		err = obj.PidTimeMap.Put(key, nsecs)
		Expect(err).NotTo(HaveOccurred())

		out, err := obj.TestKeplerSchedSwitchTrace.Run(&ebpf.RunOptions{
			Flags: uint32(1), // BPF_F_TEST_RUN_ON_CPU
			CPU:   uint32(0),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(Equal(uint32(0)))

		var res testProcessMetricsT
		err = obj.Processes.Lookup(key, &res)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.CpuCycles).To(BeNumerically("==", uint64(0)))
		Expect(res.ProcessRunTime).To(BeNumerically(">", uint64(0)))

		err = obj.Processes.Delete(key)
		Expect(err).NotTo(HaveOccurred())
	})

	It("efficiently collects hardware counter metrics for sched_switch events", Label("perf_event"), func() {
		experiment := gmeasure.NewExperiment("sched_switch tracepoint")
		AddReportEntry(experiment.Name, experiment)
		// Remove resource limits for kernels <5.11.
		err := rlimit.RemoveMemlock()
		Expect(err).NotTo(HaveOccurred())

		// Load eBPF Specs
		specs, err := loadTest()
		Expect(err).NotTo(HaveOccurred())

		err = specs.RewriteConstants(map[string]interface{}{
			"TEST": int32(1),
		})
		Expect(err).NotTo(HaveOccurred())

		var obj testObjects
		// Load eBPF objects
		err = specs.LoadAndAssign(&obj, nil)
		Expect(err).NotTo(HaveOccurred())

		perfEvents, err := createHardwarePerfEvents(
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
			preRunSchedSwitchTracepoint(&obj)
			experiment.MeasureDuration("sampled sched_switch tracepoint", func() {
				runSchedSwitchTracepoint(&obj)
			}, gmeasure.Precision(time.Nanosecond))
			err = obj.Processes.Delete(uint32(42))
			Expect(err).NotTo(HaveOccurred())
		}, gmeasure.SamplingConfig{N: 1000000, Duration: 10 * time.Second})
	})

	It("uses sample rate to reduce CPU time", Label("perf_event"), func() {
		experiment := gmeasure.NewExperiment("sampled sched_switch tracepoint")
		AddReportEntry(experiment.Name, experiment)
		// Remove resource limits for kernels <5.11.
		err := rlimit.RemoveMemlock()
		Expect(err).NotTo(HaveOccurred())

		// Load eBPF Specs
		specs, err := loadTest()
		Expect(err).NotTo(HaveOccurred())

		err = specs.RewriteConstants(map[string]interface{}{
			"TEST":        int32(1),
			"SAMPLE_RATE": int32(1000),
		})
		Expect(err).NotTo(HaveOccurred())

		var obj testObjects
		// Load eBPF objects
		err = specs.LoadAndAssign(&obj, nil)
		Expect(err).NotTo(HaveOccurred())

		perfEvents, err := createHardwarePerfEvents(
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
			preRunSchedSwitchTracepoint(&obj)
			experiment.MeasureDuration("sampled sched_switch tracepoint", func() {
				runSchedSwitchTracepoint(&obj)
			}, gmeasure.Precision(time.Nanosecond))
			err = obj.Processes.Delete(uint32(42))
			Expect(err).NotTo(HaveOccurred())
		}, gmeasure.SamplingConfig{N: 1000000, Duration: 10 * time.Second})
	})
})

func getNSecs() uint64 {
	var ts syscall.Timespec
	_, _, err := syscall.Syscall(syscall.SYS_CLOCK_GETTIME, 4, uintptr(unsafe.Pointer(&ts)), 0)
	if err != 0 {
		panic(err)
	}
	return uint64(ts.Sec*1e9 + ts.Nsec)
}

func preRunSchedSwitchTracepoint(obj *testObjects) {
	// Register TGID 42 - This would be done by register_new_process_if_not_exist
	// when we get a sched_switch event for a new process
	key := uint32(42)
	nsecs := getNSecs()
	err := obj.Processes.Put(key, testProcessMetricsT{
		CgroupId:       0,
		Pid:            42,
		ProcessRunTime: nsecs,
		CpuCycles:      0,
		CpuInstr:       0,
		CacheMiss:      0,
		PageCacheHit:   0,
		VecNr:          [10]uint16{},
		Comm:           [16]int8{},
	})
	Expect(err).NotTo(HaveOccurred())
	err = obj.PidTimeMap.Put(key, nsecs)
	Expect(err).NotTo(HaveOccurred())
}

func runSchedSwitchTracepoint(obj *testObjects) {
	out, err := obj.TestKeplerSchedSwitchTrace.Run(&ebpf.RunOptions{
		Flags: uint32(1), // BPF_F_TEST_RUN_ON_CPU
		CPU:   uint32(0),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(out).To(Equal(uint32(0)))
}

func unixOpenPerfEvent(typ, conf int) (int, error) {
	sysAttr := &unix.PerfEventAttr{
		Type:   uint32(typ),
		Size:   uint32(unsafe.Sizeof(unix.PerfEventAttr{})),
		Config: uint64(conf),
	}

	cloexecFlags := unix.PERF_FLAG_FD_CLOEXEC
	fd, err := unix.PerfEventOpen(sysAttr, -1, 0, -1, cloexecFlags)
	if fd < 0 {
		return 0, fmt.Errorf("failed to open bpf perf event on cpu 0: %w", err)
	}

	return fd, nil
}

// This function is used to create hardware perf events for CPU cycles, instructions and cache misses.
// Instead of using hardware perf events, we use the software perf event for testing purposes.
func createHardwarePerfEvents(cpuCyclesMap, cpuInstructionsMap, cacheMissMap *ebpf.Map) ([]int, error) {
	cpuCyclesFd, err := unixOpenPerfEvent(unix.PERF_TYPE_SOFTWARE, unix.PERF_COUNT_SW_CPU_CLOCK)
	if err != nil {
		return nil, err
	}
	err = cpuCyclesMap.Update(uint32(0), uint32(cpuCyclesFd), ebpf.UpdateAny)
	if err != nil {
		return nil, err
	}

	cpuInstructionsFd, err := unixOpenPerfEvent(unix.PERF_TYPE_SOFTWARE, unix.PERF_COUNT_SW_CPU_CLOCK)
	if err != nil {
		return nil, err
	}
	err = cpuInstructionsMap.Update(uint32(0), uint32(cpuInstructionsFd), ebpf.UpdateAny)
	if err != nil {
		return nil, err
	}

	cacheMissFd, err := unixOpenPerfEvent(unix.PERF_TYPE_SOFTWARE, unix.PERF_COUNT_SW_CPU_CLOCK)
	if err != nil {
		return nil, err
	}
	err = cacheMissMap.Update(uint32(0), uint32(cacheMissFd), ebpf.UpdateAny)
	if err != nil {
		return nil, err
	}

	return []int{cpuCyclesFd, cpuInstructionsFd, cacheMissFd}, nil
}
