//go:build gpu
// +build gpu

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

package accelerator

import (
	accelerator_source "github.com/sustainable-computing-io/kepler/pkg/power/accelerator/source"
	"k8s.io/klog/v2"
)

/*
Some systems have compatibility issues with the nvidia library. See https://github.com/sustainable-computing-io/kepler/issues/184
Therefore, we can disable files that use the NVIDIA library using the "+build gpu" tag. This means that the compiler will only include these files if the compilation has the gpu tag.

Only a file with "+build gpu" can access GPUNvml and GPUDummy, which also have "+build gpu".
Then, we use gpu.go file to initialize the acceleratorImpl from power.go when gpu is enabled.
*/

// init initialize the acceleratorImpl and start it
func init() {
	acceleratorImpl = &accelerator_source.GPUNvml{}
	err := acceleratorImpl.Init()
	if err == nil {
		klog.Infoln("Using nvml to obtain gpu power")
		// If the library was successfully initialized, we don't need to return an error in the Init() function
		errLib = nil
		return
	}
	// If the library was not successfully initialized, we use the dummy implementation
	klog.Infof("Failed to init nvml, err: %v\n", err)
	acceleratorImpl = &accelerator_source.GPUDummy{}
	errLib = acceleratorImpl.Init()
}
