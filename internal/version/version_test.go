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

package version

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGet verifies version info returns correct default values.
func TestGet(t *testing.T) {
	info := Get()

	assert.Equal(t, "dev", info.Version)
	assert.Equal(t, "unknown", info.GitCommit)
	assert.Equal(t, "unknown", info.BuildDate)
	assert.Equal(t, runtime.Version(), info.GoVersion)
	assert.NotEmpty(t, info.Platform)
}

// TestString verifies the human-readable version string format.
func TestString(t *testing.T) {
	info := Get()
	str := info.String()

	assert.Contains(t, str, "nginx-operator")
	assert.Contains(t, str, "dev")
	assert.Contains(t, str, "unknown")
	assert.Contains(t, str, runtime.Version())
}

// TestGetWithOverride verifies version info works when variables are overridden.
func TestGetWithOverride(t *testing.T) {
	// Save originals
	origVersion := Version
	origCommit := GitCommit
	origDate := BuildDate

	// Override
	Version = "1.2.3"
	GitCommit = "abc123"
	BuildDate = "2024-01-01T00:00:00Z"

	defer func() {
		Version = origVersion
		GitCommit = origCommit
		BuildDate = origDate
	}()

	info := Get()
	assert.Equal(t, "1.2.3", info.Version)
	assert.Equal(t, "abc123", info.GitCommit)
	assert.Equal(t, "2024-01-01T00:00:00Z", info.BuildDate)
	assert.Contains(t, info.String(), "1.2.3")
}
