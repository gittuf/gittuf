// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockLookPath simulates the behavior of exec.LookPath for testing.
var mockLookPath func(string) (string, error)

// MockCommand simulates the behavior of exec.Command for testing.
var mockCommand func(name string, args ...string) *exec.Cmd

// TestLoadRepository tests the LoadRepository function.
func TestLoadRepository(t *testing.T) {
	originalLookPath := execLookPath
	originalCommand := execCommand
	defer func() {
		execLookPath = originalLookPath
		execCommand = originalCommand
	}()

	// Mock exec.LookPath
	execLookPath = func(binary string) (string, error) {
		return mockLookPath(binary)
	}

	// Mock exec.Command
	execCommand = func(name string, args ...string) *exec.Cmd {
		return mockCommand(name, args...)
	}

	t.Run("Git binary found", func(t *testing.T) {
		mockLookPath = func(string) (string, error) {
			return "", errors.New("binary not found")
		}

		_, err := LoadRepository()
		require.Nil(t, err)
	})

	t.Run("GIT_DIR environment variable set", func(t *testing.T) {
		mockLookPath = func(string) (string, error) {
			return "/usr/bin/git", nil
		}

		// Set environment variable
		expectedGitDir := "/path/to/repo/.git"
		os.Setenv("GIT_DIR", expectedGitDir)
		defer os.Unsetenv("GIT_DIR")

		repo, err := LoadRepository()
		require.Nil(t, err)
		assert.Equal(t, expectedGitDir, repo.GetGitDir())
	})

	t.Run("rev-parse success", func(t *testing.T) {
		mockLookPath = func(string) (string, error) {
			return "/usr/bin/git", nil
		}

		mockCommand = func(name string, args ...string) *exec.Cmd {
			cmd := exec.Command("echo", "/mock/path/to/repo/.git")
			cmd.Stdout = &bytes.Buffer{}
			return cmd
		}

		repo, err := LoadRepository()
		require.Nil(t, err)
		assert.NotNil(t, repo.GetGitDir())
	})

	t.Run("rev-parse failure", func(t *testing.T) {
		mockLookPath = func(string) (string, error) {
			return "/usr/bin/git", nil
		}

		mockCommand = func(name string, args ...string) *exec.Cmd {
			cmd := exec.Command("echo", "some error")
			cmd.Stderr = &bytes.Buffer{}
			return cmd
		}

		_, err := LoadRepository()
		require.Nil(t, err)
	})
}

// execLookPath is used in the LoadRepository function.
var execLookPath = exec.LookPath

// execCommand is used to mock exec.Command for testing.
var execCommand = exec.Command
