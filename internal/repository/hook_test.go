// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"os"
	"path"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdatePrePushHook(t *testing.T) {
	t.Run("write hook", func(t *testing.T) {
		tmpDir := t.TempDir()

		repo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)
		r := &Repository{r: repo}

		err = r.UpdateHook(HookPrePush, []byte("some content"), false)
		require.NoError(t, err)

		hookFile := path.Join(tmpDir, ".git", "hooks", "pre-push")
		prepushScript, err := os.ReadFile(hookFile)
		require.NoError(t, err)
		assert.Equal(t, []byte("some content"), prepushScript)
	})

	t.Run("hook exists", func(t *testing.T) {
		tmpDir := t.TempDir()

		repo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)
		r := &Repository{r: repo}

		hookDir := path.Join(tmpDir, ".git", "hooks")
		hookFile := path.Join(hookDir, "pre-push")
		err = os.Mkdir(hookDir, 0o750)
		require.NoError(t, err)
		err = os.WriteFile(hookFile, []byte("existing hook script"), 0o700) // nolint:gosec
		require.NoError(t, err)

		err = r.UpdateHook(HookPrePush, []byte("new hook script"), false)
		var hookErr *ErrHookExists
		if assert.ErrorAs(t, err, &hookErr) {
			assert.Equal(t, HookPrePush, hookErr.HookType)
		}
	})

	t.Run("force overwrite hook", func(t *testing.T) {
		tmpDir := t.TempDir()

		repo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)
		r := &Repository{r: repo}

		hookDir := path.Join(tmpDir, ".git", "hooks")
		hookFile := path.Join(hookDir, "pre-push")
		err = os.Mkdir(hookDir, 0o750)
		require.NoError(t, err)
		err = os.WriteFile(hookFile, []byte("existing hook script"), 0o700) // nolint:gosec
		require.NoError(t, err)

		err = r.UpdateHook(HookPrePush, []byte("new hook script"), true)
		assert.NoError(t, err)

		content, err := os.ReadFile(hookFile)
		assert.NoError(t, err)
		assert.Equal(t, []byte("new hook script"), content)
	})
}
