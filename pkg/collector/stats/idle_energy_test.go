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

package stats

import (
	"math"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sustainable-computing-io/kepler/pkg/config"
)

var _ = Describe("Test IdleEnergyCalculator", func() {
	var (
		ic                     *IdleEnergyCalculator
		maxTheoreticalCPUTime  float64 = 100.0
		minSpread              float64 = 0.5
		historySize            int     = 10
		minSlope               float64 = 0.01
		minYIntercept          float64 = 0.0
		initialMinUtilizationX float64 = 10.0
		initialMinUtilizationY float64 = 15.0
		initialMaxUtilizationX float64 = 20.0
		initialMaxUtilizationY float64 = 25.0
		initialSlope           float64 = 17 / 15
		initialYIntercept      float64 = 5.0
		initialSpread          float64 = 0.1
	)
	BeforeEach(func() {
		// Initialize IdleEnergyCalculator with default starter values
		ic = NewIdleEnergyCalculator(IdleEnergyCalculatorInitialization{
			InitialUtilizationX: initialMinUtilizationX,
			InitialUtilizationY: initialMinUtilizationY,
			Config: IdleEnergyConfig{
				MinSpread:    minSpread,
				MinIntercept: minYIntercept,
				MinSlope:     minSlope,
				HistorySize:  historySize,
			},
		})
		// note a very rare case to test which should not be a problem is what happens if we get the same cpu time twice
		// at the very beginning! should this include a test?
		ic.UpdateIdleEnergy(NewEnergyPoint{
			NewResUtil:            initialMaxUtilizationX,
			NewEnergyDelta:        initialMaxUtilizationY,
			maxTheoreticalCPUTime: maxTheoreticalCPUTime,
		})
		// Guarantee Update is successful
		Expect(ic.Result.CalculatedIdleEnergy).To(Equal(initialYIntercept))
		Expect(ic.Result.Slope).To(Equal(initialSlope))
		Expect(ic.Result.Spread).To(Equal(initialSpread))
		Expect(ic.Result.History).To(Equal([]float64{initialYIntercept}))
		Expect(ic.MinUtilization.X).To(Equal(initialMinUtilizationX))
		Expect(ic.MinUtilization.Y).To(Equal(initialMinUtilizationY))
		Expect(ic.MaxUtilization.X).To(Equal(initialMaxUtilizationX))
		Expect(ic.MaxUtilization.Y).To(Equal(initialMaxUtilizationY))
	})

	Describe("Test UpdateIdleEnergy", func() {
		DescribeTable("with different data points",
			func(
				newResUtilization float64, newEnergyDelta float64,
				expectedMinX float64, expectedMinY float64,
				expectedMaxX float64, expectedMaxY float64,
				expectedYIntercept float64,
				expectedSlope float64,
				expectedSpread float64,
				expectedHistory []float64,
			) {
				// Execute the update
				ic.UpdateIdleEnergy(NewEnergyPoint{
					newResUtilization,
					newEnergyDelta,
					maxTheoreticalCPUTime,
				})

				// Verify utilization value updates
				Expect(ic.MinUtilization.X).To(Equal(expectedMinX))
				Expect(ic.MinUtilization.Y).To(Equal(expectedMinY))
				Expect(ic.MaxUtilization.X).To(Equal(expectedMaxX))
				Expect(ic.MaxUtilization.Y).To(Equal(expectedMaxY))

				// Verify slope, y intercept values, spread, and history calculations
				idleEnergy := ic.Result.CalculatedIdleEnergy
				spread := ic.Result.Spread
				slope := ic.Result.Slope
				history := ic.Result.History

				Expect(idleEnergy).To(BeNumerically("~", expectedYIntercept, 1e-9))
				Expect(slope).To(BeNumerically("~", expectedSlope, 1e-9))
				Expect(spread).To(BeNumerically("~", expectedSpread, 1e-9))
				Expect(history).To(HaveLen(len(expectedHistory)))
				for i := range expectedHistory {
					Expect(history[i]).To(BeNumerically("~", expectedHistory[i], 1e-9))
				}

			},
			Entry("excess data point (no update)",
				15.0, 10.0,
				initialMinUtilizationX, initialMinUtilizationY, // Expected min X/Y (unchanged)
				initialMaxUtilizationX, initialMaxUtilizationY, // Expected max X/Y (unchanged)
				initialYIntercept, // Expected Y Intercept (unchanged)
				initialSlope,      // Expected Slope (unchanged)
				initialSpread,     // Expected Spread (unchanged)
				[]float64{initialYIntercept, initialYIntercept}, // Expected History (unchanged)

			),
			Entry("new minimum utilization (change min X/Y)",
				5.0, 8.0,
				5.0, 8.0, // Expected min X/Y
				initialMaxUtilizationX, initialMaxUtilizationY, // Expected max X/Y (unchanged)
				7.0/3.0,                                 // Expected Y Intercept
				17.0/15.0,                               // Expected Slope
				0.15,                                    // Expected Spread
				[]float64{initialYIntercept, 7.0 / 3.0}, // Expected History
			),
			Entry("new maximum utilization (change max X/Y)",
				25.0, 20.0,
				initialMinUtilizationX, initialMinUtilizationY, // Expected min X/Y (unchanged)
				25.0, 20.0, // Expected max X/Y
				35.0/3.0,                                 // Expected Y Intercept
				1.0/3.0,                                  // Expected Slope
				0.15,                                     // Expected Spread
				[]float64{initialYIntercept, 35.0 / 3.0}, // Expected History
			),
			Entry("same minimum utilization (update min Y)",
				10.0, 14.0,
				10.0, 14.0, // Expected min X/Y
				initialMaxUtilizationX, initialMaxUtilizationY, // Expected max X/Y (unchanged)
				3.0,                 // Expected Y Intercept
				1.1,                 // Expected Slope
				0.1,                 // Expected Spread
				[]float64{5.0, 3.0}, // Expected History
			),
			Entry("same minimum utilization (no update min Y)",
				10.0, 16.0, // Inputs
				initialMinUtilizationX, initialMinUtilizationY, // Expected min X/Y (unchanged)
				initialMaxUtilizationX, initialMaxUtilizationY, // Expected max X/Y (unchanged)
				initialYIntercept, // Expected Y Intercept (unchanged)
				initialSlope,      // Expected Slope (unchanged)
				initialSpread,     // Expected Spread (unchanged)
				[]float64{initialYIntercept, initialYIntercept}, // Expected History (unchanged)
			),
			Entry("same maximum utilization (update max Y)",
				20.0, 20.0,
				initialMinUtilizationX, initialMinUtilizationY, // Expected min X/Y (unchanged)
				20.0, 20.0, // Expected max X/Y
				10.0,                               // Expected Y Intercept
				0.5,                                // Expected Slope
				0.1,                                // Expected Spread
				[]float64{initialYIntercept, 10.0}, // Expected History
			),
			Entry("same minimum utilization (no update max Y)",
				20.0, 26.0,
				initialMinUtilizationX, initialMinUtilizationY, // Expected min X/Y (unchanged)
				initialMaxUtilizationX, initialMaxUtilizationY, // Expected max X/Y (unchanged)
				initialYIntercept, // Expected Y Intercept (unchanged)
				initialSlope,      // Expected Slope (unchanged)
				initialSpread,     // Expected Spread (unchanged)
				[]float64{initialYIntercept, initialYIntercept}, // Expected History
			),
			Entry("y intercept below minYIntercept with same minUtilization (no update)",
				10.0, 5.0,
				initialMinUtilizationX, initialMinUtilizationY, // Expected min X/Y (unchanged)
				initialMaxUtilizationX, initialMaxUtilizationY, // Expected max X/Y (unchanged)
				initialYIntercept, // Expected Y Intercept (unchanged)
				initialSlope,      // Expected Slope (unchanged)
				initialSpread,     // Expected Spread (unchanged)
				[]float64{initialYIntercept, initialYIntercept}, // Expected History
			),
			Entry("y intercept below minYIntercept with lower minUtilization (no update)",
				5.0, 5.0,
				initialMinUtilizationX, initialMinUtilizationY, // Expected min X/Y (unchanged)
				initialMaxUtilizationX, initialMaxUtilizationY, // Expected max X/Y (unchanged)
				initialYIntercept, // Expected Y Intercept (unchanged)
				initialSlope,      // Expected Slope (unchanged)
				initialSpread,     // Expected Spread (unchanged)
				[]float64{initialYIntercept, initialYIntercept}, // Expected History (unchanged)
			),
			Entry("y intercept below minYIntercept with same maxUtilization (no update)",
				20.0, 50.0,
				initialMinUtilizationX, initialMinUtilizationY, // Expected min X/Y (unchanged)
				initialMaxUtilizationX, initialMaxUtilizationY, // Expected max X/Y (unchanged)
				initialYIntercept, // Expected Y Intercept (unchanged)
				initialSlope,      // Expected Slope (unchanged)
				initialSpread,     // Expected Spread (unchanged)
				[]float64{initialYIntercept, initialYIntercept}, // Expected History (unchanged)
			),
			Entry("y intercept below minYIntercept with higher maxUtilization (no update)",
				25.0, 50.0,
				initialMinUtilizationX, initialMinUtilizationY, // Expected min X/Y (unchanged)
				initialMaxUtilizationX, initialMaxUtilizationY, // Expected max X/Y (unchanged)
				initialYIntercept, // Expected Y Intercept (unchanged)
				initialSlope,      // Expected Slope (unchanged)
				initialSpread,     // Expected Spread (unchanged)
				[]float64{initialYIntercept, initialYIntercept}, // Expected History (unchanged)
			),
			Entry("slope below minSlope (no update)",
				20.0, 15.0,
				initialMinUtilizationX, initialMinUtilizationY, // Expected min X/Y (unchanged)
				initialMaxUtilizationX, initialMaxUtilizationY, // Expected max X/Y (unchanged)
				initialYIntercept, // Expected Y Intercept (unchanged)
				initialSlope,      // Expected Slope (unchanged)
				initialSpread,     // Expected Spread (unchanged)
				[]float64{initialYIntercept, initialYIntercept}, // Expected History (unchanged)
			),
		)
	})
})

