// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"path/filepath"
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
		})

		t.Run("bare=false", func(t *testing.T) {
			tmpDir := t.TempDir()
			repo := CreateTestGitRepository(t, tmpDir, false)
			assert.False(t, repo.IsBare())
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

	t.Run("empty path", func(t *testing.T) {
		_, err := LoadRepository("")
		assert.ErrorIs(t, err, ErrRepositoryPathNotSpecified)
	})

	t.Run("invalid path", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, err := LoadRepository(tmpDir)
		assert.ErrorContains(t, err, "unable to identify git directory for repository")
	})

	t.Run("GetGoGitRepository", func(t *testing.T) {
		for _, bare := range []bool{false, true} {
			name := "worktree"
			if bare {
				name = "bare"
			}
			t.Run(name, func(t *testing.T) {
				tmpDir := t.TempDir()
				repo := CreateTestGitRepository(t, tmpDir, bare)
				ggr, err := repo.GetGoGitRepository()
				require.NoError(t, err, "GetGoGitRepository on %s repo", name)
				_, err = ggr.Head()
				// Empty repo: ErrReferenceNotFound is fine; what matters is the
				// repo opened. Anything else (esp. ErrRepositoryNotExists) is a
				// failure to open the storage.
				if err != nil {
					assert.ErrorContains(t, err, "reference not found")
				}
			})
		}
	})

	t.Run("GetGoGitRepository on .git-suffixed bare repo", func(t *testing.T) {
		// Forge-style bare repos are conventionally named <name>.git.
		dir := filepath.Join(t.TempDir(), "demo.git")
		repo := CreateTestGitRepository(t, dir, true)
		_, err := repo.GetGoGitRepository()
		require.NoError(t, err)
	})
}
