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
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/sustainable-computing-io/kepler/pkg/pod_lister"
	"github.com/sustainable-computing-io/kepler/pkg/power/acpi"
	"github.com/sustainable-computing-io/kepler/pkg/power/rapl"
)

// #define CPU_VECTOR_SIZE 128
import "C"

//TODO in sync with bpf program
type CgroupTime struct {
	CGroupPID   uint64
	PID         uint64
	CPUCycles   uint64
	CPUInstr    uint64
	CacheMisses uint64
	Command     [16]byte
	CPUTime     [C.CPU_VECTOR_SIZE]uint16
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

	CurrEnergyInCore  uint64
	CurrEnergyInDram  uint64
	CurrEnergyInOther uint64
	AggEnergyInCore   uint64
	AggEnergyInDram   uint64
	AggEnergyInOther  uint64

	AvgCPUFreq float64
}

const (
	samplePeriod = 3000 * time.Millisecond
)

var (
	podEnergy      = map[string]*PodEnergy{}
	nodeEnergy     = map[string]float64{}
	cpuFrequency   = map[int32]uint64{}
	nodeName, _    = os.Hostname()
	acpiPowerMeter = acpi.NewACPIPowerMeter()
	numCPUs        = runtime.NumCPU()
	lock           sync.Mutex
)

