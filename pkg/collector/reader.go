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
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/sustainable-computing-io/kepler/pkg/pod_lister"
	"github.com/sustainable-computing-io/kepler/pkg/power"
)

import "C"

//TODO in sync with bpf program
type CgroupTime struct {
	CGroupPID             uint64
	PID                   uint64
	Time                  uint64
	CPUCycles             uint64
	CPUInstr              uint64
	CacheMisses           uint64
	StartTime             uint64
	LastAvgFreqUpdateTime uint64
	AvgFreq               uint32
	LastFreq              uint32
	Command               [16]byte
}

type PodEnergy struct {
	CGroupPID uint64
	PID       uint64
	PodName   string
	Namespace string
	Command   string

	AggCPUTime     uint64
	AggCPUCycles   uint64
	AggCPUInstr    uint64
	AggCacheMisses uint64

	CurrCPUTime     uint64
	CurrCPUCycles   uint64
	CurrCPUInstr    uint64
	CurrCacheMisses uint64

	CurrEnergyInCore uint64
	CurrEnergyInDram uint64
	AggEnergyInCore  uint64
	AggEnergyInDram  uint64

	AvgCPUFreq  uint32
	LastCPUFreq uint32
}

const (
	samplePeriod = 3000 * time.Millisecond
)

var (
	podEnergy = map[string]*PodEnergy{}
	lock      sync.Mutex
)

func (c *Collector) reader() {
	ticker := time.NewTicker(samplePeriod)
	go func() {
		var ct CgroupTime
		lastEnergyCore, _ := power.GetEnergyFromCore()
		lastEnergyDram, _ := power.GetEnergyFromDram()

		for {
			select {
			case <-ticker.C:
				var aggCPUTime, aggCPUCycles, aggCacheMisses uint64
				energyCore, err := power.GetEnergyFromCore()
				if err != nil {
					log.Printf("failed to get core power: %v\n", err)
					continue
				}
				energyDram, err := power.GetEnergyFromDram()
				if err != nil {
					log.Printf("failed to get dram power: %v\n", err)
					continue
				}
				coreDelta := uint64(energyCore - lastEnergyCore)
				dramDelta := uint64(energyDram - lastEnergyDram)
				if coreDelta == 0 {
					coreDelta = 1
				}
				lastEnergyCore = energyCore
				lastEnergyDram = energyDram

				lock.Lock()

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
					// to prevent overflow of the counts we change the unit to have smaller numbers
					val := ct.Time / 1000 /*miliseconds*/
					podEnergy[podName].CurrCPUTime = val
					podEnergy[podName].AggCPUTime += val
					val = ct.CPUCycles / 1000
					podEnergy[podName].CurrCPUCycles = val
					podEnergy[podName].AggCPUCycles += val
					val = ct.CPUInstr / 1000
					podEnergy[podName].CurrCPUInstr = val
					podEnergy[podName].AggCPUInstr += val
					val = ct.CacheMisses / 1000
					podEnergy[podName].CurrCacheMisses = val
					podEnergy[podName].AggCacheMisses += val
					podEnergy[podName].AvgCPUFreq = ct.AvgFreq
					podEnergy[podName].LastCPUFreq = ct.LastFreq

					aggCPUTime += podEnergy[podName].CurrCPUTime
					aggCPUCycles += podEnergy[podName].CurrCPUCycles
					aggCacheMisses += podEnergy[podName].CurrCacheMisses
				}
				// reset all counters in the eBPF table
				c.modules.Table.DeleteAll()

				var cyclesPerMW, cacheMissPerMW float64
				if aggCPUCycles > 0 && coreDelta > 0 {
					cyclesPerMW = float64(aggCPUCycles / coreDelta)
				}
				if aggCacheMisses > 0 && dramDelta > 0 {
					cacheMissPerMW = float64(aggCacheMisses / dramDelta)
				}

				log.Printf("energy from: core %v dram: %v time %v cycles %v misses %v\n",
					coreDelta, dramDelta, aggCPUTime, aggCPUCycles, aggCacheMisses)

				for podName, v := range podEnergy {
					v.CurrEnergyInCore = uint64(float64(v.CurrCPUCycles) / cyclesPerMW)
					v.AggEnergyInCore += v.CurrEnergyInCore
					if cacheMissPerMW > 0 {
						v.CurrEnergyInDram = uint64(float64(v.CurrCacheMisses) / cacheMissPerMW)
					}
					v.AggEnergyInDram += v.CurrEnergyInDram
					if podEnergy[podName].CurrCPUTime > 0 {
						log.Printf("\tenergy from pod: name: %s namespace: %s \n\teCore: %d eDram: %d \n\tCPUTime: %d (%f) \n\tcycles: %d (%f) \n\tmisses: %d (%f)\n\tavgCPUFreq: %v LastCPUFreq %v\n\tpid: %v comm: %v\n",
							podName, podEnergy[podName].Namespace, v.AggEnergyInCore, v.CurrEnergyInDram,
							podEnergy[podName].CurrCPUTime, float64(podEnergy[podName].CurrCPUTime)/float64(aggCPUTime),
							podEnergy[podName].CurrCPUCycles, float64(podEnergy[podName].CurrCPUCycles)/float64(aggCPUCycles),
							podEnergy[podName].CurrCacheMisses, float64(podEnergy[podName].CurrCacheMisses)/float64(aggCacheMisses),
							podEnergy[podName].AvgCPUFreq, podEnergy[podName].LastCPUFreq, podEnergy[podName].PID, podEnergy[podName].Command)
					}
				}
				lock.Unlock()
			}
		}
	}()
}
