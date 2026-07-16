// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"os"
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
		if _, has, err := findGitDirPath(tmpDir); err == nil && has {
			tmpDir = filepath.Join(tmpDir, "invalid-repository")
			require.Nil(t, os.Mkdir(tmpDir, 0o700))
			require.Nil(t, os.WriteFile(filepath.Join(tmpDir, ".git"), []byte("not a gitdir file"), 0o600))
		}

		_, err := LoadRepository(tmpDir)
		assert.Error(t, err)
	})
}

func TestEnsureNoCompatObjectFormat(t *testing.T) {
	t.Run("no compat object format", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := CreateTestGitRepository(t, tmpDir, false, WithSHA256Format())

		assert.Nil(t, repo.ensureNoCompatObjectFormat())
	})

	t.Run("compat object format", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := CreateTestGitRepository(t, tmpDir, false, WithSHA256Format())

		require.Nil(t, repo.SetGitConfig("extensions.compatObjectFormat", "sha1"))

		assert.ErrorIs(t, repo.ensureNoCompatObjectFormat(), ErrCompatObjectFormatUnsupported)

		_, err := LoadRepository(tmpDir)
		assert.ErrorIs(t, err, ErrCompatObjectFormatUnsupported)
	})

	t.Run("missing config", func(t *testing.T) {
		repo := &Repository{gitDirPath: t.TempDir()}

		err := repo.ensureNoCompatObjectFormat()
		assert.ErrorContains(t, err, "unable to read repository config")
	})

	t.Run("invalid config", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.Nil(t, os.WriteFile(filepath.Join(tmpDir, "config"), []byte("[extensions\n"), 0o600))

		repo := &Repository{gitDirPath: tmpDir}
		err := repo.ensureNoCompatObjectFormat()
		assert.ErrorContains(t, err, "unable to parse repository config")
	})
}

func TestFindGitDirPath(t *testing.T) {
	t.Run("worktree git directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		_ = CreateTestGitRepository(t, tmpDir, false)

		nestedDir := filepath.Join(tmpDir, "nested", "dir")
		require.Nil(t, os.MkdirAll(nestedDir, 0o700))

		gitDirPath, has, err := findGitDirPath(nestedDir)
		require.Nil(t, err)
		assert.True(t, has)
		assert.Equal(t, filepath.Join(tmpDir, ".git"), gitDirPath)
	})

	t.Run("bare git directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		_ = CreateTestGitRepository(t, tmpDir, true)

		gitDirPath, has, err := findGitDirPath(tmpDir)
		require.Nil(t, err)
		assert.True(t, has)
		assert.Equal(t, tmpDir, gitDirPath)
	})

	t.Run("gitdir file", func(t *testing.T) {
		tmpDir := t.TempDir()
		worktreePath := filepath.Join(tmpDir, "worktree")
		require.Nil(t, os.MkdirAll(worktreePath, 0o700))

		gitDirPath := filepath.Join(tmpDir, "actual.git")
		require.Nil(t, os.WriteFile(filepath.Join(worktreePath, ".git"), []byte("gitdir: ../actual.git\n"), 0o600))

		gotGitDirPath, has, err := findGitDirPath(worktreePath)
		require.Nil(t, err)
		assert.True(t, has)
		assert.Equal(t, gitDirPath, gotGitDirPath)
	})

	t.Run("invalid gitdir file", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.Nil(t, os.WriteFile(filepath.Join(tmpDir, ".git"), []byte("not a gitdir file"), 0o600))

		_, has, err := findGitDirPath(tmpDir)
		assert.False(t, has)
		assert.ErrorContains(t, err, "invalid gitdir file")
	})

	t.Run("no git directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		if _, has, err := findGitDirPath(tmpDir); err == nil && has {
			tmpDir = "/dev"
			if _, has, err := findGitDirPath(tmpDir); err != nil || has {
				t.Skip("unable to find a filesystem path outside a Git repository")
			}
		}

		_, has, err := findGitDirPath(tmpDir)
		require.Nil(t, err)
		assert.False(t, has)
	})
}

func TestReadGitDirFile(t *testing.T) {
	t.Run("absolute gitdir path", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitDirPath := filepath.Join(tmpDir, "actual.git")
		gitDirFilePath := filepath.Join(tmpDir, ".git")
		require.Nil(t, os.WriteFile(gitDirFilePath, []byte("gitdir: "+gitDirPath+"\n"), 0o600))

		gotGitDirPath, err := readGitDirFile(gitDirFilePath, tmpDir)
		require.Nil(t, err)
		assert.Equal(t, gitDirPath, gotGitDirPath)
	})

	t.Run("missing gitdir file", func(t *testing.T) {
		_, err := readGitDirFile(filepath.Join(t.TempDir(), ".git"), t.TempDir())
		assert.Error(t, err)
	})
}

func TestIsBareGitDir(t *testing.T) {
	t.Run("config without head", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.Nil(t, os.WriteFile(filepath.Join(tmpDir, "config"), nil, 0o600))

		assert.False(t, isBareGitDir(tmpDir))
	})

	t.Run("head as directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.Nil(t, os.WriteFile(filepath.Join(tmpDir, "config"), nil, 0o600))
		require.Nil(t, os.Mkdir(filepath.Join(tmpDir, "HEAD"), 0o700))

		assert.False(t, isBareGitDir(tmpDir))
	})
}

func TestRepositoryObjectFormat(t *testing.T) {
	t.Run("sha1", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := CreateTestGitRepository(t, tmpDir, false, WithObjectFormat(ObjectFormatSHA1))
		assert.Equal(t, ObjectFormatSHA1, repo.GetObjectFormat())
		assert.Equal(t, "0000000000000000000000000000000000000000", repo.ZeroHash().String())

		loaded, err := LoadRepository(tmpDir)
		require.Nil(t, err)
		assert.Equal(t, ObjectFormatSHA1, loaded.GetObjectFormat())
	})

	t.Run("sha256", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := CreateTestGitRepository(t, tmpDir, false, WithSHA256Format())
		assert.Equal(t, ObjectFormatSHA256, repo.GetObjectFormat())
		assert.Equal(t, "0000000000000000000000000000000000000000000000000000000000000000", repo.ZeroHash().String())

		loaded, err := LoadRepository(tmpDir)
		require.Nil(t, err)
		assert.Equal(t, ObjectFormatSHA256, loaded.GetObjectFormat())
	})
}