func (c *Collector) reader() {
	ticker := time.NewTicker(samplePeriod)
	go func() {
		lastEnergyCore, _ := rapl.GetEnergyFromCore()
		lastEnergyDram, _ := rapl.GetEnergyFromDram()

		acpiPowerMeter.Run()
		for {
			select {
			case <-ticker.C:
				cpuFrequency = acpiPowerMeter.GetCPUCoreFrequency()
				nodeEnergy, _ = acpiPowerMeter.GetEnergyFromHost()
				var aggCPUTime, aggCPUCycles, aggCacheMisses float64
				energyCore, err := rapl.GetEnergyFromCore()
				if err != nil {
					log.Printf("failed to get core power: %v\n", err)
					continue
				}
				energyDram, err := rapl.GetEnergyFromDram()
				if err != nil {
					log.Printf("failed to get dram power: %v\n", err)
					continue
				}
				coreDelta := float64(energyCore - lastEnergyCore)
				dramDelta := float64(energyDram - lastEnergyDram)
				if coreDelta == 0 && dramDelta == 0 {
					log.Printf("power reading not changed, retry\n")
					continue
				}
				lastEnergyCore = energyCore
				lastEnergyDram = energyDram

				// calculate the total energy consumed in node from all sensors
				var nodeEnergyTotal float64
				for _, energy := range nodeEnergy {
					nodeEnergyTotal += energy
				}
				// ccalculate the other energy consumed besides CPU and memory
				otherDelta := float64(0)
				if nodeEnergyTotal > 0 {
					otherDelta = nodeEnergyTotal - coreDelta - dramDelta
				}

				lock.Lock()
				var ct CgroupTime
				for it := c.modules.Table.Iter(); it.Next(); {
					data := it.Leaf()
					err := binary.Read(bytes.NewBuffer(data), binary.LittleEndian, &ct)
					if err != nil {
						log.Printf("failed to decode received data: %v", err)
						continue
					}

					podName, err := pod_lister.GetPodNameFromcGgroupID(ct.CGroupPID)
					if err != nil {
						log.Printf("failed to resolve pod for cGroup ID %v: %v", ct.CGroupPID, err)
						continue
					}

					// podName is used as Prometheus desc name, normalize it
					podName = strings.Replace(podName, "-", "_", -1)
					if _, ok := podEnergy[podName]; !ok {
						podEnergy[podName] = &PodEnergy{}
						podEnergy[podName].PodName = podName
						podNamespace, err := pod_lister.GetPodNameSpaceFromcGgroupID(ct.CGroupPID)
						if err != nil {
							log.Printf("failed to find namespace for cGroup ID %v: %v", ct.CGroupPID, err)
							podNamespace = "unknown"
						}
						podEnergy[podName].Namespace = podNamespace
						comm := (*C.char)(unsafe.Pointer(&ct.Command))
						podEnergy[podName].CGroupPID = ct.CGroupPID
						podEnergy[podName].PID = ct.PID
						podEnergy[podName].Command = C.GoString(comm)
					}
					avgFreq, totalCPUTime := getAVGCPUFreqAndTotalCPUTime(cpuFrequency, ct.CPUTime)
					// to prevent overflow of the counts we change the unit to have smaller numbers
					totalCPUTime = totalCPUTime / 1000 /*seconds*/
					podEnergy[podName].CurrCPUTime = totalCPUTime
					podEnergy[podName].AggCPUTime += podEnergy[podName].CurrCPUTime
					val := ct.CPUCycles
					podEnergy[podName].CurrCPUCycles = val
					podEnergy[podName].AggCPUCycles += val
					val = ct.CPUInstr
					podEnergy[podName].CurrCPUInstr = val
					podEnergy[podName].AggCPUInstr += val
					val = ct.CacheMisses
					podEnergy[podName].CurrCacheMisses = val
					podEnergy[podName].AggCacheMisses += val
					avgFreq = avgFreq
					podEnergy[podName].AvgCPUFreq = avgFreq

					aggCPUTime += podEnergy[podName].CurrCPUTime
					aggCPUCycles += float64(podEnergy[podName].CurrCPUCycles)
					aggCacheMisses += float64(podEnergy[podName].CurrCacheMisses)
				}
				// reset all counters in the eBPF table
				c.modules.Table.DeleteAll()

				var cyclesPerMJ, cacheMissPerMJ, perProcessOtherMJ float64
				if aggCPUCycles > 0 && coreDelta > 0 {
					cyclesPerMJ = aggCPUCycles / coreDelta
				}
				if aggCacheMisses > 0 && dramDelta > 0 {
					cacheMissPerMJ = aggCacheMisses / dramDelta
				}
				perProcessOtherMJ = otherDelta / float64(len(podEnergy))

				log.Printf("energy from: core %v dram: %v time %v cycles %v misses %v\n",
					coreDelta, dramDelta, aggCPUTime, aggCPUCycles, aggCacheMisses)

				for podName, v := range podEnergy {
					v.CurrEnergyInCore = uint64(float64(v.CurrCPUCycles) / cyclesPerMJ)
					v.AggEnergyInCore += v.CurrEnergyInCore
					if cacheMissPerMJ > 0 {
						v.CurrEnergyInDram = uint64(float64(v.CurrCacheMisses) / cacheMissPerMJ)
					}
					v.AggEnergyInDram += v.CurrEnergyInDram
					v.CurrEnergyInOther = uint64(perProcessOtherMJ)
					v.AggEnergyInOther += uint64(perProcessOtherMJ)
					if podEnergy[podName].CurrCPUTime > 0 {
						log.Printf("\tenergy from pod: name: %s namespace: %s \n\teCore: %d eDram: %d eOther: %d \n\tCPUTime: %.6f (%f) \n\tcycles: %d (%f) \n\tmisses: %d (%f)\n\tavgCPUFreq: %.4f MHZ\n\tpid: %v comm: %v\n",
							podName, podEnergy[podName].Namespace, v.AggEnergyInCore, v.CurrEnergyInDram, v.CurrEnergyInOther,
							podEnergy[podName].CurrCPUTime, float64(podEnergy[podName].CurrCPUTime)/float64(aggCPUTime),
							podEnergy[podName].CurrCPUCycles, float64(podEnergy[podName].CurrCPUCycles)/float64(aggCPUCycles),
							podEnergy[podName].CurrCacheMisses, float64(podEnergy[podName].CurrCacheMisses)/float64(aggCacheMisses),
							podEnergy[podName].AvgCPUFreq/1000, /*MHZ*/
							podEnergy[podName].PID, podEnergy[podName].Command)
					}
				}
				lock.Unlock()
			}
		}
	}()
}

// getAVGCPUFreqAndTotalCPUTime calculates the weighted cpu frequency average
func getAVGCPUFreqAndTotalCPUTime(cpuFrequency map[int32]uint64, cpuTime [C.CPU_VECTOR_SIZE]uint16) (float64, float64) {
	totalFreq := float64(0)
	totalCPUTime := float64(0)
	for cpu, freq := range cpuFrequency {
		if cpuTime[cpu] != 0 {
			totalFreq += float64(freq) * float64(cpuTime[cpu])
			totalCPUTime += float64(cpuTime[cpu])
		}
	}
	avgFreq := totalFreq / totalCPUTime
	return avgFreq, totalCPUTime
}
