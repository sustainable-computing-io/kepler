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

//nolint:all // not linting this file
package attacher

var (
	// ebpfBatchGet is true if the kernel supports batch get operation
	ebpfBatchGet = true
	// ebpfBatchGetAndDelete is true if delete all the keys after batch get
	ebpfBatchGetAndDelete = ebpfBatchGet
)
