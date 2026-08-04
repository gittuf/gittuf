// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"testing"

	"github.com/gittuf/gittuf/pkg/gitstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookupConfig(t *testing.T) {
	tmpDir := t.TempDir()
	repo := CreateTestGitRepository(t, tmpDir, false)

	// CreateTestGitRepository sets our test config
	name, ok, err := repo.LookupConfig(gitstore.ConfigUserName)
	assert.Nil(t, err)
	assert.True(t, ok)
	assert.Equal(t, testName, name)

	email, ok, err := repo.LookupConfig(gitstore.ConfigUserEmail)
	assert.Nil(t, err)
	assert.True(t, ok)
	assert.Equal(t, testEmail, email)

	_, ok, err = repo.LookupConfig("does.not.exist")
	assert.Nil(t, err)
	assert.False(t, ok)
}

func TestSetGitConfig(t *testing.T) {
	t.Run("basic sets", func(t *testing.T) {
		const name = "John Doe"
		const email = "john.doe@example.com"

		tmpDir := t.TempDir()
		repo := CreateTestGitRepository(t, tmpDir, false)

		err := repo.SetGitConfig("user.name", name)
		require.NoError(t, err)
		err = repo.SetGitConfig("user.email", email)
		require.NoError(t, err)

		gotName, ok, err := repo.LookupConfig(gitstore.ConfigUserName)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, name, gotName)
		gotEmail, ok, err := repo.LookupConfig(gitstore.ConfigUserEmail)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, email, gotEmail)
	})
	t.Run("empty set", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := CreateTestGitRepository(t, tmpDir, false)

		err := repo.SetGitConfig("user.name", "")
		require.NoError(t, err)
		err = repo.SetGitConfig("user.email", "")
		require.NoError(t, err)

		gotName, ok, err := repo.LookupConfig(gitstore.ConfigUserName)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, "", gotName)
		gotEmail, ok, err := repo.LookupConfig(gitstore.ConfigUserEmail)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, "", gotEmail)
	})
	t.Run("gpg.format special case", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := CreateTestGitRepository(t, tmpDir, false)

		err := repo.SetGitConfig("gpg.format", "gpg")
		require.NoError(t, err)

		format, ok, err := repo.LookupConfig(gitstore.ConfigGPGFormat)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, "gpg", format)

		// A set-but-empty key returns "" with ok true, distinct from unset.
		err = repo.SetGitConfig("gpg.format", "")
		require.NoError(t, err)

		format, ok, err = repo.LookupConfig(gitstore.ConfigGPGFormat)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, "", format)
	})

	t.Run("invalid key", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := CreateTestGitRepository(t, tmpDir, false)

		err := repo.SetGitConfig("", "value")
		assert.ErrorContains(t, err, "unable to set")
	})
}
