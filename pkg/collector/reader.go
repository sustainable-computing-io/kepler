/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package collector

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"github.com/sustainable-computing-io/kepler/pkg/attacher"
	"github.com/sustainable-computing-io/kepler/pkg/cgroup"
	"github.com/sustainable-computing-io/kepler/pkg/model"
	"github.com/sustainable-computing-io/kepler/pkg/pod_lister"
	"github.com/sustainable-computing-io/kepler/pkg/power/acpi"
	"github.com/sustainable-computing-io/kepler/pkg/power/gpu"
	"github.com/sustainable-computing-io/kepler/pkg/power/rapl"
	"github.com/sustainable-computing-io/kepler/pkg/power/rapl/source"
)

// #define CPU_VECTOR_SIZE 128
import "C"

//TODO in sync with bpf program
type CgroupTime struct {
	CGroupPID      uint64
	PID            uint64
	ProcessRunTime uint64
	CPUCycles      uint64
	CPUInstr       uint64
	CacheMisses    uint64
	Command        [16]byte
	CPUTime        [C.CPU_VECTOR_SIZE]uint16
}

const (
	samplePeriodSec = 3
	samplePeriod    = samplePeriodSec * 1000 * time.Millisecond
	maxEnergyDelta  = 1000 * samplePeriodSec // for sanity check, max energy delta shouldn't be more than 1000 Watts * samplePeriod
)

var (
	// latest read energy
	sensorEnergy = map[string]float64{}
	pkgEnergy    = map[int]source.PackageEnergy{}
	gpuEnergy    = map[uint32]float64{}
	// latest process energy
	podEnergy  = map[string]*PodEnergy{}
	nodeEnergy = NewNodeEnergy()

	cpuFrequency = map[int32]uint64{}

	acpiPowerMeter = acpi.NewACPIPowerMeter()
	numCPUs        = runtime.NumCPU()
	lock           sync.Mutex

	systemProcessName      = pod_lister.GetSystemProcessName()
	systemProcessNamespace = pod_lister.GetSystemProcessNamespace()
)

// readEnergy reads sensor/pkg energies in mJ
func (c *Collector) readEnergy() {
	sensorEnergy, _ = acpiPowerMeter.GetEnergyFromHost()
	pkgEnergy = rapl.GetPackageEnergy()
}

// resetCurrValue reset existing podEnergy previous curr value
func (c *Collector) resetCurrValue() {
	for _, v := range podEnergy {
		v.ResetCurr()
	}
	nodeEnergy.ResetCurr()
}

// resetBPFTables reset BPF module's tables
func (c *Collector) resetBPFTables() {
	c.modules.Table.DeleteAll()
	c.modules.TimeTable.DeleteAll()
}

// readBPFEvent reads BPF event and maps between pid/cgroupid and container/pod
//              initializes podEnergy component if not exists
//              adds stats from BPF events (CPU time, available HW counters)
func (c *Collector) readBPFEvent() (pidPodName map[uint32]string, containerIDPodName map[string]string) {
	pidPodName = make(map[uint32]string)
	containerIDPodName = make(map[string]string)
	foundPod := make(map[string]bool)
	var ct CgroupTime
	for it := c.modules.Table.Iter(); it.Next(); {
		data := it.Leaf()
		err := binary.Read(bytes.NewBuffer(data), binary.LittleEndian, &ct)
		if err != nil {
			log.Printf("failed to decode received data: %v", err)
			continue
		}
		comm := (*C.char)(unsafe.Pointer(&ct.Command))
		// fmt.Printf("pid %v cgroup %v cmd %v\n", ct.PID, ct.CGroupPID, C.GoString(comm))
		podName, err := pod_lister.GetPodName(ct.CGroupPID, ct.PID)

		if err != nil {
			log.Printf("failed to resolve pod for cGroup ID %v: %v, set podName=%s", ct.CGroupPID, err, systemProcessName)
			podName = systemProcessName
		}
		if _, ok := podEnergy[podName]; !ok {
			// new pod
			var podNamespace string
			if podName == systemProcessName {
				podNamespace = systemProcessNamespace
			} else {
				podNamespace, err = pod_lister.GetPodNameSpace(ct.CGroupPID, ct.PID)
				if err != nil {
					log.Printf("failed to find namespace for cGroup ID %v: %v", ct.CGroupPID, err)
					podNamespace = "unknown"
				}
			}
			podEnergy[podName] = NewPodEnergy(podName, podNamespace)
		}
		foundPod[podName] = true

		podEnergy[podName].SetLatestProcess(ct.CGroupPID, ct.PID, C.GoString(comm))

		var activeCPUs []int32
		var avgFreq float64
		var totalCPUTime uint64
		if attacher.EnableCPUFreq {
			avgFreq, totalCPUTime, activeCPUs = getAVGCPUFreqAndTotalCPUTime(cpuFrequency, ct.CPUTime)
			podEnergy[podName].AvgCPUFreq = avgFreq
		} else {
			totalCPUTime = uint64(ct.ProcessRunTime)
			activeCPUs = getActiveCPUs(ct.CPUTime)
		}

		for _, cpu := range activeCPUs {
			podEnergy[podName].CurrCPUTimePerCPU[uint32(cpu)] += uint64(ct.CPUTime[cpu])
		}

		podEnergy[podName].CPUTime.AddNewCurr(totalCPUTime)

		for _, counterKey := range availableCounters {
			var val uint64
			switch counterKey {
			case attacher.CPU_CYCLE_LABEL:
				val = ct.CPUCycles
			case attacher.CPU_INSTRUCTION_LABEL:
				val = ct.CPUInstr
			case attacher.CACHE_MISS_LABEL:
				val = ct.CacheMisses
			default:
				val = 0
			}
			podEnergy[podName].CounterStats[counterKey].AddNewCurr(val)
		}

		podEnergy[podName].CurrProcesses++
		containerID, err := pod_lister.GetContainerID(ct.CGroupPID, ct.PID)
		// first-time found container (should not include non-container event)
		if _, found := containerIDPodName[containerID]; !found && podName != systemProcessName {
			containerIDPodName[containerID] = podName
			// TO-DO: move to container-level section
			rBytes, wBytes, disks, err := pod_lister.ReadCgroupIOStat(ct.CGroupPID, ct.PID)

			if err == nil {
				if disks > podEnergy[podName].Disks {
					podEnergy[podName].Disks = disks
				}
				podEnergy[podName].BytesRead.AddStat(containerID, rBytes)
				podEnergy[podName].BytesWrite.AddStat(containerID, wBytes)
			}
		}
		pid := uint32(ct.PID)
		if _, found := pidPodName[pid]; !found {
			pidPodName[pid] = podName
		}
	}
	c.resetBPFTables()
	// clean podEnergy
	for podName, _ := range podEnergy {
		if _, found := foundPod[podName]; !found {
			delete(podEnergy, podName)
		}
	}
	return
}

