// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"path/filepath"
	"sync"
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
}

func TestLoadRepositoryConcurrent(t *testing.T) {
	// LoadRepository must not change the process working directory; concurrent
	// callers for different paths must each resolve their own gitDirPath.
	dirA := t.TempDir()
	dirB := t.TempDir()
	_ = CreateTestGitRepository(t, dirA, true)
	_ = CreateTestGitRepository(t, dirB, false)

	wantA, err := filepath.EvalSymlinks(dirA)
	require.NoError(t, err)
	wantB, err := filepath.EvalSymlinks(filepath.Join(dirB, ".git"))
	require.NoError(t, err)

	const iterations = 20
	var wg sync.WaitGroup
	for i := 0; i < iterations; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			r, err := LoadRepository(dirA)
			if assert.NoError(t, err) {
				assert.Equal(t, wantA, r.GetGitDir())
			}
		}()
		go func() {
			defer wg.Done()
			r, err := LoadRepository(dirB)
			if assert.NoError(t, err) {
				assert.Equal(t, wantB, r.GetGitDir())
			}
		}()
	}
	wg.Wait()
}
