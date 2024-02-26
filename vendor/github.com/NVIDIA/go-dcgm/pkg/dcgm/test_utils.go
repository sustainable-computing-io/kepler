/*
 * Copyright (c) 2024, NVIDIA CORPORATION.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package dcgm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func setupTest(t *testing.T) func(t *testing.T) {
	cleanup, err := Init(Embedded)
	assert.NoError(t, err)

	return func(t *testing.T) {
		defer cleanup()
	}
}

func runOnlyWithLiveGPUs(t *testing.T) {
	t.Helper()
	gpus, err := getSupportedDevices()
	assert.NoError(t, err)
	if len(gpus) < 1 {
		t.Skip("Skipping test that requires live GPUs. None were found")
	}
}