// readCgroup adds container-level cgroup data
func (c *Collector) readCgroup(containerIDPodName map[string]string) {
	for containerID, podName := range containerIDPodName {
		cgroup.TryInitStatReaders(containerID)
		cgroupFSStandardStats := cgroup.GetStandardStat(containerID)
		for cgroupFSKey, cgroupFSValue := range cgroupFSStandardStats {
			readVal := cgroupFSValue.(uint64)
			if _, ok := podEnergy[podName].CgroupFSStats[cgroupFSKey]; ok {
				podEnergy[podName].CgroupFSStats[cgroupFSKey].AddStat(containerID, readVal)
			}
		}
	}
	return
}

// readKubelet adds kubelet data (resident mem)
func (c *Collector) readKubelet() {
	if len(availableKubeletMetrics) == 2 {
		podCPU, podMem, _, _, _ := pod_lister.GetPodMetrics()
		log.Printf("Kubelet Read: %v, %v\n", podCPU, podMem)
		for podName, v := range podEnergy {
			k := v.Namespace + "/" + podName
			readCPU := uint64(podCPU[k])
			readMem := uint64(podMem[k])
			cpuMetricName := availableKubeletMetrics[0]
			memMetricName := availableKubeletMetrics[1]
			v.KubeletStats[cpuMetricName].SetNewAggr(readCPU)
			v.KubeletStats[memMetricName].SetNewAggr(readMem)
		}
	}
}

