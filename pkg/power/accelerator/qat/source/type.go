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

package source

/*
Further understand the device utilization obtained through telemetry by viewing the document: Intel® QuickAssist Technology Software for Linux* - Programmer's Guide.
(https://www.intel.com/content/www/us/en/content-details/743912/)
*/

type DeviceUtilizationSample struct {
	SampleCnt   uint64
	PciTransCnt uint64
	Latency     uint64
	BwIn        uint64
	BwOut       uint64
	CprUtil     uint64
	DcprUtil    uint64
	XltUtil     uint64
	CphUtil     uint64
	AthUtil     uint64
}
