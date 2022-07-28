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
	"os"
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

type PodEnergy struct {
	CGroupPID uint64
	PID       uint64
	PodName   string
	Namespace string
	Command   string

	AggCPUTime     float64
	AggCPUCycles   uint64
	AggCPUInstr    uint64
	AggCacheMisses uint64

	CurrCPUTime     float64
	CurrCPUCycles   uint64
	CurrCPUInstr    uint64
	CurrCacheMisses uint64
	CurrResidentMem uint64

	CurrEnergyInCore  uint64
	CurrEnergyInDram  uint64
	CurrEnergyInOther uint64
	CurrEnergyInGPU   uint64
	AggEnergyInCore   uint64
	AggEnergyInDram   uint64
	AggEnergyInOther  uint64
	AggEnergyInGPU    uint64

	Disks          int
	CurrBytesRead  uint64
	CurrBytesWrite uint64
	AggBytesRead   uint64
	AggBytesWrite  uint64

	AvgCPUFreq    float64
	CurrProcesses int
	CgroupFSStats map[string]*UInt64Stat
	CurrCPUTimePerCPU map[uint32]uint64
}

type UInt64Stat struct {
	Curr uint64
	Aggr uint64
}

func (s UInt64Stat) String() string {
	return fmt.Sprintf("%d (%d)", s.Curr, s.Aggr)
}

type CurrNodeEnergy struct {
	CPUTime     float64
	CPUCycles   uint64
	CPUInstr    uint64
	CacheMisses uint64
	NodeMem     float64

	EnergyInCore  float64
	EnergyInDram  float64
	EnergyInOther float64
	EnergyInGPU   float64
}

const (
	samplePeriodSec = 3
	samplePeriod    = samplePeriodSec * 1000 * time.Millisecond
	maxEnergyDelta  = 1000000 * samplePeriodSec // for sanity check, max energy delta shouldn't be more than 1000 Watts * samplePeriod
)

var (
	podEnergy      = map[string]*PodEnergy{}
	nodeEnergy     = map[string]float64{}
	gpuEnergy      = map[uint32]float64{}
	currNodeEnergy = &CurrNodeEnergy{}
	currPackageEnergy = map[int]source.PackageEnergy{}
	cpuFrequency   = map[int32]uint64{}
	nodeName, _    = os.Hostname()
	cpuArch        = "unknown"
	acpiPowerMeter = acpi.NewACPIPowerMeter()
	numCPUs        = runtime.NumCPU()
	lock           sync.Mutex
)

func init() {
	arch, err := source.GetCPUArchitecture()
	if err == nil {
		cpuArch = arch
	}
}

