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

package nodecred

import (
	"encoding/csv"
	"fmt"
	"os"

	metric_util "github.com/sustainable-computing-io/kepler/pkg/collector/stats"

	"k8s.io/klog/v2"
)

// csvNodeCredImpl is the implementation of NodeCred using on disk file
// the file is in the format of
// node1,admin,password,localhost
// node2,admin,password,localhost
// node3,admin,password,localhost
type csvNodeCred struct {
}

var (
	credMap map[string]string
)

func (c csvNodeCred) GetNodeCredByNodeName(nodeName, target string) (map[string]string, error) {
	if credMap == nil {
		return nil, fmt.Errorf("credential is not set")
	} else if target == "redfish" {
		cred := make(map[string]string)
		cred["redfish_username"] = credMap["redfish_username"]
		cred["redfish_password"] = credMap["redfish_password"]
		cred["redfish_host"] = credMap["redfish_host"]
		if cred["redfish_username"] == "" || cred["redfish_password"] == "" || cred["redfish_host"] == "" {
			return nil, fmt.Errorf("no credential found")
		}
		return cred, nil
	}

	return nil, fmt.Errorf("no credential found for target %s", target)
}

func (c csvNodeCred) IsSupported(info map[string]string) bool {
	// read redfish_cred_file_path from info
	filePath := info["redfish_cred_file_path"]
	if filePath == "" {
		return false
	} else {
		nodeName := metric_util.GetNodeName()
		// read file from filePath
		userName, password, host, err := readCSVFile(filePath, nodeName)
		if err != nil {
			klog.V(5).Infof("failed to read csv file: %v", err)
			return false
		}
		klog.V(5).Infof("read csv file successfully")
		credMap = make(map[string]string)
		credMap["redfish_username"] = userName
		credMap["redfish_password"] = password
		credMap["redfish_host"] = host
	}
	return true
}

func readCSVFile(filePath, nodeName string) (userName, password, host string, err error) {
	// Open the CSV file
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Println("Error opening the file:", err)
		return
	}
	defer file.Close()

	// Create a new CSV reader
	reader := csv.NewReader(file)

	// Read all rows from the CSV file
	rows, err := reader.ReadAll()
	if err != nil {
		fmt.Println("Error reading CSV:", err)
		return
	}

	// Iterate over each row and check if the node name matches
	for _, row := range rows {
		if row[0] == nodeName && len(row) >= 4 {
			userName = row[1]
			password = row[2]
			host = row[3]
			return userName, password, host, nil
		}
	}
	err = fmt.Errorf("node name %s not found in file %s", nodeName, filePath)
	return
}
