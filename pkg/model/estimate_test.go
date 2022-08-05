package model

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	MODEL_NAME       string      = "" // auto-select
	LR_NAME          string      = "Linear Regression_10"
	RATIO_MODEL_NAME string      = "CorrRatio"
	METRICS          []string    = []string{"curr_bytes_read", "curr_bytes_writes", "curr_cache_misses", "curr_cgroupfs_cpu_usage_us", "curr_cgroupfs_memory_usage_bytes", "curr_cgroupfs_system_cpu_usage_us", "curr_cgroupfs_user_cpu_usage_us", "curr_cpu_cycles", "curr_cpu_instructions", "curr_cpu_time"}
	VALUES           [][]float32 = [][]float32{[]float32{1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, []float32{1, 1, 1, 1, 1, 1, 1, 1, 1, 1}}
	FAIL_VALUES      [][]float32 = [][]float32{[]float32{1, 1, 1, 1, 1, 1}}
)

var _ = Describe("Test Estimator Unit", func() {
	It("Start Estimator and Get Power", func() {
		empty := []float32{}
		// should power of each pods
		powers := GetPower(MODEL_NAME, METRICS, VALUES, empty, empty, empty, empty)
		Expect(len(powers)).To(Equal(len(VALUES)))
		powers = GetPower(LR_NAME, METRICS, VALUES, empty, empty, empty, empty)
		Expect(len(powers)).To(Equal(len(VALUES)))
		powers = GetPower(RATIO_MODEL_NAME, METRICS, VALUES, []float32{10, 10}, []float32{5, 5}, []float32{0, 0}, []float32{0, 0})
		Expect(len(powers)).To(Equal(len(VALUES)))
		fmt.Println(powers)
		// should safely return empty list if fails
		powers = GetPower(MODEL_NAME, METRICS, FAIL_VALUES, empty, empty, empty, empty)
		Expect(len(powers)).To(Equal(0))
	})
})
