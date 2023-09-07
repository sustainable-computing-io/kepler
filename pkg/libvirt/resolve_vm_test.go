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

package libvirt

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func createMockLibvirtDir(directory string) {
	mockFiles := []struct {
		name    string
		content string
	}{
		{"vm1.pid", "1234"},
		{"vm2.pid", "5678"},
	}

	for _, file := range mockFiles {
		err := ioutil.WriteFile(filepath.Join(directory, file.name), []byte(file.content), 0644)
		if err != nil {
			panic(err)
		}
	}
}

func createMockProcDir(directory string) {
	mockThreadDirs := []string{
		"/proc/1234/task/123",
		"/proc/1234/task/456",
		"/proc/1234/task/789",
		"/proc/5678/task/1234",
		"/proc/5678/task/4567",
		"/proc/5678/task/7890",
	}
	for _, dir := range mockThreadDirs {
		err := os.MkdirAll(filepath.Join(directory, dir), 0755)
		if err != nil {
			panic(err)
		}
	}
}

func TestGetCurrentVMPID(t *testing.T) {
	mockLibvirtDir := t.TempDir()
	createMockLibvirtDir(mockLibvirtDir)

	mockProcDir := t.TempDir()
	createMockProcDir(mockProcDir)

	pidFiles, err := GetCurrentVMPID(mockLibvirtDir, mockProcDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expectedResult := map[string]string{
		"123":  "vm1",
		"456":  "vm1",
		"789":  "vm1",
		"1234": "vm2",
		"4567": "vm2",
		"7890": "vm2",
	}

	if !reflect.DeepEqual(pidFiles, expectedResult) {
		t.Errorf("Expected: %v, Got: %v", expectedResult, pidFiles)
	}
}