func (c *Collector) reader() {
	ticker := time.NewTicker(samplePeriod)
	go func() {
		lastEnergyCore, _ := rapl.GetEnergyFromCore()
		lastEnergyDram, _ := rapl.GetEnergyFromDram()
		_ = gpu.GetGpuEnergy() // reset power usage counter

		acpiPowerMeter.Run()
		for {
			samples := float64(1)
			select {
			case <-ticker.C:
				lock.Lock()
				cpuFrequency = acpiPowerMeter.GetCPUCoreFrequency()
				nodeEnergy, _ = acpiPowerMeter.GetEnergyFromHost()
				currPackageEnergy = rapl.GetPackageEnergy()

				var aggCPUTime, avgFreq, totalCPUTime float64
				var aggCPUCycles, aggCPUInstr, aggCacheMisses, aggBytesRead, aggBytesWrite uint64
				var aggProcesses int
				avgFreq = 0
				totalCPUTime = 0
				energyCore, err := rapl.GetEnergyFromCore()
				if err != nil {
					log.Printf("failed to get core power: %v\n", err)
					samples += 1
					lock.Unlock()
					continue
				}
				energyDram, err := rapl.GetEnergyFromDram()
				if err != nil {
					log.Printf("failed to get dram power: %v\n", err)
					samples += 1
					lock.Unlock()
					continue
				}
				if energyCore < lastEnergyCore || energyDram < lastEnergyDram {
					log.Printf("failed to get latest core or dram energy. Core energy %v should be more than %v; Dram energy %v should be more than %v\n",
						energyCore, lastEnergyCore, energyDram, lastEnergyDram)
					// reset last value
					log.Println("reset last energy")
					if energyCore < lastEnergyCore {
						lastEnergyCore = energyCore
					}
					if energyDram < lastEnergyDram {
						lastEnergyDram = energyDram
					}
					samples = 1
					lock.Unlock()
					continue
				}

				coreDelta := float64(energyCore-lastEnergyCore) / samples
				dramDelta := float64(energyDram-lastEnergyDram) / samples
				if coreDelta == 0 && dramDelta == 0 {
					log.Printf("power reading not changed, retry\n")
					samples += 1
					lock.Unlock()
					continue
				}

				if coreDelta > maxEnergyDelta || dramDelta > maxEnergyDelta {
					log.Printf("power reading off limit: core: %f, dram %f retry\n", coreDelta, dramDelta)
					// reset last value
					log.Println("reset last energy")
					if coreDelta > maxEnergyDelta {
						lastEnergyCore = energyCore
					}
					if dramDelta > maxEnergyDelta {
						lastEnergyDram = energyDram
					}
					samples = 1
					lock.Unlock()
					continue
				}

				gpuDelta := float64(0)
				for _, e := range gpuEnergy {
					gpuDelta += e
				}
				lastEnergyCore = energyCore
				lastEnergyDram = energyDram

				// calculate the total energy consumed in node from all sensors
				var nodeEnergyTotal float64 = 0
				for _, energy := range nodeEnergy {
					nodeEnergyTotal += energy
				}
				// ccalculate the other energy consumed besides CPU/GPU and memory
				otherDelta := float64(0)
				if nodeEnergyTotal > 0 {
					otherDelta = nodeEnergyTotal - coreDelta - dramDelta - gpuDelta
				}

				var ct CgroupTime
				aggCPUTime = 0
				aggCPUCycles = 0
				aggCacheMisses = 0
				aggCPUCycles = 0
				aggCPUInstr = 0
				aggBytesRead = 0
				aggBytesWrite = 0
				cgroupIO := make(map[uint64]bool)
				aggProcesses = 0

				gpuEnergy, _ = gpu.GetCurrGpuEnergyPerPid()

				for _, v := range podEnergy {
					v.CurrCPUCycles = 0
					v.CurrCPUTime = 0
					v.CurrCacheMisses = 0
					v.CurrCPUInstr = 0
					v.CurrBytesRead = 0
					v.CurrBytesWrite = 0
					v.CurrProcesses = 0
					v.CurrCPUTimePerCPU = make(map[uint32]uint64)
				}
				
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
						log.Printf("failed to resolve pod for cGroup ID %v: %v", ct.CGroupPID, err)
						continue
					}
					if _, ok := podEnergy[podName]; !ok {
						podEnergy[podName] = &PodEnergy{}
						podEnergy[podName].PodName = podName
						podNamespace, err := pod_lister.GetPodNameSpace(ct.CGroupPID, ct.PID)
						if err != nil {
							log.Printf("failed to find namespace for cGroup ID %v: %v", ct.CGroupPID, err)
							podNamespace = "unknown"
						}
						podEnergy[podName].Namespace = podNamespace
						podEnergy[podName].CGroupPID = ct.CGroupPID
						podEnergy[podName].PID = ct.PID
						podEnergy[podName].Command = C.GoString(comm)
						podEnergy[podName].CgroupFSStats = make(map[string]*UInt64Stat)
						podEnergy[podName].CurrCPUTimePerCPU = make(map[uint32]uint64)
					}
					
					var activeCPUs []int32
					
					if attacher.EnableCPUFreq {
						avgFreq, totalCPUTime, activeCPUs = getAVGCPUFreqAndTotalCPUTime(cpuFrequency, ct.CPUTime)
					} else {
						activeCPUs = getActiveCPUs(ct.CPUTime)
					}

					for _, cpu := range activeCPUs {
						podEnergy[podName].CurrCPUTimePerCPU[uint32(cpu)] += uint64(ct.CPUTime[cpu])
					}

					if totalCPUTime == 0 {
						// cannot get CPUTime divided by Frequency, set to total process runtime
						totalCPUTime = float64(ct.ProcessRunTime)
					}

					// to prevent overflow of the counts we change the unit to have smaller numbers
					totalCPUTime = totalCPUTime / 1000
					podEnergy[podName].CurrCPUTime += totalCPUTime
					podEnergy[podName].AggCPUTime += totalCPUTime
					aggCPUTime += totalCPUTime
					val := ct.CPUCycles
					podEnergy[podName].CurrCPUCycles += val
					podEnergy[podName].AggCPUCycles += val
					aggCPUCycles += val
					val = ct.CPUInstr
					podEnergy[podName].CurrCPUInstr += val
					podEnergy[podName].AggCPUInstr += val
					aggCPUInstr += val
					val = ct.CacheMisses
					podEnergy[podName].CurrCacheMisses += val
					podEnergy[podName].AggCacheMisses += val
					aggCacheMisses += val

					podEnergy[podName].CurrProcesses++
					aggProcesses++

					podEnergy[podName].AvgCPUFreq = avgFreq
					if e, ok := gpuEnergy[uint32(ct.PID)]; ok {
						// fmt.Printf("gpu energy pod %v comm %v pid %v: %v\n", podName, C.GoString(comm), ct.PID, e)
						podEnergy[podName].CurrEnergyInGPU += uint64(e)
						podEnergy[podName].AggEnergyInGPU += podEnergy[podName].CurrEnergyInGPU
					}
					rBytes, wBytes, disks, err := pod_lister.ReadCgroupIOStat(ct.CGroupPID, ct.PID)

					containerID, err := pod_lister.GetContainerID(ct.CGroupPID, ct.PID)

					cgroup.TryInitStatReaders(containerID)
					cgroupFSStandardStats := cgroup.GetStandardStat(containerID)
					for cgroupFSKey, cgroupFSValue := range cgroupFSStandardStats {
						if _, ok := podEnergy[podName].CgroupFSStats[cgroupFSKey]; !ok {
							podEnergy[podName].CgroupFSStats[cgroupFSKey] = &UInt64Stat{
								Curr: cgroupFSValue.(uint64),
							}
						} else {
							podEnergy[podName].CgroupFSStats[cgroupFSKey].Curr += cgroupFSValue.(uint64)
						}
					}
					// fmt.Printf("read %d write %d. Agg read %d write %d, err %v\n", rBytes, wBytes, aggBytesRead, aggBytesWrite, err)
					if err == nil {
						// if this is the first time the cgroup's I/O is accounted, add it to the pod
						if _, ok := cgroupIO[ct.CGroupPID]; !ok {
							cgroupIO[ct.CGroupPID] = true
							if disks > podEnergy[podName].Disks {
								podEnergy[podName].Disks = disks
							}
							// save the current I/O in CurrByteRead and adjust it later
							podEnergy[podName].CurrBytesRead += rBytes
							aggBytesRead += rBytes
							podEnergy[podName].CurrBytesWrite += wBytes
							aggBytesWrite += wBytes
						}
					}
				}
				// reset all counters in the eBPF table
				c.modules.Table.DeleteAll()
				totalReadBytes, totalWriteBytes, disks, err := pod_lister.ReadAllCgroupIOStat()

				if err == nil {
					if totalReadBytes > aggBytesRead && totalWriteBytes > aggBytesWrite {
						rBytes := totalReadBytes - aggBytesRead
						wBytes := totalWriteBytes - aggBytesWrite
						podName := pod_lister.GetSystemProcessName()
						podEnergy[podName].Disks = disks
						podEnergy[podName].CurrBytesRead = rBytes
						podEnergy[podName].CurrBytesWrite = wBytes
					} else {
						fmt.Printf("total read %d write %d should be greater than agg read %d agg write %d\n", totalReadBytes, totalWriteBytes, aggBytesRead, aggBytesWrite)
					}
				}

				//evenly attribute other energy among all pods
				perProcessOtherMJ := float64(otherDelta / float64(len(podEnergy)))

				_, podMem, _, nodeMem, err := pod_lister.GetPodMetrics()
				if err != nil {
					fmt.Printf("failed to get kubelet metrics: %v", err)
				}

				log.Printf("energy count: core %.2f dram: %.2f time %.6f cycles %d instructions %d misses %d node memory %f\n",
					coreDelta, dramDelta, aggCPUTime, aggCPUCycles, aggCPUInstr, aggCacheMisses, nodeMem)
				currNodeEnergy = &CurrNodeEnergy{
					CPUTime:       aggCPUTime,
					CPUCycles:     aggCPUCycles,
					CPUInstr:      aggCPUInstr,
					CacheMisses:   aggCacheMisses,
					NodeMem:       nodeMem,
					EnergyInCore:  coreDelta,
					EnergyInDram:  dramDelta,
					EnergyInOther: otherDelta,
					EnergyInGPU:   gpuDelta,
				}
				for podName, v := range podEnergy {
					// evenly attribute core intercept (i.e. static power) on all processes
					interceptCoreRatio := float64(model.RunTimeCoeff.InterceptCore / float64(aggProcesses))
					// evenly attribute dram intercept on all node memory
					interceptDramRatio := float64(model.RunTimeCoeff.InterceptDram / float64(nodeMem))

					v.CurrEnergyInCore = uint64(model.RunTimeCoeff.CPUTime*v.CurrCPUTime +
						model.RunTimeCoeff.CPUCycle*float64(v.CurrCPUCycles) +
						model.RunTimeCoeff.CPUInstr*float64(v.CurrCPUInstr) + interceptCoreRatio*float64(v.CurrProcesses))
					v.AggEnergyInCore += v.CurrEnergyInCore

					k := v.Namespace + "/" + podName
					if mem, ok := podMem[k]; ok {
						v.CurrResidentMem = uint64(mem)
					}

					v.CurrEnergyInDram = uint64(model.RunTimeCoeff.CacheMisses*float64(v.CurrCacheMisses) +
						interceptDramRatio*float64(v.CurrResidentMem))
					v.AggEnergyInDram += v.CurrEnergyInDram
					v.CurrEnergyInOther = uint64(perProcessOtherMJ)
					v.AggEnergyInOther += uint64(perProcessOtherMJ)

					val := uint64(0)
					if v.CurrBytesRead >= v.AggBytesRead {
						val = v.CurrBytesRead - v.AggBytesRead
						v.AggBytesRead = v.CurrBytesRead
						v.CurrBytesRead = val
					}
					if v.CurrBytesWrite >= v.AggBytesWrite {
						val = v.CurrBytesWrite - v.AggBytesWrite
						v.AggBytesWrite = v.CurrBytesWrite
						v.CurrBytesWrite = val
					}
					for cgroupFSKey, FSStat := range v.CgroupFSStats {
						if FSStat.Curr >= FSStat.Aggr {
							val = FSStat.Curr - FSStat.Aggr
							v.CgroupFSStats[cgroupFSKey].Aggr = FSStat.Curr
							v.CgroupFSStats[cgroupFSKey].Curr = val
						}
					}

					if v.CurrEnergyInCore > 0 {
						log.Printf("\tenergy from pod: name: %s namespace: %s \n"+
							"\teCore: %d(%d) eDram: %d(%d) eOther: %d(%d) eGPU: %d(%d) \n"+
							"\tCPUTime: %.2f (%.4f) \n"+
							"\tcycles: %d (%.4f) \n"+
							"\tinstructions: %d (%.4f) \n"+
							"\tDiskReadBytes: %d (%d) \n"+
							"\tDiskWriteBytes: %d (%d) \n"+
							"\tmisses: %d (%.4f)\tResidentMemRatio: %.4f\n"+
							"\tavgCPUFreq: %.4f MHZ\n"+
							"\tpid: %v comm: %v\n"+
							"\tcgroupfs: %v\n",
							podName, v.Namespace,
							v.CurrEnergyInCore, v.AggEnergyInCore, v.CurrEnergyInDram, v.AggEnergyInDram, v.CurrEnergyInOther, v.AggEnergyInOther, v.CurrEnergyInGPU, v.AggEnergyInGPU,
							v.CurrCPUTime, float64(v.CurrCPUTime)/float64(aggCPUTime),
							v.CurrCPUCycles, float64(v.CurrCPUCycles)/float64(aggCPUCycles),
							v.CurrCPUInstr, float64(v.CurrCPUInstr)/float64(aggCPUInstr),
							v.CurrBytesRead, v.AggBytesRead,
							v.CurrBytesRead, v.AggBytesWrite,
							v.CurrCacheMisses, float64(v.CurrCacheMisses)/float64(aggCacheMisses), float64(v.CurrResidentMem)/nodeMem,
							v.AvgCPUFreq/1000, /*MHZ*/
							v.PID, v.Command,
							v.CgroupFSStats)
					}
				}
				samples = 1
				lock.Unlock()
			}
		}
	}()
}

