// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addhooks

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddHooksCommand(t *testing.T) {
	t.Run("install hooks successfully", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		// Change to repo directory
		oldWd, err := os.Getwd()
		require.NoError(t, err)
		defer func() { _ = os.Chdir(oldWd) }()

		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		o := &options{
			hookTypes: []string{"pre-push"}, // Set default value manually since we're not parsing flags
		}
		cmd := New()

		err = o.Run(cmd, []string{})
		require.NoError(t, err)

		// Verify hook was installed
		hookFile := filepath.Join(gitRepo.GetGitDir(), "hooks", "pre-push")
		_, err = os.Stat(hookFile)
		require.NoError(t, err)

		// Verify hook content
		content, err := os.ReadFile(hookFile)
		require.NoError(t, err)
		assert.Contains(t, string(content), "gittuf")
		// Check for platform-appropriate script header
		if runtime.GOOS == "windows" {
			assert.Contains(t, string(content), "@echo off")
		} else {
			assert.Contains(t, string(content), "#!/bin/sh")
		}
	})

	t.Run("install multiple hooks", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		oldWd, err := os.Getwd()
		require.NoError(t, err)
		defer func() { _ = os.Chdir(oldWd) }()

		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		o := &options{
			hookTypes: []string{"pre-push", "pre-commit", "post-commit"},
		}
		cmd := New()

		err = o.Run(cmd, []string{})
		require.NoError(t, err)

		// Verify all hooks were installed
		hookTypes := []string{"pre-push", "pre-commit", "post-commit"}
		for _, hookType := range hookTypes {
			hookFile := filepath.Join(gitRepo.GetGitDir(), "hooks", hookType)
			_, err = os.Stat(hookFile)
			require.NoError(t, err, "Hook %s should be installed", hookType)

			// Verify hook content
			content, err := os.ReadFile(hookFile)
			require.NoError(t, err)
			assert.Contains(t, string(content), "gittuf")
			// Check for platform-appropriate script header
			if runtime.GOOS == "windows" {
				assert.Contains(t, string(content), "@echo off")
			} else {
				assert.Contains(t, string(content), "#!/bin/sh")
			}
		}
	})

	t.Run("list hooks functionality", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		oldWd, err := os.Getwd()
		require.NoError(t, err)
		defer func() { _ = os.Chdir(oldWd) }()

		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		// Install a hook first
		o := &options{
			hookTypes: []string{"pre-push"}, // Set default value manually
		}
		cmd := New()
		err = o.Run(cmd, []string{})
		require.NoError(t, err)

		// Verify hook was created
		hookFile := filepath.Join(gitRepo.GetGitDir(), "hooks", "pre-push")
		_, err = os.Stat(hookFile)
		require.NoError(t, err)

		// Now list hooks
		o = &options{listHooks: true}
		cmd = New()
		err = o.Run(cmd, []string{})
		require.NoError(t, err)
	})

	t.Run("unsupported hook type", func(t *testing.T) {
		tmpDir := t.TempDir()
		_ = gitinterface.CreateTestGitRepository(t, tmpDir, false)

		oldWd, err := os.Getwd()
		require.NoError(t, err)
		defer func() { _ = os.Chdir(oldWd) }()

		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		o := &options{
			hookTypes: []string{"unsupported-hook"},
		}
		cmd := New()

		err = o.Run(cmd, []string{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported hook type")
	})

	t.Run("hook already exists without force", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		oldWd, err := os.Getwd()
		require.NoError(t, err)
		defer func() { _ = os.Chdir(oldWd) }()

		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		// Create existing hook
		hookFile := filepath.Join(gitRepo.GetGitDir(), "hooks", "pre-push")
		err = os.MkdirAll(filepath.Dir(hookFile), 0o755)
		require.NoError(t, err)
		err = os.WriteFile(hookFile, []byte("existing hook"), 0o600)
		require.NoError(t, err)

		o := &options{
			force:     false,
			hookTypes: []string{"pre-push"}, // Set default value manually
		}
		cmd := New()

		err = o.Run(cmd, []string{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("force overwrite existing hook", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		oldWd, err := os.Getwd()
		require.NoError(t, err)
		defer func() { _ = os.Chdir(oldWd) }()

		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		// Create existing hook
		hookFile := filepath.Join(gitRepo.GetGitDir(), "hooks", "pre-push")
		err = os.MkdirAll(filepath.Dir(hookFile), 0o755)
		require.NoError(t, err)
		err = os.WriteFile(hookFile, []byte("existing hook"), 0o600)
		require.NoError(t, err)

		o := &options{
			force:     true,
			hookTypes: []string{"pre-push"}, // Set default value manually
		}
		cmd := New()

		err = o.Run(cmd, []string{})
		require.NoError(t, err)

		// Verify hook was overwritten
		content, err := os.ReadFile(hookFile)
		require.NoError(t, err)
		assert.Contains(t, string(content), "gittuf")
		assert.NotContains(t, string(content), "existing hook")
	})

	t.Run("repository not found", func(t *testing.T) {
		tmpDir := t.TempDir()

		oldWd, err := os.Getwd()
		require.NoError(t, err)
		defer func() { _ = os.Chdir(oldWd) }()

		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		o := &options{}
		cmd := New()

		err = o.Run(cmd, []string{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "repository")
	})

	t.Run("remove hooks functionality", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		oldWd, err := os.Getwd()
		require.NoError(t, err)
		defer func() { _ = os.Chdir(oldWd) }()

		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		// Install hooks first
		o := &options{
			hookTypes: []string{"pre-push", "pre-commit"},
		}
		cmd := New()
		err = o.Run(cmd, []string{})
		require.NoError(t, err)

		// Verify hooks were installed
		prePushHook := filepath.Join(gitRepo.GetGitDir(), "hooks", "pre-push")
		preCommitHook := filepath.Join(gitRepo.GetGitDir(), "hooks", "pre-commit")
		_, err = os.Stat(prePushHook)
		require.NoError(t, err)
		_, err = os.Stat(preCommitHook)
		require.NoError(t, err)

		// Now remove hooks
		o = &options{
			remove:    true,
			hookTypes: []string{"pre-push", "pre-commit"},
		}
		cmd = New()
		err = o.Run(cmd, []string{})
		require.NoError(t, err)

		// Verify hooks were removed
		_, err = os.Stat(prePushHook)
		require.True(t, os.IsNotExist(err))
		_, err = os.Stat(preCommitHook)
		require.True(t, os.IsNotExist(err))
	})

	t.Run("remove non-gittuf hook", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		oldWd, err := os.Getwd()
		require.NoError(t, err)
		defer func() { _ = os.Chdir(oldWd) }()

		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		// Create a non-gittuf hook
		hookFile := filepath.Join(gitRepo.GetGitDir(), "hooks", "pre-push")
		err = os.MkdirAll(filepath.Dir(hookFile), 0o755)
		require.NoError(t, err)
		err = os.WriteFile(hookFile, []byte("#!/bin/sh\necho 'custom hook'"), 0o600)
		require.NoError(t, err)

		// Try to remove it
		o := &options{
			remove:    true,
			hookTypes: []string{"pre-push"},
		}
		cmd := New()
		err = o.Run(cmd, []string{})
		require.NoError(t, err)

		// Verify hook still exists (should not be removed since it's not a gittuf hook)
		_, err = os.Stat(hookFile)
		require.NoError(t, err)
	})

	t.Run("remove non-existent hook", func(t *testing.T) {
		tmpDir := t.TempDir()
		_ = gitinterface.CreateTestGitRepository(t, tmpDir, false)

		oldWd, err := os.Getwd()
		require.NoError(t, err)
		defer func() { _ = os.Chdir(oldWd) }()

		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		// Try to remove a hook that doesn't exist
		o := &options{
			remove:    true,
			hookTypes: []string{"pre-push"},
		}
		cmd := New()
		err = o.Run(cmd, []string{})
		require.NoError(t, err) // Should not error, just skip
	})
}
