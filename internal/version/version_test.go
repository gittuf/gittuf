// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetVersion(t *testing.T) {
	// The behavior of debug.ReadBuildInfo() depends on how the test is run.
	// We primarily want to ensure that GetVersion() returns something and doesn't panic.
	version := GetVersion()
	assert.NotEmpty(t, version)

	// Verify that if it falls back to gitVersion, we get the expected fallback value
	// If the build info contains an actual version, it will return that instead.
	// Since we can't easily mock debug.ReadBuildInfo, we just verify the return isn't empty.

	// We can explicitly test the gitVersion fallback behavior by inspecting the default
	assert.Equal(t, "devel", gitVersion)
}
