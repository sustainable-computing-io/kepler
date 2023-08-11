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

package qat

import (
	qat_source "github.com/sustainable-computing-io/kepler/pkg/power/accelerator/qat/source"
	"k8s.io/klog/v2"
)

// init initialize the qatImpl and start it
func init() {
	qatImpl = &qat_source.QATTelemetry{}
	err := qatImpl.Init()
	if err == nil {
		klog.Infoln("Using qat-telemetry to obtain qat metrics")
		// If the library was successfully initialized, we don't need to return an error in the Init() function
		errLib = nil
		return
	}

	klog.Infof("Failed to init qat-telemtry err: %v\n", err)
}
