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
	// SampleCnt is a message counter
	SampleCnt uint64
	// PciTransCnt is a PCIe Partial Transaction counter
	PciTransCnt uint64
	// Latency is the Average Get To Put latency in nanoseconds
	Latency uint64
	// BwIn is the PCIe write bandwidth in Mbps
	BwIn uint64
	// BwOut is the PCIe read bandwidth in Mbps
	BwOut uint64
	// CprUtil is the Compression Slice Utilization On Slice X in percentage execution cycles
	CprUtil uint64
	// DcprUtil is the Decompression Slice Utilization On Slice X in percentage execution cycles
	DcprUtil uint64
	// XltUtil is the Translator Slice Utilization On Slice X in percentage execution cycles
	XltUtil uint64
	// CphUtil is the Cipher Slice Utilization On Slice X in percentage execution cycles
	CphUtil uint64
	// AthUtil is the Authentication Slice Utilization On Slice X, percentage execution cycles
	AthUtil uint64
}
