/*
Copyright 2024.

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

package devices

type GPUProcessUtilizationSample struct {
	Pid         uint32
	TimeStamp   uint64
	ComputeUtil uint32
	MemUtil     uint32
	EncUtil     uint32
	DecUtil     uint32
}

// Device can hold GPU Device or Multi Instance GPU slice handler
type GPUDevice struct {
	DeviceHandler interface{}
	ID            int // Entity ID or Parent ID if Subdevice
	IsSubdevice   bool
	ParentID      int     // GPU Entity ID  or Parent GPU ID if MIG slice
	MIGSMRatio    float64 // Ratio of MIG SMs / Total GPU SMs to be used to normalize the MIG metrics
}