type EnergyModelInterface interface {
	EnergyFn(secondsElapsed, cpuRatio, maxEnergy float64)
}

type MockRaplZone struct {
	Idle               float64
	MaxEnergyPerSecond float64
	Energy             float64
	EnergyFn           func(float64, float64, float64) float64
}

func (m *MockRaplZone) tick(secondsElapsed float64, cpuRatio float64) float64 {
	prevEnergy := m.Energy
	m.Energy = m.Energy + m.EnergyFn(secondsElapsed, cpuRatio, m.MaxEnergyPerSecond) + (m.Idle * secondsElapsed)
	return prevEnergy
}

func LinearEnergy(secondsElapsed float64, cpuRatio float64, maxEnergyPerSecond float64) float64 {
	return cpuRatio * maxEnergyPerSecond * secondsElapsed
}

type ScrapeInfo struct {
	CpuTime   uint64
	Converged bool
}

type IdleEnergyCalcTestInput struct {
	IdlePerSecond      float64
	MaxEnergyPerSecond float64
	EnergyFn           func(float64, float64, float64) float64
	Scrapes            []ScrapeInfo
	Duration           uint64
	CpuCount           uint64
}

var _ = Describe("Test Node Stats Idle Energy Calculation", func() {

	var (
		ns             *NodeStats
		MockedSocketID = "socket0"
	)
	BeforeEach(func() {
		// Initialize Node Stats
		_, err := config.Initialize(".")
		Expect(err).NotTo(HaveOccurred())

		SetMockedCollectorMetrics()
		CreateMockedProcessStats(2)
		ns = NewNodeStats()
		// Initialize CPU Time in Node Stats
		ns.ResourceUsage[config.CPUTime].SetDeltaStat(MockedSocketID, 0)
		// Initialize Energy in Node Stats
		ns.EnergyUsage[config.AbsEnergyInPkg].SetDeltaStat(MockedSocketID, 0)
	})
	// precondition: CpuTime > 0.0
	Describe("UpdateIdleEnergyWithLinearRegression", func() {
		DescribeTable("with different data point arrays",
			func(input IdleEnergyCalcTestInput) {
				testRapl := MockRaplZone{
					Idle:               input.IdlePerSecond,
					MaxEnergyPerSecond: input.MaxEnergyPerSecond,
					Energy:             0.0,
					EnergyFn:           input.EnergyFn,
				}
				minCpuTime := float64(input.Scrapes[0].CpuTime)
				maxCpuTime := float64(input.Scrapes[0].CpuTime)

				for index, scrape := range input.Scrapes {
					maxTheoreticalCpuTime := input.CpuCount * input.Duration * 1000
					cpuRatio := float64(scrape.CpuTime) / float64(maxTheoreticalCpuTime)
					// Tick Mock Rapl
					prevEnergy := testRapl.tick(
						float64(input.Duration),
						cpuRatio,
					)
					// Update CPU Time in Node Stats
					ns.ResourceUsage[config.CPUTime].SetDeltaStat(MockedSocketID, scrape.CpuTime)
					Expect(scrape.CpuTime).To(Equal(ns.ResourceUsage[config.CPUTime].SumAllDeltaValues()))
					minCpuTime = math.Min(float64(minCpuTime), float64(scrape.CpuTime))
					maxCpuTime = math.Max(float64(maxCpuTime), float64(scrape.CpuTime))
					// Update Pkg Energy in Node Stats
					// Note: SetAggrStat won't allow addition of 0 at the start and SetDeltaStat won't allow 0 values after initialization
					ns.EnergyUsage[config.AbsEnergyInPkg].SetDeltaStat(MockedSocketID, uint64(testRapl.Energy)-uint64(prevEnergy))
					Expect(uint64(testRapl.Energy) - uint64(prevEnergy)).To(Equal(ns.EnergyUsage[config.AbsEnergyInPkg].SumAllDeltaValues()))
					// including platform and uncore together might detail if there is a bug
					ns.UpdateIdleEnergyWithLinearRegression(true, input.Duration, input.CpuCount)

					// Validate utilization points, spread, convergence, and idle energy
					calculator := ns.IdleEnergy[config.IdleEnergyInPkg]
					minUtilization := ns.IdleEnergy[config.IdleEnergyInPkg].MinUtilization
					maxUtilization := ns.IdleEnergy[config.IdleEnergyInPkg].MaxUtilization
					Expect(minUtilization.X).To(BeNumerically("~", minCpuTime, 1e-9))
					Expect(maxUtilization.X).To(BeNumerically("~", maxCpuTime, 1e-9))
					Expect(calculator.Result.Spread).To(BeNumerically("~",
						maxCpuTime/float64(maxTheoreticalCpuTime)-minCpuTime/float64(maxTheoreticalCpuTime),
						1e-9))
					if index == 0 {
						Expect(calculator.Result.CalculatedIdleEnergy).To(BeNumerically("~", 0.0, 1e-9))
					} else {
						Expect(calculator.Result.CalculatedIdleEnergy).To(BeNumerically("~", input.IdlePerSecond*float64(input.Duration), 1e-9))
					}
					Expect(calculator.IsIdlePowerReliable).To(Equal(scrape.Converged))
				}
			},
			Entry("test linear model",
				IdleEnergyCalcTestInput{
					IdlePerSecond:      200.0,
					MaxEnergyPerSecond: 1000,
					EnergyFn:           LinearEnergy,
					Scrapes: []ScrapeInfo{
						{
							CpuTime:   700,
							Converged: false,
						},
						{
							CpuTime:   500,
							Converged: false,
						},
						{
							CpuTime:   1500,
							Converged: false,
						},
						{
							CpuTime:   200,
							Converged: false,
						},
						{
							CpuTime:   5000,
							Converged: true,
						},
						{
							CpuTime:   6000,
							Converged: true,
						},
						{
							CpuTime:   10,
							Converged: true,
						},
					},
					CpuCount: 2,
					Duration: 3,
				},
			),
		)
	})
})
