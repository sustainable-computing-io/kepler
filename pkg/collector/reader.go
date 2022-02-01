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
	"sync"
	"time"
	"unsafe"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/sustainable-computing-io/kepler/pkg/pod_lister"
	"github.com/sustainable-computing-io/kepler/pkg/rapl"
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
	Pod, Namespace, Command string

	CPUTime     uint64
	LastCPUTime uint64

	CPUCycles       uint64
	CPUInstr        uint64
	CacheMisses     uint64
	LastCPUCycles   uint64
	LastCPUInstr    uint64
	LastCacheMisses uint64

	EnergyInCore     int
	EnergyInDram     int
	LastEnergyInCore int
	LastEnergyInDram int

	desc *prometheus.Desc
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
		lastEnergyCore, _ := rapl.GetEnergyFromCore()
		lastEnergyDram, _ := rapl.GetEnergyFromDram()
		var lastCPUTime, lastCPUCycles, lastCacheMisses, cpuTime, cpuCycles, cacheMisses uint64

		for {
			select {
			case <-ticker.C:
				energyCore, _ := rapl.GetEnergyFromCore()
				energyDram, _ := rapl.GetEnergyFromDram()
				coreDiff := energyCore - lastEnergyCore
				dramDiff := energyDram - lastEnergyDram
				lastEnergyCore = energyCore
				lastEnergyDram = energyDram
				cpuTime = 0
				cpuCycles = 0
				cacheMisses = 0

				lock.Lock()

				for it := c.modules.Table.Iter(); it.Next(); {
					data := it.Leaf()
					err := binary.Read(bytes.NewBuffer(data), binary.LittleEndian, &ct)
					if err != nil {
						log.Printf("failed to decode received data: %s", err)
						continue
					}
					if ct.CgroupID != 0 {
						path, err := pod_lister.CgroupIdToName(ct.CgroupID)
						if err != nil {
							log.Printf("failed to get cgroup path %v for %v", err, ct.CgroupID)
							continue
						}

						comm := (*C.char)(unsafe.Pointer(&ct.Command))

						meta, err := pod_lister.CgroupToPod(path)
						podName := "system processes"
						if meta != nil && err == nil {
							podName = meta.Namespace + "/" + meta.Name
						}
						if _, ok := podEnergy[podName]; !ok {
							podEnergy[podName] = &PodEnergy{}
							if meta != nil && err == nil {
								podEnergy[podName].Pod = meta.Name
								podEnergy[podName].Namespace = meta.Namespace
							} else {
								podEnergy[podName].Pod = podName
								podEnergy[podName].Namespace = "system"
							}
						}
						podEnergy[podName].LastCPUTime = podEnergy[podName].CPUTime
						podEnergy[podName].CPUTime = ct.Time - podEnergy[podName].CPUTime

						podEnergy[podName].LastCPUCycles = podEnergy[podName].CPUCycles
						podEnergy[podName].CPUCycles = ct.CPUCycles - podEnergy[podName].CPUCycles

						podEnergy[podName].LastCPUInstr = podEnergy[podName].CPUInstr
						podEnergy[podName].CPUInstr = ct.CPUInstr - podEnergy[podName].CPUInstr

						podEnergy[podName].LastCacheMisses = podEnergy[podName].CacheMisses
						podEnergy[podName].CacheMisses = ct.CacheMisses - podEnergy[podName].CacheMisses

						podEnergy[podName].Command = C.GoString(comm)

						cpuTime += ct.Time
						cpuCycles += ct.CPUCycles
						cacheMisses += ct.CacheMisses
					}
				}
				cpuTimeDiff := cpuTime - lastCPUTime
				cpuCyclesDiff := cpuCycles - lastCPUCycles
				cacheMissesDiff := cacheMisses - lastCacheMisses
				lastCPUCycles = cpuCycles
				lastCPUTime = cpuTime
				lastCacheMisses = cacheMisses
				energyPerCycle := coreDiff / int(cpuCyclesDiff)
				energyPerCacheMiss := 0
				if cacheMissesDiff > 0 {
					energyPerCacheMiss = dramDiff / int(cacheMissesDiff)
				}

				log.Printf("energy from: core %d dram: %d time %d cycles %d misses %d\n", coreDiff, dramDiff, cpuTimeDiff, cpuCyclesDiff, cacheMissesDiff)

				for _, v := range podEnergy {
					v.LastEnergyInCore = v.EnergyInCore
					v.EnergyInCore = int(v.CPUCycles) * energyPerCycle
					v.LastEnergyInDram = v.EnergyInDram
					v.EnergyInDram = int(v.CacheMisses) * energyPerCacheMiss
				}
				lock.Unlock()
			}
		}
	}()
}
