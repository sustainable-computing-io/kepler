// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package fakeworkload

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeProc(t *testing.T, root, pid, comm, cwdTarget string) {
	t.Helper()
	dir := filepath.Join(root, pid)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "comm"), []byte(comm+"\n"), 0o644))
	if cwdTarget != "" {
		require.NoError(t, os.Symlink(cwdTarget, filepath.Join(dir, "cwd")))
	}
}

func TestStressNGGPUWorkload_CwdFilter(t *testing.T) {
	matchDir := t.TempDir()
	otherDir := t.TempDir()
	matchCwd, err := filepath.EvalSymlinks(matchDir)
	require.NoError(t, err)
	otherCwd, err := filepath.EvalSymlinks(otherDir)
	require.NoError(t, err)

	root := t.TempDir()
	writeProc(t, root, "2001", "stress-ng-cpu", matchCwd)
	writeProc(t, root, "2002", "stress-ng-cpu", otherCwd)
	writeProc(t, root, "2003", "bash", matchCwd)

	finder, err := New(root, matchDir)
	require.NoError(t, err)
	assert.Equal(t, []uint32{2001}, finder.PIDs())

	t.Run("relative workload dir", func(t *testing.T) {
		prev, err := os.Getwd()
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.Chdir(prev) })
		require.NoError(t, os.Chdir(matchDir))

		finder, err := New(root, ".")
		require.NoError(t, err)
		assert.Equal(t, []uint32{2001}, finder.PIDs())
	})

	t.Run("empty dir yields no PIDs", func(t *testing.T) {
		finder, err := New(root, "")
		require.NoError(t, err)
		assert.Empty(t, finder.PIDs())
	})
}

func TestNew_InvalidProcfs(t *testing.T) {
	_, err := New(filepath.Join(t.TempDir(), "missing"), t.TempDir())
	assert.Error(t, err)
}
