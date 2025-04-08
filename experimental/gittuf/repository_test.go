// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"os"
	"path/filepath"
	"testing"

	repositoryopts "github.com/gittuf/gittuf/experimental/gittuf/options/repository"
	"github.com/gittuf/gittuf/internal/gitinterface"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadRepository(t *testing.T) {
	t.Run("load with no repo, unsuccessful", func(t *testing.T) {
		tmpDir := t.TempDir()
		currentDir, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		repository, err := LoadRepository()
		assert.NotNil(t, err)
		assert.Nil(t, repository)
	})

	t.Run("successful load", func(t *testing.T) {
		tmpDir := t.TempDir()
		currentDir, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		gitinterface.CreateTestGitRepository(t, tmpDir, false)
		repository, err := LoadRepository()
		assert.Nil(t, err)
		assert.NotNil(t, repository.r)
	})

	t.Run("load with incorrect path, unsuccessful", func(t *testing.T) {
		tmpDir := t.TempDir()
		currentDir, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		_, err = LoadRepository(repositoryopts.WithRepositoryPath(tmpDir))
		assert.ErrorContains(t, err, "unable to identify GIT_DIR")
	})

	t.Run("load with path, successful", func(t *testing.T) {
		tmpDir := t.TempDir()
		currentDir, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		gitinterface.CreateTestGitRepository(t, tmpDir, false)
		repository, err := LoadRepository(repositoryopts.WithRepositoryPath(tmpDir))
		assert.Nil(t, err)

		expectedPath, err := filepath.Abs(filepath.Join(tmpDir, ".git"))
		require.Nil(t, err)
		actualPath, err := filepath.Abs(repository.r.GetGitDir())
		require.Nil(t, err)
		assert.Equal(t, expectedPath, actualPath)
	})
}

func TestUnauthorizedKey(t *testing.T) {
	tempDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

	rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	targetsSigner := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)

	r := &Repository{r: repo}
	if err := r.InitializeRoot(testCtx, rootSigner, false); err != nil {
		t.Fatal(err)
	}

	t.Run("test add targets key", func(t *testing.T) {
		key := tufv01.NewKeyFromSSLibKey(targetsSigner.MetadataKey())

		err := r.AddTopLevelTargetsKey(testCtx, targetsSigner, key, false)
		assert.ErrorIs(t, err, ErrUnauthorizedKey)
	})

	t.Run("test remove targets key", func(t *testing.T) {
		err := r.RemoveTopLevelTargetsKey(testCtx, targetsSigner, "some key ID", false)
		assert.ErrorIs(t, err, ErrUnauthorizedKey)
	})
}

func TestGetGitRepository(t *testing.T) {
	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	r := &Repository{r: repo}
	result := r.GetGitRepository()
	assert.Equal(t, repo, result)
}
