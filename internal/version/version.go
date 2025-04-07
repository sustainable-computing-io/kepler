// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package version

import "runtime"

var (
	version   string
	buildTime string
	gitBranch string
	gitCommit string
)

type VersionInfo struct {
	Version   string
	BuildTime string
	GitBranch string
	GitCommit string

	GoVersion string
	GoOS      string
	GoArch    string
}

// Info returns the version information
func Info() VersionInfo {
	return VersionInfo{
		Version:   version,
		BuildTime: buildTime,
		GitBranch: gitBranch,
		GitCommit: gitCommit,

		GoVersion: runtime.Version(),
		GoOS:      runtime.GOOS,
		GoArch:    runtime.GOARCH,
	}
}
