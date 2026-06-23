// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package rslrecordat

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestRSLRecordAt(t *testing.T) {
	t.Setenv("GITTUF_DEV", "1")

	t.Run("no repository", func(t *testing.T) {
		tmpDir := t.TempDir()
		currentDir, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = os.Chdir(currentDir)
		}()

		keyPath := filepath.Join(tmpDir, "key")
		if err := os.WriteFile(keyPath, []byte("key"), 0o600); err != nil {
			t.Fatal(err)
		}

		_, _, _, err = cmd.ExecuteCommandC(New(), "-t", "some-target", "-k", keyPath, "refs/heads/main")
		assert.ErrorContains(t, err, "unable to identify git directory")
	})

	t.Run("missing required flag target", func(t *testing.T) {
		_, _, _, err := cmd.ExecuteCommandC(New(), "-k", "key-path", "refs/heads/main")
		assert.ErrorContains(t, err, `required flag(s) "target" not set`)
	})

	t.Run("missing required flag signing-key", func(t *testing.T) {
		_, _, _, err := cmd.ExecuteCommandC(New(), "-t", "some-target", "refs/heads/main")
		assert.ErrorContains(t, err, `required flag(s) "signing-key" not set`)
	})

	t.Run("missing positional arguments", func(t *testing.T) {
		_, _, _, err := cmd.ExecuteCommandC(New(), "-t", "some-target", "-k", "key-path")
		assert.ErrorContains(t, err, "accepts 1 arg(s), received 0")
	})

	t.Run("key file not found", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		currentDir, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = os.Chdir(currentDir)
		}()

		_, _, _, err = cmd.ExecuteCommandC(New(), "-t", "some-target", "-k", "non-existent-key-file", "refs/heads/main")
		assert.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("invalid target ID format", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		currentDir, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = os.Chdir(currentDir)
		}()

		keyPath := filepath.Join(tmpDir, "key")
		if err := os.WriteFile(keyPath, []byte("key"), 0o600); err != nil {
			t.Fatal(err)
		}

		_, _, _, err = cmd.ExecuteCommandC(New(), "-t", "invalid-hash", "-k", keyPath, "refs/heads/main")
		assert.ErrorContains(t, err, "wrong length")
	})
}
