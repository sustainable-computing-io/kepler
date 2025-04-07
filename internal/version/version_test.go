// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package version

import (
	"runtime"
	"testing"
)

func TestInfo(t *testing.T) {
	// Get version info
	info := Info()

	// Check that runtime fields are properly set
	if info.GoVersion != runtime.Version() {
		t.Errorf("GoVersion = %v, want %v", info.GoVersion, runtime.Version())
	}

	if info.GoOS != runtime.GOOS {
		t.Errorf("GoOS = %v, want %v", info.GoOS, runtime.GOOS)
	}

	if info.GoArch != runtime.GOARCH {
		t.Errorf("GoArch = %v, want %v", info.GoArch, runtime.GOARCH)
	}
}

func TestVersionValues(t *testing.T) {
	// test cases
	testCases := []struct {
		name   string
		ver    string
		time   string
		branch string
		commit string
	}{
		{
			name:   "empty values",
			ver:    "",
			time:   "",
			branch: "",
			commit: "",
		},
		{
			name:   "typical values",
			ver:    "v1.2.3",
			time:   "2025-04-01T12:00:00Z",
			branch: "main",
			commit: "abcdef123456",
		},
		{
			name:   "dev values",
			ver:    "dev",
			time:   "unknown",
			branch: "feature-branch",
			commit: "deadbeef",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set test values
			version = tc.ver
			buildTime = tc.time
			gitBranch = tc.branch
			gitCommit = tc.commit

			// Get version info
			info := Info()

			// Check values match what we set
			if info.Version != tc.ver {
				t.Errorf("Version = %v, want %v", info.Version, tc.ver)
			}

			if info.BuildTime != tc.time {
				t.Errorf("BuildTime = %v, want %v", info.BuildTime, tc.time)
			}

			if info.GitBranch != tc.branch {
				t.Errorf("GitBranch = %v, want %v", info.GitBranch, tc.branch)
			}

			if info.GitCommit != tc.commit {
				t.Errorf("GitCommit = %v, want %v", info.GitCommit, tc.commit)
			}
		})
	}
}
