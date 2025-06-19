// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPodClone(t *testing.T) {
	t.Run("Clone full Pod with all fields", func(t *testing.T) {
		original := &Pod{
			ID:           "pod-123",
			Name:         "test-pod",
			Namespace:    "default",
			CPUTotalTime: 42.5,
			CPUTimeDelta: 10.2,
		}

		clone := original.Clone()
		require.NotNil(t, clone)
		assert.Equal(t, original.ID, clone.ID)
		assert.Equal(t, original.Name, clone.Name)
		assert.Equal(t, original.Namespace, clone.Namespace)
		// CPU times should not be copied in Clone
		assert.Equal(t, float64(0), clone.CPUTotalTime)
		assert.Equal(t, float64(0), clone.CPUTimeDelta)

		// Verify they are separate objects
		assert.NotSame(t, original, clone)
	})

	t.Run("Clone nil Pod", func(t *testing.T) {
		var nilPod *Pod
		nilClone := nilPod.Clone()
		assert.Nil(t, nilClone, "Cloning nil Pod should return nil")
	})
}
