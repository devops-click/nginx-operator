/*
Copyright 2024 DevOps Click.

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

// Package version provides build-time version information for the operator.
// These variables are set via ldflags during the build process.
//
// Usage:
//
//	fmt.Printf("Version: %s, Commit: %s\n", version.Version, version.GitCommit)
package version

import (
	"fmt"
	"runtime"
)

// These variables are set at build time via -ldflags.
var (
	// Version is the semantic version of the operator (e.g., "1.0.0").
	Version = "dev"

	// GitCommit is the short SHA of the git commit used to build the binary.
	GitCommit = "unknown"

	// BuildDate is the ISO 8601 date when the binary was built.
	BuildDate = "unknown"
)

// Info holds structured version information for the operator.
type Info struct {
	Version   string `json:"version"`
	GitCommit string `json:"gitCommit"`
	BuildDate string `json:"buildDate"`
	GoVersion string `json:"goVersion"`
	Platform  string `json:"platform"`
}

// Get returns the current version information.
//
// Usage:
//
//	info := version.Get()
//	log.Info("starting operator", "version", info.Version, "commit", info.GitCommit)
func Get() Info {
	return Info{
		Version:   Version,
		GitCommit: GitCommit,
		BuildDate: BuildDate,
		GoVersion: runtime.Version(),
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

// String returns a human-readable version string.
//
// Usage:
//
//	fmt.Println(version.Get().String())
//	// Output: nginx-operator dev (commit: unknown, built: unknown, go: go1.24.0, platform: linux/amd64)
func (i Info) String() string {
	return fmt.Sprintf("nginx-operator %s (commit: %s, built: %s, go: %s, platform: %s)",
		i.Version, i.GitCommit, i.BuildDate, i.GoVersion, i.Platform)
}
