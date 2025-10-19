// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetGitConfig(t *testing.T) {
	tmpDir := t.TempDir()
	repo := CreateTestGitRepository(t, tmpDir, false)

	// CreateTestGitRepository sets our test config
	config, err := repo.GetGitConfig()
	assert.Nil(t, err)
	assert.Equal(t, testName, config["user.name"])
	assert.Equal(t, testEmail, config["user.email"])
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

		config, err := repo.GetGitConfig()
		require.NoError(t, err)
		assert.Equal(t, name, config["user.name"])
		assert.Equal(t, email, config["user.email"])
	})
	t.Run("empty set", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := CreateTestGitRepository(t, tmpDir, false)

		err := repo.SetGitConfig("user.name", "")
		require.NoError(t, err)
		err = repo.SetGitConfig("user.email", "")
		require.NoError(t, err)

		config, err := repo.GetGitConfig()
		require.NoError(t, err)
		assert.Equal(t, "", config["user.name"])
		assert.Equal(t, "", config["user.email"])
	})
	t.Run("gpg.format special case", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := CreateTestGitRepository(t, tmpDir, false)

		err := repo.SetGitConfig("gpg.format", "gpg")
		require.NoError(t, err)

		config, err := repo.GetGitConfig()
		require.NoError(t, err)
		assert.Equal(t, "gpg", config["gpg.format"])

		err = repo.SetGitConfig("gpg.format", "")
		require.NoError(t, err)

		config, err = repo.GetGitConfig()
		require.NoError(t, err)
		assert.Equal(t, "", config["gpg.format"])
	})
}
