// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addhooks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestAddHooks(t *testing.T) {
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

		_, _, _, err = cmd.ExecuteCommandC(New())
		assert.ErrorContains(t, err, "unable to identify git directory")
	})

	t.Run("success", func(t *testing.T) {
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

		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		_, _, _, err = cmd.ExecuteCommandC(New())
		assert.NoError(t, err)

		hookPath := filepath.Join(".git", "hooks", "pre-push")
		content, err := os.ReadFile(hookPath)
		assert.NoError(t, err)
		assert.Equal(t, prePushScript, content)
	})

	t.Run("already exists", func(t *testing.T) {
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

		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		// Create a dummy hook file to simulate an existing hook
		hookPath := filepath.Join(".git", "hooks")
		if err := os.MkdirAll(hookPath, 0755); err != nil {
			t.Fatal(err)
		}
		dummyHookPath := filepath.Join(hookPath, "pre-push")
		if err := os.WriteFile(dummyHookPath, []byte("dummy hook content"), 0o600); err != nil {
			t.Fatal(err)
		}

		_, _, stdErr, err := cmd.ExecuteCommandC(New())
		assert.ErrorContains(t, err, "already exists")

		expectedWarning := "'pre-push' already exists. Use --force flag"
		assert.Contains(t, stdErr.String(), expectedWarning)
	})

	t.Run("force overwrite", func(t *testing.T) {
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

		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		// Create a dummy hook file to simulate an existing hook
		hookPath := filepath.Join(".git", "hooks")
		if err := os.MkdirAll(hookPath, 0755); err != nil {
			t.Fatal(err)
		}
		dummyHookPath := filepath.Join(hookPath, "pre-push")
		if err := os.WriteFile(dummyHookPath, []byte("dummy hook content"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, _, _, err = cmd.ExecuteCommandC(New(), "--force")
		assert.NoError(t, err)

		content, err := os.ReadFile(dummyHookPath)
		assert.NoError(t, err)
		assert.Equal(t, prePushScript, content)
	})
}
