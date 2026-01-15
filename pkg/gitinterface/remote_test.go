// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
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
}
