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
	"testing"
)

func TestQAT_parseStatusInfo(t *testing.T) {
	// device status data
	statusData := `Checking status of all devices.
        There is 6 QAT acceleration device(s) in the system:
         qat_dev0 - type: 4xxx,  inst_id: 0,  node_id: 0,  bsf: 0000:6b:00.0,  #accel: 1 #engines: 9 state: up
         qat_dev1 - type: 4xxx,  inst_id: 1,  node_id: 0,  bsf: 0000:70:00.0,  #accel: 1 #engines: 9 state: down
         qat_dev2 - type: 4xxx,  inst_id: 2,  node_id: 1,  bsf: 0000:75:00.1,  #accel: 1 #engines: 9 state: up
         qat_dev3 - type: 4xxx,  inst_id: 3,  node_id: 1,  bsf: 0000:7a:00.1,  #accel: 1 #engines: 9 state: down
         qat_dev4 - type: 4xxxvf,  inst_id: 4,  node_id: 0,  bsf: 0000:6b:00.1,  #accel: 1 #engines: 1 state: up
         qat_dev5 - type: 4xxxvf,  inst_id: 5,  node_id: 1,  bsf: 0000:70:00.1,  #accel: 1 #engines: 1 state: down`

	// the device information that should be obtained after parsing
	actualDevices := map[string]interface{}{
		"qat_dev0": qatDevInfo{addr: "6b"},
		"qat_dev2": qatDevInfo{addr: "75"},
	}

	// parse status data
	parseDevices, err := parseStatusInfo(statusData)
	if err != nil {
		t.Errorf("parsing failed, unable to obtain available QAT device: %s", err)
	}

	// compare the length of actualDevices and parseDevices
	if len(actualDevices) != len(parseDevices) {
		t.Error("parsing failed, obtain incorrect qat device information")
	}

	// compare the key value pairs of actualDevices and parseDevices
	for qatDev, trueInfo := range actualDevices {
		parseInfo, exits := parseDevices[qatDev]
		if !exits || parseInfo != trueInfo {
			t.Errorf("parsing failed, incorrect information obtain from:%s", qatDev)
		}
	}
}
