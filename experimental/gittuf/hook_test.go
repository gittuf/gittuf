// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/gittuf/gittuf/pkg/gitstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdatePrePushHook(t *testing.T) {
	t.Run("write hook", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		r := &Repository{r: newStorer(repo)}

		err := r.UpdateGitHook(HookPrePush, []byte("some content"), false)
		require.NoError(t, err)

		hookFile := filepath.Join(repo.GetGitDir(), "hooks", "pre-push")
		prepushScript, err := os.ReadFile(hookFile)
		require.NoError(t, err)
		assert.Equal(t, []byte("some content"), prepushScript)
	})

	t.Run("hook exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		r := &Repository{r: newStorer(repo)}

		hookFile := filepath.Join(repo.GetGitDir(), "hooks", "pre-push")
		err := os.WriteFile(hookFile, []byte("existing hook script"), 0o700) // nolint:gosec
		require.NoError(t, err)

		err = r.UpdateGitHook(HookPrePush, []byte("new hook script"), false)
		var hookErr *ErrHookExists
		if assert.ErrorAs(t, err, &hookErr) {
			assert.Equal(t, HookPrePush, hookErr.HookType)
		}
	})

	t.Run("force overwrite hook", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		r := &Repository{r: newStorer(repo)}

		hookFile := filepath.Join(repo.GetGitDir(), "hooks", "pre-push")
		err := os.WriteFile(hookFile, []byte("existing hook script"), 0o700) // nolint:gosec
		require.NoError(t, err)

		err = r.UpdateGitHook(HookPrePush, []byte("new hook script"), true)
		assert.NoError(t, err)

		content, err := os.ReadFile(hookFile)
		assert.NoError(t, err)
		assert.Equal(t, []byte("new hook script"), content)
	})

	t.Run("write hook with relative core.hooksPath", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		require.NoError(t, repo.SetGitConfig("core.hooksPath", ".githooks"))

		r := &Repository{r: newStorer(repo)}

		err := r.UpdateGitHook(HookPrePush, []byte("custom hook content"), false)
		require.NoError(t, err)

		customHookFile := filepath.Join(tmpDir, ".githooks", "pre-push")
		content, err := os.ReadFile(customHookFile)
		require.NoError(t, err)
		assert.Equal(t, []byte("custom hook content"), content)

		// Ensure hook was not written to standard .git/hooks directory
		defaultHookFile := filepath.Join(repo.GetGitDir(), "hooks", "pre-push")
		_, err = os.Stat(defaultHookFile)
		assert.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("write hook with absolute core.hooksPath", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)
		customHooksDir := t.TempDir()

		require.NoError(t, repo.SetGitConfig("core.hooksPath", customHooksDir))

		r := &Repository{r: newStorer(repo)}

		err := r.UpdateGitHook(HookPrePush, []byte("abs hook content"), false)
		require.NoError(t, err)

		customHookFile := filepath.Join(customHooksDir, "pre-push")
		content, err := os.ReadFile(customHookFile)
		require.NoError(t, err)
		assert.Equal(t, []byte("abs hook content"), content)
	})

	t.Run("hook exists with custom core.hooksPath", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		require.NoError(t, repo.SetGitConfig("core.hooksPath", ".githooks"))

		r := &Repository{r: newStorer(repo)}

		customHooksDir := filepath.Join(tmpDir, ".githooks")
		require.NoError(t, os.MkdirAll(customHooksDir, 0o750))
		customHookFile := filepath.Join(customHooksDir, "pre-push")
		require.NoError(t, os.WriteFile(customHookFile, []byte("existing custom hook"), 0o700)) // nolint:gosec

		err := r.UpdateGitHook(HookPrePush, []byte("new custom hook"), false)
		var hookErr *ErrHookExists
		if assert.ErrorAs(t, err, &hookErr) {
			assert.Equal(t, HookPrePush, hookErr.HookType)
		}

		// Force overwrite succeeds
		err = r.UpdateGitHook(HookPrePush, []byte("new custom hook"), true)
		assert.NoError(t, err)

		content, err := os.ReadFile(customHookFile)
		assert.NoError(t, err)
		assert.Equal(t, []byte("new custom hook"), content)
	})

	t.Run("core.hooksPath set to /dev/null", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		require.NoError(t, repo.SetGitConfig(string(gitstore.ConfigCoreHooksPath), "/dev/null"))

		r := &Repository{r: newStorer(repo)}

		err := r.UpdateGitHook(HookPrePush, []byte("custom hook content"), false)
		assert.ErrorIs(t, err, ErrHooksDisabled)

		// Force overwrite also fails when hooks are disabled
		err = r.UpdateGitHook(HookPrePush, []byte("custom hook content"), true)
		assert.ErrorIs(t, err, ErrHooksDisabled)
	})
}
