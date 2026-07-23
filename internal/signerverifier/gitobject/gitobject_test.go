// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitobject

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/secure-systems-lab/go-securesystemslib/signerverifier"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerify(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	treeBuilder := gitinterface.NewTreeBuilder(repo)
	emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
	require.Nil(t, err)

	commitID, err := repo.CommitUsingSpecificKey(emptyTreeHash, "refs/heads/main", "Signed commit\n", artifacts.SSHED25519Private)
	require.Nil(t, err)

	payload, signature, err := repo.GetObjectSignature(commitID)
	require.Nil(t, err)

	sshKey := loadSSHKey(t, tmpDir, "right-key.pub", artifacts.SSHED25519PublicSSH)
	wrongKey := loadSSHKey(t, tmpDir, "wrong-key.pub", artifacts.SSHRSAPublicSSH)

	t.Run("correct key", func(t *testing.T) {
		t.Parallel()
		err := Verify(context.Background(), sshKey, payload, signature)
		assert.Nil(t, err)
	})

	t.Run("wrong key", func(t *testing.T) {
		t.Parallel()
		err := Verify(context.Background(), wrongKey, payload, signature)
		assert.ErrorIs(t, err, ErrIncorrectVerificationKey)
	})

	t.Run("empty signature", func(t *testing.T) {
		t.Parallel()
		err := Verify(context.Background(), sshKey, payload, nil)
		assert.ErrorIs(t, err, ErrIncorrectVerificationKey)
	})

	t.Run("unknown key type", func(t *testing.T) {
		t.Parallel()
		unknownKey := &signerverifier.SSLibKey{KeyType: "unknown", Scheme: "unknown"}
		err := Verify(context.Background(), unknownKey, payload, signature)
		assert.ErrorIs(t, err, ErrUnknownSigningMethod)
	})

	t.Run("multiple signature blocks rejected", func(t *testing.T) {
		t.Parallel()
		doubled := append(append([]byte{}, signature...), signature...)
		err := Verify(context.Background(), sshKey, payload, doubled)
		assert.ErrorIs(t, err, ErrMultipleSignatures)
	})
}

func TestVerifyGPG(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	commitID := createGPGSignedCommit(t, repo)

	payload, signature, err := repo.GetObjectSignature(commitID)
	require.Nil(t, err)

	gpgKey, err := gpg.LoadGPGKeyFromBytes(artifacts.GPGKey1Public)
	require.Nil(t, err)

	wrongGPGKey, err := gpg.LoadGPGKeyFromBytes(artifacts.GPGKey2Public)
	require.Nil(t, err)

	sshKeyDir := t.TempDir()
	sshKey := loadSSHKey(t, sshKeyDir, "ssh.pub", artifacts.SSHED25519PublicSSH)

	t.Run("correct gpg key", func(t *testing.T) {
		t.Parallel()
		err := Verify(context.Background(), gpgKey, payload, signature)
		assert.Nil(t, err)
	})

	t.Run("wrong gpg key", func(t *testing.T) {
		t.Parallel()
		err := Verify(context.Background(), wrongGPGKey, payload, signature)
		assert.ErrorIs(t, err, ErrIncorrectVerificationKey)
	})

	t.Run("ssh key against gpg signature", func(t *testing.T) {
		t.Parallel()
		err := Verify(context.Background(), sshKey, payload, signature)
		assert.ErrorIs(t, err, ErrIncorrectVerificationKey)
	})
}

func TestVerifyWithRekorURL(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	treeBuilder := gitinterface.NewTreeBuilder(repo)
	emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
	require.Nil(t, err)

	commitID, err := repo.CommitUsingSpecificKey(emptyTreeHash, "refs/heads/main", "Signed commit\n", artifacts.SSHED25519Private)
	require.Nil(t, err)

	payload, signature, err := repo.GetObjectSignature(commitID)
	require.Nil(t, err)

	sshKeyDir := t.TempDir()
	sshKey := loadSSHKey(t, sshKeyDir, "ssh.pub", artifacts.SSHED25519PublicSSH)

	// Passing WithRekorURL exercises the option code path. The SSH verification
	// branch does not use the rekorURL, so this succeeds while covering the
	// option func on line 50.
	t.Run("ssh verify with custom rekor url option", func(t *testing.T) {
		t.Parallel()
		err := Verify(context.Background(), sshKey, payload, signature, WithRekorURL("https://rekor.example.test"))
		assert.Nil(t, err)
	})
}

// createGPGSignedCommit builds a GPG-signed commit object directly via go-git's
// storer and returns its hash. It mirrors the approach used in
// pkg/gitinterface/commit_test.go's createTestGPGSignedCommit helper.
func createGPGSignedCommit(t *testing.T, repo *gitinterface.Repository) gitinterface.Hash {
	t.Helper()

	goGitRepo, err := repo.GetGoGitRepository()
	require.Nil(t, err)

	testCommit := &object.Commit{
		Author: object.Signature{
			Name:  "Test Author",
			Email: "test@example.com",
			When:  time.Date(1995, time.October, 26, 9, 0, 0, 0, time.UTC),
		},
		Committer: object.Signature{
			Name:  "Test Author",
			Email: "test@example.com",
			When:  time.Date(1995, time.October, 26, 9, 0, 0, 0, time.UTC),
		},
		Message:  "Test GPG signed commit\n",
		TreeHash: plumbing.ZeroHash,
	}

	commitEncoded := goGitRepo.Storer.NewEncodedObject()
	require.Nil(t, testCommit.EncodeWithoutSignature(commitEncoded))

	r, err := commitEncoded.Reader()
	require.Nil(t, err)

	keyring, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(artifacts.GPGKey1Private))
	require.Nil(t, err)

	sig := new(strings.Builder)
	require.Nil(t, openpgp.ArmoredDetachSign(sig, keyring[0], r, nil))
	testCommit.Signature = sig.String()

	commitEncoded = goGitRepo.Storer.NewEncodedObject()
	require.Nil(t, testCommit.Encode(commitEncoded))

	commitID, err := goGitRepo.Storer.SetEncodedObject(commitEncoded)
	require.Nil(t, err)

	hash, err := gitinterface.NewHash(commitID.String())
	require.Nil(t, err)

	return hash
}

func loadSSHKey(t *testing.T, dir, name string, contents []byte) *signerverifier.SSLibKey {
	t.Helper()
	keyPath := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(keyPath, contents, 0o600))
	key, err := ssh.NewKeyFromFile(keyPath)
	require.Nil(t, err)
	return key
}
