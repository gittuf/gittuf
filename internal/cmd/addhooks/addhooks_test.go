// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addhooks

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestAddHooks(t *testing.T) {
	t.Run("install hooks successfully", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		// Change to repo directory
		oldWd, err := os.Getwd()
		assert.Nil(t, err)
		defer func() { _ = os.Chdir(oldWd) }()

		err = os.Chdir(tmpDir)
		assert.Nil(t, err)

		o := &options{
			hookTypes: []string{"pre-push"}, // Set default value manually since we're not parsing flags
		}
		cmd := New()

		err = o.Run(cmd, []string{})
		assert.Nil(t, err)

		// Verify hook was installed
		hookFile := filepath.Join(gitRepo.GetGitDir(), "hooks", "pre-push")
		_, err = os.Stat(hookFile)
		assert.Nil(t, err)

		// Verify hook content
		content, err := os.ReadFile(hookFile)
		assert.Nil(t, err)
		assert.Contains(t, string(content), "gittuf")
		// Check for platform-appropriate script header
		if runtime.GOOS == "windows" {
			assert.Contains(t, string(content), "@echo off")
		} else {
			assert.Contains(t, string(content), "#!/bin/sh")
		}
	})

	t.Run("list hooks functionality", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		oldWd, err := os.Getwd()
		assert.Nil(t, err)
		defer func() { _ = os.Chdir(oldWd) }()

		err = os.Chdir(tmpDir)
		assert.Nil(t, err)

		// Install a hook first
		o := &options{
			hookTypes: []string{"pre-push"}, // Set default value manually
		}
		cmd := New()
		err = o.Run(cmd, []string{})
		assert.Nil(t, err)

		// Verify hook was created
		hookFile := filepath.Join(gitRepo.GetGitDir(), "hooks", "pre-push")
		_, err = os.Stat(hookFile)
		assert.Nil(t, err)

		// Now list hooks
		o = &options{listHooks: true}
		cmd = New()
		err = o.Run(cmd, []string{})
		assert.Nil(t, err)
	})

	t.Run("unsupported hook type", func(t *testing.T) {
		tmpDir := t.TempDir()
		_ = gitinterface.CreateTestGitRepository(t, tmpDir, false)

		oldWd, err := os.Getwd()
		assert.Nil(t, err)
		defer func() { _ = os.Chdir(oldWd) }()

		err = os.Chdir(tmpDir)
		assert.Nil(t, err)

		o := &options{
			hookTypes: []string{"unsupported-hook"},
		}
		cmd := New()

		err = o.Run(cmd, []string{})
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "unsupported hook type")
	})

	t.Run("hook already exists without force", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		oldWd, err := os.Getwd()
		assert.Nil(t, err)
		defer func() { _ = os.Chdir(oldWd) }()

		err = os.Chdir(tmpDir)
		assert.Nil(t, err)

		// Create existing hook
		hookFile := filepath.Join(gitRepo.GetGitDir(), "hooks", "pre-push")
		err = os.MkdirAll(filepath.Dir(hookFile), 0o755)
		assert.Nil(t, err)
		err = os.WriteFile(hookFile, []byte("existing hook"), 0o600)
		assert.Nil(t, err)

		o := &options{
			force:     false,
			hookTypes: []string{"pre-push"}, // Set default value manually
		}
		cmd := New()

		err = o.Run(cmd, []string{})
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("force overwrite existing hook", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		oldWd, err := os.Getwd()
		assert.Nil(t, err)
		defer func() { _ = os.Chdir(oldWd) }()

		err = os.Chdir(tmpDir)
		assert.Nil(t, err)

		// Create existing hook
		hookFile := filepath.Join(gitRepo.GetGitDir(), "hooks", "pre-push")
		err = os.MkdirAll(filepath.Dir(hookFile), 0o755)
		assert.Nil(t, err)
		err = os.WriteFile(hookFile, []byte("existing hook"), 0o600)
		assert.Nil(t, err)

		o := &options{
			force:     true,
			hookTypes: []string{"pre-push"}, // Set default value manually
		}
		cmd := New()

		err = o.Run(cmd, []string{})
		assert.Nil(t, err)

		// Verify hook was overwritten
		content, err := os.ReadFile(hookFile)
		assert.Nil(t, err)
		assert.Contains(t, string(content), "gittuf")
		assert.NotContains(t, string(content), "existing hook")
	})

	t.Run("repository not found", func(t *testing.T) {
		tmpDir := t.TempDir()

		oldWd, err := os.Getwd()
		assert.Nil(t, err)
		defer func() { _ = os.Chdir(oldWd) }()

		err = os.Chdir(tmpDir)
		assert.Nil(t, err)

		o := &options{}
		cmd := New()

		err = o.Run(cmd, []string{})
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "repository")
	})
}

func TestInstallHooks(t *testing.T) {
	t.Run("install pre-push hook", func(t *testing.T) {
		tmpDir := t.TempDir()
		_ = gitinterface.CreateTestGitRepository(t, tmpDir, false)

		oldWd, err := os.Getwd()
		assert.Nil(t, err)
		defer func() { _ = os.Chdir(oldWd) }()

		err = os.Chdir(tmpDir)
		assert.Nil(t, err)

		repo, err := gittuf.LoadRepository(".")
		assert.Nil(t, err)

		cmd := New()
		err = installHooks(cmd, repo, []string{"pre-push"}, false)
		assert.Nil(t, err)
	})

	t.Run("unsupported hook type", func(t *testing.T) {
		tmpDir := t.TempDir()
		_ = gitinterface.CreateTestGitRepository(t, tmpDir, false)

		oldWd, err := os.Getwd()
		assert.Nil(t, err)
		defer func() { _ = os.Chdir(oldWd) }()

		err = os.Chdir(tmpDir)
		assert.Nil(t, err)

		repo, err := gittuf.LoadRepository(".")
		assert.Nil(t, err)

		cmd := New()
		err = installHooks(cmd, repo, []string{"unsupported"}, false)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "unsupported hook type")
	})
}

func TestListInstalledHooks(t *testing.T) {
	t.Run("list hooks", func(t *testing.T) {
		tmpDir := t.TempDir()
		_ = gitinterface.CreateTestGitRepository(t, tmpDir, false)

		oldWd, err := os.Getwd()
		assert.Nil(t, err)
		defer func() { _ = os.Chdir(oldWd) }()

		err = os.Chdir(tmpDir)
		assert.Nil(t, err)

		repo, err := gittuf.LoadRepository(".")
		assert.Nil(t, err)

		cmd := New()
		err = listInstalledHooks(cmd, repo)
		assert.Nil(t, err)
	})
}
