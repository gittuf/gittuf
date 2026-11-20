// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunWritesProfilesWhenCommandFails(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir) // not a Git repository, so the command below fails

	cpuFile := filepath.Join(tmpDir, "cpu.prof")
	memoryFile := filepath.Join(tmpDir, "memory.prof")

	args := os.Args
	t.Cleanup(func() { os.Args = args })
	os.Args = []string{"gittuf", "--profile", "--profile-CPU-file", cpuFile, "--profile-memory-file", memoryFile, "rsl", "log"}

	assert.Equal(t, 1, run())

	for _, file := range []string{cpuFile, memoryFile} {
		info, err := os.Stat(file)
		require.NoError(t, err)
		assert.NotZero(t, info.Size(), "profile '%s' is empty", file)
	}
}
