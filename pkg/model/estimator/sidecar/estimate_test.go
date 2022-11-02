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

package sidecar

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sustainable-computing-io/kepler/pkg/model/types"
)

var (
	SampleDynPowerValue float64 = 100.0

	usageMetrics   = []string{"bytes_read", "bytes_writes", "cache_miss", "cgroupfs_cpu_usage_us", "cgroupfs_memory_usage_bytes", "cgroupfs_system_cpu_usage_us", "cgroupfs_user_cpu_usage_us", "cpu_cycles", "cpu_instr", "cpu_time"}
	systemFeatures = []string{"cpu_architecture"}
	usageValues    = [][]float64{{1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, {1, 1, 1, 1, 1, 1, 1, 1, 1, 1}}
	nodeUsageValue = usageValues[0]
	systemValues   = []string{"Sandy Bridge"}
)

func dummyEstimator(serveSocket string, start, quit chan bool) {
	cleanup := func() {
		if _, err := os.Stat(serveSocket); err == nil {
			if err := os.RemoveAll(serveSocket); err != nil {
				panic(err)
			}
		}
	}
	cleanup()

	listener, err := net.Listen("unix", serveSocket)
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	go func() {
		<-quit
		cleanup()
	}()

	start <- true

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("Close dummy estimator %v\n", err)
			break
		}
		buf := make([]byte, 4096)
		n, err := conn.Read(buf)
		if err != nil {
			panic(err)
		}
		var powerRequest PowerRequest
		err = json.Unmarshal(buf[0:n], &powerRequest)
		if err != nil {
			panic(err)
		}
		fmt.Printf("%v\n", powerRequest)
		powers := make([]float64, len(powerRequest.UsageValues))
		powers[0] = SampleDynPowerValue
		msg := ""
		var powerResponseJSON []byte
		if strings.Contains(powerRequest.OutputType, "Component") {
			powerResponse := ComponentPowerResponse{
				Powers:  map[string][]float64{"pkg": powers},
				Message: msg,
			}
			powerResponseJSON, err = json.Marshal(powerResponse)
		} else {
			powerResponse := TotalPowerResponse{
				Powers:  powers,
				Message: msg,
			}
			powerResponseJSON, err = json.Marshal(powerResponse)
		}
		if err != nil {
			panic(err)
		}
		_, err = conn.Write(powerResponseJSON)
		if err != nil {
			panic(err)
		}
		conn.Close()
	}
}

func genEstimatorSidecarConnector(serveSocket string, outputType types.ModelOutputType) EstimatorSidecarConnector {
	return EstimatorSidecarConnector{
		Socket:         serveSocket,
		UsageMetrics:   usageMetrics,
		OutputType:     outputType,
		SystemFeatures: systemFeatures,
	}
}

var _ = Describe("Test Estimate Unit", func() {
	It("GetNodeTotalPowerByEstimator", func() {
		serveSocket := "/tmp/node-total-power.sock"
		start := make(chan bool)
		quit := make(chan bool)
		defer close(quit)
		defer close(start)
		go dummyEstimator(serveSocket, start, quit)
		<-start

		c := genEstimatorSidecarConnector(serveSocket, types.AbsPower)
		valid := c.Init(systemValues)
		Expect(valid).To(Equal(true))
		powers, err := c.GetTotalPower([][]float64{nodeUsageValue}, systemValues)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(powers)).Should(Equal(1))
		Expect(powers[0]).Should(Equal(SampleDynPowerValue))
		quit <- true
	})
	It("GetPodTotalPowerByEstimator", func() {
		serveSocket := "/tmp/pod-total-power.sock"
		start := make(chan bool)
		quit := make(chan bool)
		defer close(quit)
		defer close(start)
		go dummyEstimator(serveSocket, start, quit)
		<-start

		c := genEstimatorSidecarConnector(serveSocket, types.DynPower)
		valid := c.Init(systemValues)
		Expect(valid).To(Equal(true))
		powers, err := c.GetTotalPower(usageValues, systemValues)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(powers)).Should(Equal(len(usageValues)))
		Expect(powers[0]).Should(Equal(SampleDynPowerValue))
		quit <- true
	})
	It("GetNodeComponentPowerByEstimator", func() {
		serveSocket := "/tmp/node-comp-power.sock"
		start := make(chan bool)
		quit := make(chan bool)
		defer close(quit)
		defer close(start)
		go dummyEstimator(serveSocket, start, quit)
		<-start

		c := genEstimatorSidecarConnector(serveSocket, types.AbsComponentPower)
		valid := c.Init(systemValues)
		Expect(valid).To(Equal(true))
		powers, err := c.GetComponentPower([][]float64{nodeUsageValue}, systemValues)
		Expect(err).NotTo(HaveOccurred())
		pkgPowers, ok := powers["pkg"]
		Expect(ok).To(Equal(true))
		Expect(len(pkgPowers)).Should(Equal(1))
		Expect(pkgPowers[0]).Should(Equal(SampleDynPowerValue))
		quit <- true
	})
	It("GetPodComponentPowerByEstimator", func() {
		serveSocket := "/tmp/pod-comp-power.sock"
		start := make(chan bool)
		quit := make(chan bool)
		defer close(quit)
		defer close(start)
		go dummyEstimator(serveSocket, start, quit)
		<-start

		c := genEstimatorSidecarConnector(serveSocket, types.DynComponentPower)
		valid := c.Init(systemValues)
		Expect(valid).To(Equal(true))
		powers, err := c.GetComponentPower(usageValues, systemValues)
		Expect(err).NotTo(HaveOccurred())
		pkgPowers, ok := powers["pkg"]
		Expect(ok).To(Equal(true))
		Expect(len(pkgPowers)).Should(Equal(len(usageValues)))
		Expect(pkgPowers[0]).Should(Equal(SampleDynPowerValue))
		quit <- true
	})
})
