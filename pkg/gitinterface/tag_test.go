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
	"github.com/stretchr/testify/assert"
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

	t.Run("error with non-tag object", func(t *testing.T) {
		blobID, err := repo.WriteBlob([]byte("test"))
		if err != nil {
			t.Fatal(err)
		}

		_, err = repo.GetTagTarget(blobID)
		assert.NotNil(t, err)
	})

	t.Run("tag pointing to commit", func(t *testing.T) {
		tagID, err := repo.TagUsingSpecificKey(commitID, "tag-to-commit", "Tag to commit\n", artifacts.SSHED25519Private)
		if err != nil {
			t.Fatal(err)
		}

		targetID, err := repo.GetTagTarget(tagID)
		assert.Nil(t, err)
		assert.Equal(t, commitID, targetID)
	})

	t.Run("invalid tag ID", func(t *testing.T) {
		_, err := repo.GetTagTarget(ZeroHash)
		assert.NotNil(t, err)
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

	t.Run("verify with wrong key", func(t *testing.T) {
		tagID, err := repo.TagUsingSpecificKey(commitID, "test-tag-wrong", "test-tag\n", artifacts.SSHED25519Private)
		if err != nil {
			t.Fatal(err)
		}

		keyDir := t.TempDir()
		keyPath := filepath.Join(keyDir, "ssh-key.pub")
		if err := os.WriteFile(keyPath, artifacts.SSHRSAPublicSSH, 0o600); err != nil {
			t.Fatal(err)
		}
		wrongKey, err := ssh.NewKeyFromFile(keyPath)
		if err != nil {
			t.Fatal(err)
		}

		err = repo.verifyTagSignature(context.Background(), tagID, wrongKey)
		assert.NotNil(t, err)
	})

	t.Run("create tag with message without newline", func(t *testing.T) {
		tagID, err := repo.TagUsingSpecificKey(commitID, "test-tag-no-newline", "test message without newline", artifacts.SSHED25519Private)
		assert.Nil(t, err)
		assert.False(t, tagID.IsZero())
	})

	t.Run("create tag with message with newline", func(t *testing.T) {
		tagID, err := repo.TagUsingSpecificKey(commitID, "test-tag-with-newline", "test message with newline\n", artifacts.SSHED25519Private)
		assert.Nil(t, err)
		assert.False(t, tagID.IsZero())
	})

	t.Run("create tag with GPG key", func(t *testing.T) {
		tagID, err := repo.TagUsingSpecificKey(commitID, "test-tag-gpg-key", "GPG signed tag\n", artifacts.GPGKey1Private)
		assert.Nil(t, err)
		assert.False(t, tagID.IsZero())
	})

	t.Run("create tag with RSA SSH key", func(t *testing.T) {
		tagID, err := repo.TagUsingSpecificKey(commitID, "test-tag-rsa", "RSA SSH signed tag\n", artifacts.SSHRSAPrivate)
		assert.Nil(t, err)
		assert.False(t, tagID.IsZero())
	})

	t.Run("create tag pointing to tree", func(t *testing.T) {
		tagID, err := repo.TagUsingSpecificKey(emptyTreeID, "test-tag-tree", "Tag pointing to tree\n", artifacts.SSHED25519Private)
		assert.Nil(t, err)
		assert.False(t, tagID.IsZero())
	})

	t.Run("create tag pointing to blob", func(t *testing.T) {
		blobID, err := repo.WriteBlob([]byte("test blob content"))
		if err != nil {
			t.Fatal(err)
		}
		tagID, err := repo.TagUsingSpecificKey(blobID, "test-tag-blob", "Tag pointing to blob\n", artifacts.SSHED25519Private)
		assert.Nil(t, err)
		assert.False(t, tagID.IsZero())
	})

	t.Run("verify SSH ED25519 signed tag", func(t *testing.T) {
		tagID, err := repo.TagUsingSpecificKey(commitID, "test-ssh-ed25519", "SSH ED25519 tag\n", artifacts.SSHED25519Private)
		if err != nil {
			t.Fatal(err)
		}

		keyDir := t.TempDir()
		keyPath := filepath.Join(keyDir, "key.pub")
		if err := os.WriteFile(keyPath, artifacts.SSHED25519PublicSSH, 0o600); err != nil {
			t.Fatal(err)
		}
		key, err := ssh.NewKeyFromFile(keyPath)
		if err != nil {
			t.Fatal(err)
		}

		err = repo.verifyTagSignature(context.Background(), tagID, key)
		assert.Nil(t, err)
	})

	t.Run("verify SSH RSA signed tag", func(t *testing.T) {
		tagID, err := repo.TagUsingSpecificKey(commitID, "test-ssh-rsa", "SSH RSA tag\n", artifacts.SSHRSAPrivate)
		if err != nil {
			t.Fatal(err)
		}

		keyDir := t.TempDir()
		keyPath := filepath.Join(keyDir, "key.pub")
		if err := os.WriteFile(keyPath, artifacts.SSHRSAPublicSSH, 0o600); err != nil {
			t.Fatal(err)
		}
		key, err := ssh.NewKeyFromFile(keyPath)
		if err != nil {
			t.Fatal(err)
		}

		err = repo.verifyTagSignature(context.Background(), tagID, key)
		assert.Nil(t, err)
	})

	t.Run("verify GPG signed tag with correct key", func(t *testing.T) {
		tagID, err := repo.TagUsingSpecificKey(commitID, "test-gpg-correct", "GPG tag\n", artifacts.GPGKey1Private)
		if err != nil {
			t.Fatal(err)
		}

		key, err := gpg.LoadGPGKeyFromBytes(artifacts.GPGKey1Public)
		if err != nil {
			t.Fatal(err)
		}

		err = repo.verifyTagSignature(context.Background(), tagID, key)
		assert.Nil(t, err)
	})

	t.Run("verify GPG signed tag with wrong key", func(t *testing.T) {
		tagID, err := repo.TagUsingSpecificKey(commitID, "test-gpg-wrong", "GPG tag wrong\n", artifacts.GPGKey1Private)
		if err != nil {
			t.Fatal(err)
		}

		key, err := gpg.LoadGPGKeyFromBytes(artifacts.GPGKey2Public)
		if err != nil {
			t.Fatal(err)
		}

		err = repo.verifyTagSignature(context.Background(), tagID, key)
		assert.NotNil(t, err)
	})
}

func TestEnsureIsTag(t *testing.T) {
	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir, false)

	treeBuilder := NewTreeBuilder(repo)
	emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
	if err != nil {
		t.Fatal(err)
	}

	commitID, err := repo.Commit(emptyTreeID, "refs/heads/main", "Initial commit\n", true)
	if err != nil {
		t.Fatal(err)
	}

	tagID, err := repo.TagUsingSpecificKey(commitID, "test-tag-ensure", "test tag\n", artifacts.SSHED25519Private)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("valid tag object", func(t *testing.T) {
		err := repo.ensureIsTag(tagID)
		assert.Nil(t, err)
	})

	t.Run("commit object is not a tag", func(t *testing.T) {
		err := repo.ensureIsTag(commitID)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "is not a tag object")
	})

	t.Run("blob object is not a tag", func(t *testing.T) {
		blobID, err := repo.WriteBlob([]byte("test"))
		if err != nil {
			t.Fatal(err)
		}
		err = repo.ensureIsTag(blobID)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "is not a tag object")
	})

	t.Run("tree object is not a tag", func(t *testing.T) {
		err := repo.ensureIsTag(emptyTreeID)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "is not a tag object")
	})

	t.Run("non-existent object", func(t *testing.T) {
		err := repo.ensureIsTag(ZeroHash)
		assert.NotNil(t, err)
	})
}
