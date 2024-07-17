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

package source

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"k8s.io/klog/v2"
)

var (
	// telemetry base path
	teleBasePath = "/sys/devices/pci0000:%s/0000:%s:00.0/telemetry/"

	// control telemetry switch path
	controlPath = filepath.Join(teleBasePath, "control")

	// obtain device utilization data path
	deviceDataPath = filepath.Join(teleBasePath, "device_data")
)

// getDevices obtain available qat devices and search for ID
func getDevices() (map[string]interface{}, error) {
	// use adf_ctl get qat devices status
	commandText := "adf_ctl status"
	cmd := exec.Command("bash", "-c", commandText)
	statusData, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("could not get qat status %s", err)
	}

	return parseStatusInfo(string(statusData))
}

// parseStatusInfo parse all qat devices information and return available devices
func parseStatusInfo(statusData string) (map[string]interface{}, error) {
	// available devices
	availableDev := make(map[string]interface{})

	lines := strings.Split(statusData, "\n")
	// regular expression pattern, matching rows that meet the condition
	pattern := regexp.MustCompile(`(.*?) - type: (.*?),.* bsf: 0000:(.*?):`)

	for _, line := range lines {
		// match regular expressions
		matches := pattern.FindStringSubmatch(line)

		// extract the identifier of the lines starting with 'qat_dev' and confirm the device status is "up"
		if len(matches) == 4 && !strings.HasSuffix(matches[2], "vf") && strings.Contains(line, "state: up") {
			qatDev := strings.TrimSpace(matches[1])

			// get the corresponding bsf number
			bsf := strings.ReplaceAll(matches[3], "0000:", "")
			availableDev[qatDev] = qatDevInfo{addr: bsf}
		}
	}

	if len(availableDev) == 0 {
		return nil, fmt.Errorf("unable to find an available QAT device. Please check the status of QAT")
	}
	return availableDev, nil
}

// controlTelemetry obtain control paths based on QAT information, then turn on/off telemtry
func controlTelemetry(devices map[string]interface{}, mode int) error {
	var err error
	for qatDev, qatInfo := range devices {
		// path to control the telemetry switch
		bsf := qatInfo.(qatDevInfo).addr
		path := fmt.Sprintf(controlPath, bsf, bsf)

		// turn on/off telemetry
		err = switchTelemetry(path, mode)
		if err != nil {
			klog.V(3).Infof("failed to control %s with mode %d: %s ", qatDev, mode, err)
			delete(devices, qatDev)
		}
	}

	if len(devices) == 0 {
		return fmt.Errorf("unable to control any QAT device. Please check the status of QAT")
	}

	return err
}

// switchTelemetry turn on/off telemetry
func switchTelemetry(filename string, mode int) error {
	file, err := os.OpenFile(filename, os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// turn on/off telemetry
	_, err = file.WriteString(strconv.Itoa(mode))
	if err != nil {
		return err
	}
	return nil
}

// openDataFile open qat telemetry data file, and obtain available devices
func openDataFile(devices map[string]interface{}) (map[string]interface{}, error) {
	// available devices
	availableDev := make(map[string]interface{})
	for qatDev, qatinfo := range devices {
		// dataPath that can read data from telemetry
		bsf := qatinfo.(qatDevInfo).addr

		dataPath := fmt.Sprintf(deviceDataPath, bsf, bsf)

		f, err := os.OpenFile(dataPath, os.O_RDONLY, 0444)
		if err != nil {
			klog.V(3).Infof("failed to open %s telemetry data file: %v\n", qatDev, err)
			delete(devices, qatDev)
			continue
		}
		availableDev[qatDev] = qatDevInfo{addr: bsf, datafile: f}
	}

	if len(availableDev) == 0 {
		return nil, fmt.Errorf("unable to open any telemetry data file for QAT. Please check the status of QAT")
	}
	return availableDev, nil
}

// closeDataFile close qat telemetry data file
func closeDataFile() error {
	var err error
	if len(devices) == 0 {
		return nil
	}
	for qatDev, info := range devices {
		file := info.(qatDevInfo).datafile
		if err = file.Close(); err != nil {
			return fmt.Errorf("failed to close %s telemetry data file: %v", qatDev, err)
		}
	}
	return nil
}

// getUtilization calculate utilization from each qat device
func getUtilization(file *os.File) (DeviceUtilizationSample, error) {
	utilizationSample := DeviceUtilizationSample{}

	// reset file pointer to the beginning
	_, err := file.Seek(0, 0)
	if err != nil {
		return utilizationSample, fmt.Errorf("failed to reset file pointer: %s", err)
	}

	// get all data from telemetry
	data, err := io.ReadAll(file)
	if err != nil {
		return utilizationSample, fmt.Errorf("failed to read device_data file: %s", err)
	}
	if len(data) == 0 {
		return utilizationSample, fmt.Errorf("failed to get device_data, the length of data is zero")
	}

	return processData(strings.Fields(string(data))), nil
}

// processData calculate all telemetry data
func processData(data []string) DeviceUtilizationSample {
	var (
		// compression utilization of all slices
		cprSum uint64
		// decompression utilization of all slices
		dcprSum uint64
		// translator utilization of all slices
		xltSum uint64
		// cipher utilization of on all slices
		cphSum uint64
		// authentication utilization of all slices
		athSum uint64
	)

	// preprocess data by converting []string into map[string]uint64
	output := make(map[string]uint64)

	for i := 0; i < len(data)-1; i += 2 {
		key := data[i]
		value, _ := strconv.ParseUint(data[i+1], 10, 64)
		output[key] = value
	}

	// calculate the utilization of different functions(compression, decopression...)
	for key, value := range output {
		if strings.HasPrefix(key, "util_cpr") {
			cprSum += value
			continue
		}
		if strings.HasPrefix(key, "util_dcpr") {
			dcprSum += value
			continue
		}
		if strings.HasPrefix(key, "util_xlt") {
			xltSum += value
			continue
		}
		if strings.HasPrefix(key, "util_cph") {
			cphSum += value
			continue
		}
		if strings.HasPrefix(key, "util_ath") {
			athSum += value
			continue
		}
	}

	return DeviceUtilizationSample{
		SampleCnt:   output["sample_cnt"],
		PciTransCnt: output["pci_trans_cnt"],
		Latency:     output["lat_acc_avg"],
		BwIn:        output["bw_in"],
		BwOut:       output["bw_out"],
		CprUtil:     cprSum,
		DcprUtil:    dcprSum,
		XltUtil:     xltSum,
		CphUtil:     cphSum,
		AthUtil:     athSum,
	}
}
