// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package profile

import (
	"errors"
	"path/filepath"
	"runtime/pprof"
	"testing"

	"github.com/stretchr/testify/assert"
)

func cleanupProfiling() {
	pprof.StopCPUProfile()
	stopProfilingQueue = nil
}

func TestProfiling(t *testing.T) {
	t.Cleanup(cleanupProfiling)

	t.Run("stop profiling without active profiling", func(t *testing.T) {
		cleanupProfiling()

		err := StopProfiling()
		assert.NoError(t, err)
	})

	t.Run("invalid cpu profile path", func(t *testing.T) {
		cleanupProfiling()

		err := StartProfiling("/nonexistent/directory/cpu.pprof", "")
		assert.Error(t, err)
	})

	t.Run("cpu profile already active error", func(t *testing.T) {
		cleanupProfiling()

		tmpDir := t.TempDir()
		cpuFile1 := filepath.Join(tmpDir, "cpu1.pprof")
		cpuFile2 := filepath.Join(tmpDir, "cpu2.pprof")
		memFile := filepath.Join(tmpDir, "mem.pprof")

		err := StartProfiling(cpuFile1, memFile)
		assert.NoError(t, err)

		err = StartProfiling(cpuFile2, memFile)
		assert.Error(t, err)

		_ = StopProfiling()
		cleanupProfiling()
	})

	t.Run("invalid memory profile path", func(t *testing.T) {
		cleanupProfiling()

		tmpDir := t.TempDir()
		cpuFile := filepath.Join(tmpDir, "cpu.pprof")

		err := StartProfiling(cpuFile, "/nonexistent/directory/mem.pprof")
		assert.Error(t, err)

		_ = StopProfiling()
		cleanupProfiling()
	})

	t.Run("stop profiling with queue error", func(t *testing.T) {
		cleanupProfiling()

		stopProfilingQueue = []func() error{
			func() error {
				return errors.New("simulated queue error")
			},
		}

		err := StopProfiling()
		assert.ErrorContains(t, err, "simulated queue error")

		cleanupProfiling()
	})

	t.Run("successful profiling start and stop", func(t *testing.T) {
		cleanupProfiling()

		tmpDir := t.TempDir()
		cpuFile := filepath.Join(tmpDir, "cpu.pprof")
		memFile := filepath.Join(tmpDir, "mem.pprof")

		err := StartProfiling(cpuFile, memFile)
		assert.NoError(t, err)

		err = StopProfiling()
		assert.NoError(t, err)

		assert.FileExists(t, cpuFile)
		assert.FileExists(t, memFile)

		cleanupProfiling()
	})
}
