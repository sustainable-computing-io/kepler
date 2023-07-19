//go:build !bcc
// +build !bcc

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

package attacher

import (
	"fmt"
)

const (
	BccBuilt = false
)

var bccCounters = map[string]perfCounter{}

func attachBccModule() (interface{}, error) {
	return nil, fmt.Errorf("no bcc build tag")
}

func detachBccModule() {
}

func bccCollectProcess() (processesData []ProcessBPFMetrics, err error) {
	processesData = []ProcessBPFMetrics{}
	return
}

func bccCollectFreq() (cpuFreqData map[int32]uint64, err error) {
	cpuFreqData = make(map[int32]uint64)
	return
}
