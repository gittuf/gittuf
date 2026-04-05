// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepository(t *testing.T) {
	t.Run("repository.isBare", func(t *testing.T) {
		t.Run("bare=true", func(t *testing.T) {
			tmpDir := t.TempDir()
			repo := CreateTestGitRepository(t, tmpDir, true)
			assert.True(t, repo.IsBare())
			gitDir := repo.GetGitDir()
			assert.False(t, strings.HasSuffix(gitDir, ".git"))
		})

		t.Run("bare=false", func(t *testing.T) {
			tmpDir := t.TempDir()
			repo := CreateTestGitRepository(t, tmpDir, false)
			assert.False(t, repo.IsBare())
			gitDir := repo.GetGitDir()
			assert.True(t, strings.HasSuffix(gitDir, ".git"))
		})
	})

	t.Run("with specified path, not bare", func(t *testing.T) {
		tmpDir := t.TempDir()

		_ = CreateTestGitRepository(t, tmpDir, false)
		repo, err := LoadRepository(tmpDir)
		assert.Nil(t, err)

		expectedPath, err := filepath.EvalSymlinks(filepath.Join(tmpDir, ".git"))
		require.Nil(t, err)
		actualPath, err := filepath.EvalSymlinks(repo.GetGitDir())
		require.Nil(t, err)
		assert.Equal(t, expectedPath, actualPath)
	})

	t.Run("with specified path, is bare", func(t *testing.T) {
		tmpDir := t.TempDir()

		_ = CreateTestGitRepository(t, tmpDir, true)
		repo, err := LoadRepository(tmpDir)
		assert.Nil(t, err)

		expectedPath, err := filepath.EvalSymlinks(tmpDir)
		require.Nil(t, err)
		actualPath, err := filepath.EvalSymlinks(repo.GetGitDir())
		require.Nil(t, err)
		assert.Equal(t, expectedPath, actualPath)
	})

	t.Run("load repository errors", func(t *testing.T) {
		_, err := LoadRepository("")
		assert.ErrorIs(t, err, ErrRepositoryPathNotSpecified)

		_, err = LoadRepository("/nonexistent/path/to/repo")
		assert.NotNil(t, err)

		tmpDir := t.TempDir()
		_, err = LoadRepository(tmpDir)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "unable to identify git directory")
	})

	t.Run("load from subdirectory", func(t *testing.T) {
		tmpDir := t.TempDir()
		_ = CreateTestGitRepository(t, tmpDir, false)

		subDir := filepath.Join(tmpDir, "subdir")
		err := os.Mkdir(subDir, 0o755)
		require.Nil(t, err)

		repo, err := LoadRepository(tmpDir)
		assert.Nil(t, err)
		assert.NotNil(t, repo)
	})

	t.Run("get go-git repository", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := CreateTestGitRepository(t, tmpDir, false)

		goGitRepo, err := repo.GetGoGitRepository()
		assert.Nil(t, err)
		assert.NotNil(t, goGitRepo)
	})

	t.Run("executor with environment", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := CreateTestGitRepository(t, tmpDir, false)

		exec := repo.executor("config", "user.name").withEnv("TEST_VAR=test_value")
		assert.NotNil(t, exec)
		assert.Contains(t, exec.env, "TEST_VAR=test_value")

		exec = repo.executor("config", "user.name").
			withEnv("VAR1=value1", "VAR2=value2", "VAR3=value3")
		assert.NotNil(t, exec)
		assert.Contains(t, exec.env, "VAR1=value1")
		assert.Contains(t, exec.env, "VAR2=value2")
		assert.Contains(t, exec.env, "VAR3=value3")
	})

	t.Run("executor without git dir", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := CreateTestGitRepository(t, tmpDir, false)

		exec := repo.executor("version").withoutGitDir()
		assert.NotNil(t, exec)
		assert.True(t, exec.unsetGitDir)

		output, err := exec.executeString()
		assert.Nil(t, err)
		assert.Contains(t, output, "git version")
	})

	t.Run("executor execute string", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := CreateTestGitRepository(t, tmpDir, false)

		output, err := repo.executor("config", "user.name").executeString()
		assert.Nil(t, err)
		assert.NotEmpty(t, output)
		assert.Equal(t, strings.TrimSpace(output), output)

		_, err = repo.executor("config", "nonexistent.key").executeString()
		assert.NotNil(t, err)
	})

	t.Run("executor chaining", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := CreateTestGitRepository(t, tmpDir, false)

		exec := repo.executor("version").
			withoutGitDir().
			withEnv("TEST=value")

		assert.NotNil(t, exec)
		assert.True(t, exec.unsetGitDir)
		assert.Contains(t, exec.env, "TEST=value")

		output, err := exec.executeString()
		assert.Nil(t, err)
		assert.Contains(t, output, "git version")
	})

	t.Run("repository clock behavior", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := CreateTestGitRepository(t, tmpDir, false)

		assert.NotNil(t, repo.clock)
		assert.NotNil(t, repo.clock.Now())
	})
}
