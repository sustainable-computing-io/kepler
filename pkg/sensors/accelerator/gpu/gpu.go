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

package gpu

import (
	gpu_source "github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator/gpu/source"
	"k8s.io/klog/v2"
)

var (
	acceleratorOrder = []acceleratorInterface{
		&gpu_source.GPUDcgm{},
		&gpu_source.GPUNvml{},
		&gpu_source.GPUDummy{},
	}
)

/*
Some systems have compatibility issues with the nvidia library. See https://github.com/sustainable-computing-io/kepler/issues/184
Therefore, we can disable files that use the NVIDIA library using the "+build gpu" tag. This means that the compiler will only include these files if the compilation has the gpu tag.

Only a file with "+build gpu" can access GPUNvml and GPUDummy, which also have "+build gpu".
Then, we use gpu.go file to initialize the acceleratorImpl from power.go when gpu is enabled.
*/

// init initialize the acceleratorImpl and start it
func init() {
	var errLib error
	for i := 0; i < len(acceleratorOrder); i++ {
		klog.Infof("Trying to initialize GPU collector using %s", acceleratorOrder[i].GetName())
		acceleratorImpl = acceleratorOrder[i]
		errLib = acceleratorImpl.InitLib()
		if errLib == nil {
			klog.Infof("Using %s to obtain gpu power", acceleratorImpl.GetName())
			return
		} else {
			klog.Infof("Error initializing %s: %v", acceleratorImpl.GetName(), errLib)
		}
	}
	klog.Infof("no gpu collector available: %v", errLib)
}
