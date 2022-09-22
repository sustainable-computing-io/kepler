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

package cgroup

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func SearchByContainerID(topFolder, containerID string) string {
	found := filepath.Walk(topFolder,
		func(path string, info os.FileInfo, err error) error {
			if path == topFolder {
				return nil
			}
			if strings.Contains(path, containerID) {
				return errors.New(path)
			}
			return nil
		})
	if found != nil {
		return found.Error()
	}
	return ""
}

func ReadUInt64(fileName string) (uint64, error) {
	value, err := os.ReadFile(fileName)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(strings.TrimSpace(string(value)), 10, 64)
}

func ReadKV(fileName string) (map[string]interface{}, error) {
	values := make(map[string]interface{})
	f, err := os.Open(fileName)
	if err != nil {
		return values, err
	}

	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 2 {
			v, err := strconv.ParseUint(fields[1], 10, 64)
			if err == nil {
				values[fields[0]] = v
			}
		}
	}

	return values, sc.Err()
}

func ReadLineKEqualToV(fileName string) (map[string]interface{}, error) {
	values := make(map[string]interface{})
	f, err := os.Open(fileName)
	if err != nil {
		return values, err
	}

	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if strings.Contains(fields[0], "253:") {
			// device-mapper
			continue
		}
		for _, field := range fields {
			if strings.Contains(field, "=") {
				kv := strings.Split(field, "=")
				if _, exists := values[kv[0]]; !exists {
					values[kv[0]] = uint64(0)
				}
				v, err := strconv.ParseUint(kv[1], 10, 64)
				if err == nil {
					values[kv[0]] = values[kv[0]].(uint64) + v
				}
			}
		}
	}

	return values, sc.Err()
}
