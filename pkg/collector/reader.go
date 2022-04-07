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
	CgroupID    uint64
	Time        uint64
	CPUCycles   uint64
	CPUInstr    uint64
	CacheMisses uint64
	Command     [16]byte
}

type PodEnergy struct {
	PodName   string
	Namespace string
	Command   string

	CPUTime     uint64
	CPUCycles   uint64
	CPUInstr    uint64
	CacheMisses uint64

	CurrEnergyInCore uint64
	CurrEnergyInDram uint64
	AggEnergyInCore  uint64
	AggEnergyInDram  uint64
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
				energyCore, _ := power.GetEnergyFromCore()
				energyDram, _ := power.GetEnergyFromDram()
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
						log.Printf("failed to decode received data: %s", err)
						continue
					}

					podName, _ := pod_lister.GetPodNameFromPID(ct.CgroupID)
					// podName is used as Prometheus desc name, normalize it
					podName = strings.Replace(podName, "-", "_", -1)
					if _, ok := podEnergy[podName]; !ok {
						podEnergy[podName] = &PodEnergy{}
						podEnergy[podName].PodName = podName
						podNamespace, _ := pod_lister.GetPodNameSpaceFromPID(ct.CgroupID)
						podEnergy[podName].Namespace = podNamespace
						comm := (*C.char)(unsafe.Pointer(&ct.Command))
						podEnergy[podName].Command = C.GoString(comm)
					}
					// to prevent overflow of the counts we change the unit to have smaller numbers
					podEnergy[podName].CPUTime += ct.Time / 1000 /*miliseconds*/
					podEnergy[podName].CPUCycles += ct.CPUCycles / 1000
					podEnergy[podName].CPUInstr += ct.CPUInstr / 1000
					podEnergy[podName].CacheMisses += ct.CacheMisses / 1000

					aggCPUTime += podEnergy[podName].CPUTime
					aggCPUCycles += podEnergy[podName].CPUCycles
					aggCacheMisses += podEnergy[podName].CacheMisses
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

				log.Printf("energy from: core %d dram: %d time %d cycles %d misses %d\n",
					coreDelta, dramDelta, aggCPUTime, aggCPUCycles, aggCacheMisses)

				for podName, v := range podEnergy {
					v.CurrEnergyInCore = uint64(float64(v.CPUCycles) / cyclesPerMW)
					v.AggEnergyInCore += v.CurrEnergyInCore
					if cacheMissPerMW > 0 {
						v.CurrEnergyInDram = uint64(float64(v.CacheMisses) / cacheMissPerMW)
					}
					v.AggEnergyInDram += v.CurrEnergyInDram
					if podEnergy[podName].CPUTime > 0 {
						log.Printf("\tenergy from pod: name: %s \n\teCore: %d eDram: %d \n\tcurraggCPUTime: %d (%f) \n\tcycles: %d (%f) \n\tmisses: %d (%f)\n",
							podName, v.AggEnergyInCore, v.CurrEnergyInDram,
							podEnergy[podName].CPUTime, float64(podEnergy[podName].CPUTime)/float64(aggCPUTime),
							podEnergy[podName].CPUCycles, float64(podEnergy[podName].CPUCycles)/float64(aggCPUCycles),
							podEnergy[podName].CacheMisses, float64(podEnergy[podName].CacheMisses)/float64(aggCacheMisses))
					}
				}
				lock.Unlock()
			}
		}
	}()
}
