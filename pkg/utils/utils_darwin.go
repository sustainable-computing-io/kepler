//go:build darwin
// +build darwin

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
package utils

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"unsafe"
)

func CreateTempFile(contents string) (filename string, reterr error) {
	f, err := os.CreateTemp("", "")
	if err != nil {
		return "", err
	}
	defer func() {
		if err = f.Close(); err != nil {
			return
		}
	}()
	_, err = f.WriteString(contents)
	if err != nil {
		return "", err
	}
	return f.Name(), nil
}

func CreateTempDir() (dir string, err error) {
	return os.MkdirTemp("", "")
}

func DetermineHostByteOrder() binary.ByteOrder {
	var i int32 = 0x01020304
	u := unsafe.Pointer(&i)
	pb := (*byte)(u)
	b := *pb
	if b == 0x04 {
		return binary.LittleEndian
	}

	return binary.BigEndian
}

const (
	KernelProcessName      string = "kernel_processes"
	KernelProcessNamespace string = "kernel"
	SystemProcessName      string = "system_processes"
	SystemProcessNamespace string = "system"
	EmptyString            string = ""
	GenericSocketID        string = "socket0"
	GenericGPUID           string = "gpu"
)

func GetPathFromPID(searchPath string, pid uint64) (string, error) {
	path := fmt.Sprintf(searchPath, pid)
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open cgroup description file for pid %d: %v", pid, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "pod") || strings.Contains(line, "containerd") || strings.Contains(line, "crio") {
			return line, nil
		}
	}
	return "", fmt.Errorf("could not find cgroup description entry for pid %d", pid)
}

func GetCgroupIDFromPath(byteOrder binary.ByteOrder, path string) (uint64, error) {
	return uint64(0), nil
}

func IsFileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}