func (c *Collector) reader() {
	ticker := time.NewTicker(samplePeriod)
	go func() {
		_ = gpu.GetGpuEnergy() // reset power usage counter
		c.resetBPFTables()
		c.readEnergy()
		nodeEnergy.SetValues(sensorEnergy, pkgEnergy, 0, map[string]float64{}) // set initial energy
		acpiPowerMeter.Run()
		for {
			select {
			case <-ticker.C:
				lock.Lock()
				c.resetCurrValue()
				var coreNDelta, dramNDelta, uncoreNDelta, pkgNDelta, gpuNDelta []float64
				// read node-level settings (frequency)
				cpuFrequency = acpiPowerMeter.GetCPUCoreFrequency()
				// read pod metrics
				pidPodName, containerIDPodName := c.readBPFEvent()
				c.readCgroup(containerIDPodName)
				c.readKubelet()
				// convert to pod metrics to array
				var podMetricValues [][]float64
				var podNameList []string
				for podName, v := range podEnergy {
					values := v.ToEstimatorValues()
					podMetricValues = append(podMetricValues, values)
					podNameList = append(podNameList, podName)
				}
				// TO-DO: handle metrics read by GPU device in the same way as the other usage metrics
				// read gpu power
				gpuPerPid, _ := gpu.GetCurrGpuEnergyPerPid() // power not energy
				podGPUDelta := make(map[string]float64)
				for pid, podName := range pidPodName {
					gpuPower := gpuPerPid[pid]
					if _, found := podGPUDelta[podName]; !found {
						podGPUDelta[podName] = 0
					} else {
						podGPUDelta[podName] += gpuPower
					}
				}
				for _, podName := range podNameList {
					gpuNDelta = append(gpuNDelta, podGPUDelta[podName])
				}
				// read and compute power (energy delta)
				var totalCorePower, totalDRAMPower, totalUncorePower, totalPkgPower, totalGPUPower uint64

				for _, val := range gpuNDelta {
					totalGPUPower += uint64(val)
				}
				c.readEnergy()
				sumUsage := model.GetSumUsageMap(metricNames, podMetricValues)
				nodeEnergy.SetValues(sensorEnergy, pkgEnergy, totalGPUPower, sumUsage)
				for pkgIDKey, pkgStat := range nodeEnergy.EnergyInPkg.Stat {
					coreDelta, dramDelta, uncoreDelta := nodeEnergy.GetCurrPerPkg(pkgIDKey)
					pkgDelta := pkgStat.Curr
					coreNDelta = append(coreNDelta, float64(coreDelta))
					dramNDelta = append(dramNDelta, float64(dramDelta))
					uncoreNDelta = append(uncoreNDelta, float64(uncoreDelta))
					pkgNDelta = append(pkgNDelta, float64(pkgDelta))
					totalCorePower += coreDelta
					totalDRAMPower += dramDelta
					totalUncorePower += uncoreDelta
					totalPkgPower += pkgDelta
				}
				// get power from usage ratio
				podCore, podDRAM, podUncore, podPkg := model.GetPowerFromUsageRatio(podMetricValues, totalCorePower, totalDRAMPower, totalUncorePower, totalPkgPower, sumUsage)
				// get dynamic power from usage metrics
				podDynamicPower := model.GetDynamicPower(metricNames, podMetricValues, coreNDelta, dramNDelta, uncoreNDelta, pkgNDelta, gpuNDelta)
				// get other energy - divide equally
				podOther := uint64(0)
				podCount := len(podNameList)
				if podCount > 0 {
					podOther = nodeEnergy.EnergyInOther / uint64(podCount)
				}

				// set pod energy
				for i, podName := range podNameList {
					podEnergy[podName].EnergyInCore.AddNewCurr(podCore[i])
					podEnergy[podName].EnergyInDRAM.AddNewCurr(podDRAM[i])
					podEnergy[podName].EnergyInUncore.AddNewCurr(podUncore[i])
					podEnergy[podName].EnergyInPkg.AddNewCurr(podPkg[i])
					podGPU := uint64(math.Ceil(podGPUDelta[podName]))
					podEnergy[podName].EnergyInGPU.AddNewCurr(podGPU)
					podEnergy[podName].EnergyInOther.AddNewCurr(podOther)
				}
				if len(podDynamicPower) != 0 {
					for i, podName := range podNameList {
						power := uint64(podDynamicPower[i])
						podEnergy[podName].DynEnergy.AddNewCurr(power)
					}
					fmt.Printf("Get pod powers: %v \n %v from %v (%d x %d)\n", podMetricValues[0:2], podDynamicPower, metricNames, len(podMetricValues), len(podMetricValues[0]))
				}
				for _, v := range podEnergy {
					fmt.Println(v)
				}
				fmt.Println(nodeEnergy)
				lock.Unlock()
			}
		}
	}()
}

// getAVGCPUFreqAndTotalCPUTime calculates the weighted cpu frequency average
func getAVGCPUFreqAndTotalCPUTime(cpuFrequency map[int32]uint64, cpuTime [C.CPU_VECTOR_SIZE]uint16) (avgFreq float64, totalCPUTime uint64, activeCPUs []int32) {
	totalFreq := float64(0)
	totalFreqWithoutWeight := float64(0)
	for cpu, freq := range cpuFrequency {
		if int(cpu) > len(cpuTime)-1 {
			break
		}
		totalCPUTime += uint64(cpuTime[cpu])
		totalFreqWithoutWeight += float64(freq)
	}
	if totalCPUTime == 0 {
		if len(cpuFrequency) == 0 {
			return
		}
		avgFreq = totalFreqWithoutWeight / float64(len(cpuFrequency))
	} else {
		for cpu, freq := range cpuFrequency {
			if int(cpu) > len(cpuTime)-1 {
				break
			}
			if cpuTime[cpu] != 0 {
				totalFreq += float64(freq) * (float64(cpuTime[cpu]) / float64(totalCPUTime))
				activeCPUs = append(activeCPUs, cpu)
			}
		}
		avgFreq = totalFreqWithoutWeight / float64(len(cpuFrequency))
	}
	return
}

// getActiveCPUs returns active cpu(vcpu) (in case that frequency is not active)
func getActiveCPUs(cpuTime [C.CPU_VECTOR_SIZE]uint16) (activeCPUs []int32) {
	for cpu, _ := range cpuTime {
		if cpuTime[cpu] != 0 {
			activeCPUs = append(activeCPUs, int32(cpu))
		}
	}
	return
}
