// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"os"
	"path/filepath"
	"testing"

	gitinterfaceopts "github.com/gittuf/gittuf/internal/gitinterface/options/gitinterface"
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

	t.Run("with specified path", func(t *testing.T) {
		tmpDir := t.TempDir()
		currentDir, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		CreateTestGitRepository(t, tmpDir, false)
		repo, err := LoadRepository(gitinterfaceopts.WithRepositoryPath(tmpDir))
		assert.Nil(t, err)

		expectedPath, err := filepath.Abs(filepath.Join(tmpDir, ".git"))
		require.Nil(t, err)
		actualPath, err := filepath.Abs(repo.GetGitDir())
		require.Nil(t, err)
		assert.Equal(t, expectedPath, actualPath)
	})
}