// getAVGCPUFreqAndTotalCPUTime calculates the weighted cpu frequency average
func getAVGCPUFreqAndTotalCPUTime(cpuFrequency map[int32]uint64, cpuTime [C.CPU_VECTOR_SIZE]uint16) (float64, float64, []int32) {
	var activeCPUs []int32
	totalFreq := float64(0)
	totalCPUTime := float64(0)
	totalFreqWithoutWeight := float64(0)
	for cpu, freq := range cpuFrequency {
		if int(cpu) > len(cpuTime)-1 {
			break
		}
		if cpuTime[cpu] != 0 {
			totalFreq += float64(freq) * float64(cpuTime[cpu])
			totalCPUTime += float64(cpuTime[cpu])
			activeCPUs = append(activeCPUs, cpu)
		}
		totalFreqWithoutWeight += float64(freq)
	}
	avgFreq := float64(totalFreq / totalCPUTime)
	if totalCPUTime == 0 {
		avgFreq = totalFreqWithoutWeight / float64(len(cpuFrequency))
	}
	return avgFreq, totalCPUTime, activeCPUs
}

// getActiveCPUs returns active cpu(vcpu) (in case that frequency is not active)
func getActiveCPUs(cpuTime [C.CPU_VECTOR_SIZE]uint16) []int32 {
	var activeCPUs []int32
	for cpu, _ := range cpuTime {
		if cpuTime[cpu] != 0 {
			activeCPUs = append(activeCPUs, int32(cpu))
		}
	}
	return activeCPUs
}