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
	"os"
	"testing"

	metric_util "github.com/sustainable-computing-io/kepler/pkg/collector/stats"
)

func TestGetNodeCredByNodeName(t *testing.T) {
	credMap = map[string]string{
		"redfish_username": "admin",
		"redfish_password": "password",
		"redfish_host":     "node1",
	}
	c := csvNodeCred{}

	// Test with target "redfish"
	result, err := c.GetNodeCredByNodeName("node1", "redfish")
	if err != nil {
		t.Errorf("Expected nil error, got: %v", err)
	}
	expected := credMap
	if !mapStringStringEqual(result, expected) {
		t.Errorf("Expected credMap: %v, got: %v", expected, result)
	}

	// Test with unsupported target
	_, err = c.GetNodeCredByNodeName("node1", "unsupported")
	if err == nil {
		t.Errorf("Expected an error, got nil")
	}

	// Test with nil credMap
	credMap = nil
	_, err = c.GetNodeCredByNodeName("node1", "redfish")
	if err == nil {
		t.Errorf("Expected an error, got nil")
	}
}

func TestIsSupported(t *testing.T) {
	// Test when redfish_cred_file_path is missing
	info := map[string]string{}
	c := csvNodeCred{}
	result := c.IsSupported(info)
	if result {
		t.Errorf("Expected false, got: %v", result)
	}

	// Test when redfish_cred_file_path is empty
	info = map[string]string{
		"redfish_cred_file_path": "",
	}
	result = c.IsSupported(info)
	if result {
		t.Errorf("Expected false, got: %v", result)
	}

	// create a temp csv file with the following content:
	// node1,admin,password,localhost
	// node2,admin,password,localhost
	// node3,admin,password,localhost
	file, err := os.CreateTemp("", "test.csv")
	if err != nil {
		t.Errorf("Expected nil error, got: %v", err)
	}
	defer os.Remove(file.Name())
	_, err = file.WriteString("node1,admin,password,localhost\nnode2,admin,password,localhost\nnode3,admin,password,localhost\n")
	if err != nil {
		t.Errorf("Expected nil error, got: %v", err)
	}

	// Test with valid redfish_cred_file_path
	info = map[string]string{
		"redfish_cred_file_path": file.Name(),
	}

	// set ENV variable NODE_NAME to "node1"
	os.Setenv("NODE_NAME", "node1")
	// check if getNodeName() returns "node1"
	nodeName := metric_util.GetNodeName()
	if nodeName != "node1" {
		t.Errorf("Expected nodeName: node1, got: %v", nodeName)
	}
	// readCSVFile should return the credentials for node1
	userName, password, host, err := readCSVFile(file.Name(), nodeName)
	if err != nil {
		t.Errorf("Expected nil error, got: %v", err)
	}
	if host != "localhost" {
		t.Errorf("Expected host: localhost, got: %v", host)
	}
	if userName != "admin" {
		t.Errorf("Expected userName: admin, got: %v", userName)
	}
	if password != "password" {
		t.Errorf("Expected password: password, got: %v", password)
	}
	result = c.IsSupported(info)
	if !result {
		t.Errorf("Expected true, got: %v", result)
	}
}

// Helper function to compare two maps of strings
func mapStringStringEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, valA := range a {
		valB, ok := b[key]
		if !ok || valA != valB {
			return false
		}
	}
	return true
}
