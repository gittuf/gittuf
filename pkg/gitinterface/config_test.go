// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"fmt"
	"strings"
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

	t.Run("get config with multiple values", func(t *testing.T) {
		err := repo.SetGitConfig("test.key1", "value1")
		require.NoError(t, err)
		err = repo.SetGitConfig("test.key2", "value2")
		require.NoError(t, err)
		err = repo.SetGitConfig("test.key3", "value3")
		require.NoError(t, err)

		config, err := repo.GetGitConfig()
		assert.Nil(t, err)
		assert.Equal(t, "value1", config["test.key1"])
		assert.Equal(t, "value2", config["test.key2"])
		assert.Equal(t, "value3", config["test.key3"])
	})

	t.Run("get config with special characters in value", func(t *testing.T) {
		err := repo.SetGitConfig("test.special", "value with spaces and !@#$%")
		require.NoError(t, err)

		config, err := repo.GetGitConfig()
		assert.Nil(t, err)
		assert.Equal(t, "value with spaces and !@#$%", config["test.special"])
	})

	t.Run("get config with empty value", func(t *testing.T) {
		err := repo.SetGitConfig("test.empty", "")
		require.NoError(t, err)

		config, err := repo.GetGitConfig()
		assert.Nil(t, err)
		assert.Equal(t, "", config["test.empty"])
	})

	t.Run("config keys are lowercase", func(t *testing.T) {
		err := repo.SetGitConfig("Test.Key", "value")
		require.Nil(t, err)

		config, err := repo.GetGitConfig()
		assert.Nil(t, err)
		assert.Contains(t, config, "test.key")
		assert.Equal(t, "value", config["test.key"])
	})

	t.Run("get config after multiple sets", func(t *testing.T) {
		keys := []string{"test.m1", "test.m2", "test.m3"}
		for i, key := range keys {
			err := repo.SetGitConfig(key, fmt.Sprintf("value%d", i))
			require.Nil(t, err)
		}

		config, err := repo.GetGitConfig()
		assert.Nil(t, err)

		for i, key := range keys {
			assert.Equal(t, fmt.Sprintf("value%d", i), config[key])
		}
	})

	t.Run("config with very long value", func(t *testing.T) {
		longValue := strings.Repeat("a", 500)
		err := repo.SetGitConfig("test.long", longValue)
		assert.Nil(t, err)

		config, err := repo.GetGitConfig()
		assert.Nil(t, err)
		assert.Equal(t, longValue, config["test.long"])
	})
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

	t.Run("set config with dots in key", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := CreateTestGitRepository(t, tmpDir, false)

		err := repo.SetGitConfig("section.subsection.key", "value")
		require.NoError(t, err)

		config, err := repo.GetGitConfig()
		assert.Nil(t, err)
		assert.Equal(t, "value", config["section.subsection.key"])
	})

	t.Run("overwrite existing config", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := CreateTestGitRepository(t, tmpDir, false)

		err := repo.SetGitConfig("test.overwrite", "original")
		require.NoError(t, err)

		err = repo.SetGitConfig("test.overwrite", "updated")
		require.NoError(t, err)

		config, err := repo.GetGitConfig()
		assert.Nil(t, err)
		assert.Equal(t, "updated", config["test.overwrite"])
	})

	t.Run("set config with numeric value", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := CreateTestGitRepository(t, tmpDir, false)

		err := repo.SetGitConfig("test.number", "12345")
		assert.Nil(t, err)

		config, err := repo.GetGitConfig()
		assert.Nil(t, err)
		assert.Equal(t, "12345", config["test.number"])
	})

	t.Run("set config with boolean value", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := CreateTestGitRepository(t, tmpDir, false)

		err := repo.SetGitConfig("test.boolean", "true")
		assert.Nil(t, err)

		config, err := repo.GetGitConfig()
		assert.Nil(t, err)
		assert.Equal(t, "true", config["test.boolean"])
	})

	t.Run("uppercase key becomes lowercase", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := CreateTestGitRepository(t, tmpDir, false)

		err := repo.SetGitConfig("TEST.UPPERCASE", "value")
		require.Nil(t, err)

		config, err := repo.GetGitConfig()
		assert.Nil(t, err)
		assert.Contains(t, config, "test.uppercase")
	})

	t.Run("error with empty key", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := CreateTestGitRepository(t, tmpDir, false)

		err := repo.SetGitConfig("", "value")
		assert.NotNil(t, err)
	})
}
