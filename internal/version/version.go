/*
Copyright 2025.

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
