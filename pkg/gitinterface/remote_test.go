// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemote(t *testing.T) {
	tmpDir := t.TempDir()
	repo := CreateTestGitRepository(t, tmpDir, false)

	output, err := repo.executor("remote").executeString()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "", output) // no output because there are no remotes

	remoteName := "origin"
	remoteURL := "git@example.com:repo.git"

	// Test AddRemote
	err = repo.AddRemote(remoteName, remoteURL)
	assert.Nil(t, err)

	output, err = repo.executor("remote", "-v").executeString()
	if err != nil {
		t.Fatal(err)
	}

	expectedOutput := fmt.Sprintf("%s\t%s (fetch)\n%s\t%s (push)", remoteName, remoteURL, remoteName, remoteURL)
	assert.Equal(t, expectedOutput, output)

	// Test GetRemoteURL
	returnedRemoteURL, err := repo.GetRemoteURL(remoteName)
	assert.Nil(t, err)
	assert.Equal(t, remoteURL, returnedRemoteURL)

	_, err = repo.GetRemoteURL("does-not-exist")
	assert.ErrorContains(t, err, "No such remote")

	// Test RemoveRemote
	err = repo.RemoveRemote(remoteName)
	assert.Nil(t, err)

	output, err = repo.executor("remote").executeString()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "", output) // no output because there are no remotes

	t.Run("add remote error - duplicate", func(t *testing.T) {
		err := repo.AddRemote("dup", "git@example.com:repo.git")
		assert.Nil(t, err)

		err = repo.AddRemote("dup", "git@example.com:repo.git")
		assert.NotNil(t, err)
	})

	t.Run("remove remote error - non-existent", func(t *testing.T) {
		err := repo.RemoveRemote("nonexistent")
		assert.NotNil(t, err)
	})

	t.Run("multiple remotes", func(t *testing.T) {
		err := repo.AddRemote("r1", "git@github.com:user/repo1.git")
		assert.Nil(t, err)

		err = repo.AddRemote("r2", "git@github.com:org/repo2.git")
		assert.Nil(t, err)

		url1, err := repo.GetRemoteURL("r1")
		assert.Nil(t, err)
		assert.Equal(t, "git@github.com:user/repo1.git", url1)

		url2, err := repo.GetRemoteURL("r2")
		assert.Nil(t, err)
		assert.Equal(t, "git@github.com:org/repo2.git", url2)
	})

	t.Run("https protocol", func(t *testing.T) {
		err := repo.AddRemote("https-r", "https://github.com/user/repo.git")
		assert.Nil(t, err)

		url, err := repo.GetRemoteURL("https-r")
		assert.Nil(t, err)
		assert.Equal(t, "https://github.com/user/repo.git", url)
	})

	t.Run("remove one of multiple remotes", func(t *testing.T) {
		err := repo.AddRemote("keep", "git@example.com:repo1.git")
		assert.Nil(t, err)

		err = repo.AddRemote("remove", "git@example.com:repo2.git")
		assert.Nil(t, err)

		err = repo.RemoveRemote("remove")
		assert.Nil(t, err)

		_, err = repo.GetRemoteURL("remove")
		assert.NotNil(t, err)

		url, err := repo.GetRemoteURL("keep")
		assert.Nil(t, err)
		assert.Equal(t, "git@example.com:repo1.git", url)
	})

	t.Run("update remote URL", func(t *testing.T) {
		err := repo.AddRemote("update", "https://old-url.com/repo.git")
		require.Nil(t, err)

		err = repo.RemoveRemote("update")
		require.Nil(t, err)

		err = repo.AddRemote("update", "https://new-url.com/repo.git")
		require.Nil(t, err)

		url, err := repo.GetRemoteURL("update")
		assert.Nil(t, err)
		assert.Equal(t, "https://new-url.com/repo.git", url)
	})

	t.Run("remote with special characters in URL", func(t *testing.T) {
		err := repo.CreateRemote("special", "https://user:pass@example.com:8080/repo.git")
		assert.Nil(t, err)

		url, err := repo.GetRemoteURL("special")
		assert.Nil(t, err)
		assert.Contains(t, url, "example.com")
	})
}
