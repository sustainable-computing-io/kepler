/*
Copyright 2023.

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

package platform_validation_test

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"testing"

	"github.com/joho/godotenv"
	"github.com/jszwec/csvutil"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type componentsPower struct {
	pkgPower    float64
	corePower   float64
	uncorePower float64
	dramPower   float64
}

type csvPowerData struct {
	Pkg    string `csv:"Pkg"`
	Core   string `csv:"Core"`
	Uncore string `csv:"Uncore"`
	Dram   string `csv:"Dram"`
}

var (
	keplerAddr, promAddr, cpuArch string
	ok, raplEnable, acpiEnable, hmcEnable, redfishEnable,
	raplPkgEnable, raplDramEnable, raplCoreEnable, raplUncoreEnable bool
	testPowerData []componentsPower
)

const TRUE = "true"

func TestPlatformValidation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PlatformValidation Suite")
}

var _ = BeforeSuite(func() {
	keplerAddr, ok = os.LookupEnv("kepler_address")
	Expect(ok).To(BeTrue())
	promAddr, ok = os.LookupEnv("prometheus_address")
	Expect(ok).To(BeTrue())

	// get test ENVs
	err := godotenv.Load("platform-validation.env")
	if err != nil {
		fmt.Println("Error loading .env file")
	}
	Expect(err).NotTo(HaveOccurred())
	cpuArch = os.Getenv("CPU_ARCH")
	raplEnable = (os.Getenv("RAPL_ENABLED") == TRUE)
	if raplEnable {
		raplPkgEnable = (os.Getenv("RAPL_PKG_ENABLED") == TRUE)
		raplDramEnable = (os.Getenv("RAPL_DRAM_ENABLED") == TRUE)
		raplCoreEnable = (os.Getenv("RAPL_CORE_ENABLED") == TRUE)
		raplUncoreEnable = (os.Getenv("RAPL_UNCORE_ENABLED") == TRUE)
	}
	hmcEnable = (os.Getenv("HMC_ENABLED") == TRUE)
	redfishEnable = (os.Getenv("REDFISH_ENABLED") == TRUE)
	acpiEnable = (os.Getenv("ACPI_ENABLED") == TRUE)
	fmt.Println("Dump ENVs...")
	fmt.Printf("cpuArch:%s\nraplEnable:%t\nacpiEnable:%t\nhmcEnable:%t\nredfishEnable:%t\nraplPkgEnable:%t\nraplDramEnable:%t\nraplCoreEnable:%t\nraplUncoreEnable:%t\n", cpuArch, raplEnable, acpiEnable, hmcEnable, redfishEnable, raplPkgEnable, raplDramEnable, raplCoreEnable, raplUncoreEnable)

	// get test power data: before kind/before kepler/afte kepler
	f, e := os.Open("power.csv")
	if e != nil {
		fmt.Println("open power.csv failed")
	}
	Expect(e).NotTo(HaveOccurred())

	reader := csv.NewReader(f)
	dec, err := csvutil.NewDecoder(reader)
	Expect(err).NotTo(HaveOccurred())

	testPowerData = make([]componentsPower, 0)
	for {
		var d csvPowerData
		if err := dec.Decode(&d); err == io.EOF {
			break
		}
		var p componentsPower
		var e error
		p.pkgPower, e = strconv.ParseFloat(d.Pkg, 64)
		Expect(e).NotTo(HaveOccurred())
		p.corePower, e = strconv.ParseFloat(d.Core, 64)
		Expect(e).NotTo(HaveOccurred())
		p.uncorePower, e = strconv.ParseFloat(d.Uncore, 64)
		Expect(e).NotTo(HaveOccurred())
		p.dramPower, e = strconv.ParseFloat(d.Dram, 64)
		Expect(e).NotTo(HaveOccurred())
		testPowerData = append(testPowerData, p)
	}
	powers := []string{
		"\n--- node components power before kind cluster up ---\n",
		"\n--- node components power before deploy kepler ---\n",
		"\n--- node components power after deploy kepler ---\n",
	}
	for i := 0; i < 3; i++ {
		fmt.Println(powers[i])
		fmt.Println(testPowerData[i])
	}
})
