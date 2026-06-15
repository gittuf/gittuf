// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/secure-systems-lab/go-securesystemslib/signerverifier"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTagTarget(t *testing.T) {
	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir, false)

	treeBuilder := NewTreeBuilder(repo)

	// Write empty tree
	emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
	if err != nil {
		t.Fatal(err)
	}

	commitID, err := repo.Commit(emptyTreeID, "refs/heads/main", "Initial commit\n", true)
	if err != nil {
		t.Fatal(err)
	}

	tagID, err := repo.TagUsingSpecificKey(commitID, "test-tag", "test-tag\n", artifacts.SSHED25519Private)
	if err != nil {
		t.Fatal(err)
	}

	targetID, err := repo.GetTagTarget(tagID)
	assert.Nil(t, err)
	assert.Equal(t, commitID, targetID)

	t.Run("non-existent tag", func(t *testing.T) {
		_, err := repo.GetTagTarget(ZeroHash)
		assert.ErrorContains(t, err, "unable to resolve tag's target ID")
	})
}

func TestRepositoryVerifyTag(t *testing.T) {
	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir, false)

	treeBuilder := NewTreeBuilder(repo)

	// Write empty tree
	emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
	if err != nil {
		t.Fatal(err)
	}

	commitID, err := repo.Commit(emptyTreeID, "refs/heads/main", "Initial commit\n", true)
	if err != nil {
		t.Fatal(err)
	}

	sshSignedTag, err := repo.TagUsingSpecificKey(commitID, "test-tag-ssh", "test-tag-ssh\n", artifacts.SSHED25519Private)
	if err != nil {
		t.Fatal(err)
	}

	keyDir := t.TempDir()
	keyPath := filepath.Join(keyDir, "ssh-key.pub")
	if err := os.WriteFile(keyPath, artifacts.SSHED25519PublicSSH, 0o600); err != nil {
		t.Fatal(err)
	}
	sshKey, err := ssh.NewKeyFromFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}

	gpgSignedTag, err := repo.TagUsingSpecificKey(commitID, "test-tag-gpg", "test-tag-gpg\n", artifacts.GPGKey1Private)
	if err != nil {
		t.Fatal(err)
	}
	gpgKey, err := gpg.LoadGPGKeyFromBytes(artifacts.GPGKey1Public)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("ssh signed tag, verify with ssh key", func(t *testing.T) {
		err = repo.verifyTagSignature(context.Background(), sshSignedTag, sshKey)
		assert.Nil(t, err)
	})

	t.Run("gpg signed tag, verify with gpg key", func(t *testing.T) {
		err = repo.verifyTagSignature(context.Background(), gpgSignedTag, gpgKey)
		assert.Nil(t, err)
	})

	t.Run("unknown signing method", func(t *testing.T) {
		unknownKey := &signerverifier.SSLibKey{KeyType: "unknown"}
		err = repo.verifyTagSignature(context.Background(), gpgSignedTag, unknownKey)
		assert.ErrorIs(t, err, ErrUnknownSigningMethod)
	})
}

func TestEnsureIsTag(t *testing.T) {
	tmpDir := t.TempDir()
	repo := CreateTestGitRepository(t, tmpDir, false)
	treeBuilder := NewTreeBuilder(repo)

	emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
	require.Nil(t, err)

	commitID, err := repo.Commit(emptyTreeID, "refs/heads/main", "Initial commit\n", false)
	require.Nil(t, err)

	tagID, err := repo.TagUsingSpecificKey(commitID, "test-tag", "test-tag\n", artifacts.SSHED25519Private)
	require.Nil(t, err)

	err = repo.ensureIsTag(tagID)
	assert.Nil(t, err)

	err = repo.ensureIsTag(commitID)
	assert.ErrorContains(t, err, "is not a tag object")

	err = repo.ensureIsTag(ZeroHash)
	assert.ErrorContains(t, err, "unable to inspect if object is tag")
}

func TestTagUsingSpecificKey(t *testing.T) {
	t.Run("invalid target", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := CreateTestGitRepository(t, tmpDir, false)

		_, err := repo.TagUsingSpecificKey(ZeroHash, "test-tag", "test-tag\n", artifacts.SSHED25519Private)
		assert.ErrorContains(t, err, "not found")
	})
}
