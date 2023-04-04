//go:build darwin
// +build darwin

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

package cgroup

type CCgroupStatHandler struct{}

var (
	AvailableCGroupMetrics = []string{}
)

// If Kepler in not running in Linux OS the cgroup stat handler is nil
func NewCGroupStatHandler(pid int) (*CCgroupStatHandler, error) {
	return nil, nil
}

func GetAvailableCGroupMetrics() []string {
	return AvailableCGroupMetrics
}

func (hander *CCgroupStatHandler) GetCGroupStat() (stats map[string]uint64, err error) {
	statsMap := make(map[string]uint64)
	return statsMap, nil
}
