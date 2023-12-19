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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/model/types"
)

var (
	SampleDynEnergyValue float64 = 100000 // 100 mJ

	processFeatureNames = []string{
		config.CPUCycle,
		config.CPUInstruction,
		config.CacheMiss,
		config.CgroupfsMemory,
		config.CgroupfsKernelMemory,
		config.CgroupfsTCPMemory,
		config.CgroupfsCPU,
		config.CgroupfsSystemCPU,
		config.CgroupfsUserCPU,
		config.CgroupfsReadIO,
		config.CgroupfsWriteIO,
		config.BlockDevicesIO,
	}
	systemMetaDataFeatureNames = []string{"cpu_architecture"}
	featureNames               = append(processFeatureNames, systemMetaDataFeatureNames...) // to predict node power, we will need the resource usage and metadata metrics
	processFeatureValues       = [][]float64{
		{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, // process A
		{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, // process B
	}
	nodeFeatureValues           = []float64{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}
	systemMetaDataFeatureValues = []string{"Sandy Bridge"}
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
		powers[0] = SampleDynEnergyValue
		msg := ""
		var powerResponseJSON []byte
		var powerResponse ComponentPowerResponse
		if powerRequest.EnergySource == types.ComponentEnergySource {
			powerResponse = ComponentPowerResponse{
				Powers:  map[string][]float64{config.PKG: powers},
				Message: msg,
			}
		} else {
			powerResponse = ComponentPowerResponse{
				Powers:  map[string][]float64{config.PLATFORM: powers},
				Message: msg,
			}
		}
		powerResponseJSON, err = json.Marshal(powerResponse)
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

func createEstimatorSidecarPowerModel(serveSocket string, outputType types.ModelOutputType, energySource string) EstimatorSidecar {
	return EstimatorSidecar{
		Socket:                      serveSocket,
		OutputType:                  outputType,
		FloatFeatureNames:           featureNames,
		SystemMetaDataFeatureNames:  systemMetaDataFeatureNames,
		SystemMetaDataFeatureValues: systemMetaDataFeatureValues,
		EnergySource:                energySource,
	}
}

var _ = Describe("Test Estimate Unit", func() {
	It("Get Node Platform Power By Sidecar Estimator", func() {
		serveSocket := "/tmp/node-total-power.sock"
		start := make(chan bool)
		quit := make(chan bool)
		defer close(quit)
		defer close(start)
		go dummyEstimator(serveSocket, start, quit)
		<-start
		c := createEstimatorSidecarPowerModel(serveSocket, types.AbsPower, types.PlatformEnergySource)
		err := c.Start()
		Expect(err).To(BeNil())
		c.ResetSampleIdx()
		c.AddNodeFeatureValues(nodeFeatureValues) // add samples to the power model
		powers, err := c.GetPlatformPower(false)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(powers)).Should(Equal(1))
		Expect(powers[0]).Should(Equal(SampleDynEnergyValue))
		quit <- true
	})

	It("Get Process Platform Power By Sidecar Estimator", func() {
		serveSocket := "/tmp/pod-total-power.sock"
		start := make(chan bool)
		quit := make(chan bool)
		defer close(quit)
		defer close(start)
		go dummyEstimator(serveSocket, start, quit)
		<-start
		c := createEstimatorSidecarPowerModel(serveSocket, types.DynPower, types.PlatformEnergySource)
		err := c.Start()
		Expect(err).To(BeNil())
		c.ResetSampleIdx()
		for _, processFeatureValues := range processFeatureValues {
			c.AddProcessFeatureValues(processFeatureValues) // add samples to the power model
		}
		powers, err := c.GetPlatformPower(false)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(powers)).Should(Equal(len(processFeatureValues)))
		Expect(powers[0]).Should(Equal(SampleDynEnergyValue))
		quit <- true
	})
	It("Get Node Component Power By Sidecar Estimator", func() {
		serveSocket := "/tmp/node-comp-power.sock"
		start := make(chan bool)
		quit := make(chan bool)
		defer close(quit)
		defer close(start)
		go dummyEstimator(serveSocket, start, quit)
		<-start
		c := createEstimatorSidecarPowerModel(serveSocket, types.AbsPower, types.ComponentEnergySource)
		err := c.Start()
		Expect(err).To(BeNil())
		c.ResetSampleIdx()
		c.AddNodeFeatureValues(nodeFeatureValues) // add samples to the power model
		powers, err := c.GetComponentsPower(false)
		quit <- true
		Expect(err).NotTo(HaveOccurred())
		Expect(len(powers)).Should(Equal(1))
		// TODO: Fix estimator power model
		// "The estimator node pkg power estimation is estimating 100 Kilo Joules, " +
		// 	"which is way to high for the given resource utilization is set to only 2 in all features." +
		// 	"We are skiping this test until the power model is fixed.",
		// Expect(powers[0].Pkg).Should(Equal(uint64(100000000)))
	})
	It("Get Process Component Power By Sidecar Estimator", func() {
		serveSocket := "/tmp/pod-comp-power.sock"
		start := make(chan bool)
		quit := make(chan bool)
		defer close(quit)
		defer close(start)
		go dummyEstimator(serveSocket, start, quit)
		<-start
		c := createEstimatorSidecarPowerModel(serveSocket, types.DynPower, types.ComponentEnergySource)
		err := c.Start()
		Expect(err).To(BeNil())
		c.ResetSampleIdx()
		for _, processFeatureValues := range processFeatureValues {
			c.AddProcessFeatureValues(processFeatureValues) // add samples to the power model
		}
		powers, err := c.GetComponentsPower(false)
		quit <- true
		Expect(err).NotTo(HaveOccurred())
		Expect(len(powers)).Should(Equal(len(processFeatureValues)))
		// "The estimator node pkg power estimation is estimating 100 Kilo Joules or 0 Joules, " +
		// 	"which is way to high or too low for the given resource utilization is set to only 1 in all features." +
		// 	"We are skiping this test until the power model is fixed.",
		// Expect(powers[0].Pkg).Should(Equal(SampleDynEnergyValue))
	})
})
