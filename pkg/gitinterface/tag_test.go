// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
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

// Git appends tag signatures to the tag payload, and go-git only ever
// surfaces the trailing block as the tag's signature; the earlier blocks stay
// in the message, so the signed payload no longer matches and verification
// fails. The explicit multi-block check in verifyTagSignature is
// defense-in-depth and is not reachable through go-git's tag decoding.
func TestVerifyTagSignatureRejectsMultipleSignatures(t *testing.T) {
	tests := map[string]struct {
		signingKey      []byte
		verificationKey func(t *testing.T) *signerverifier.SSLibKey
	}{
		"gpg": {
			signingKey: artifacts.GPGKey1Private,
			verificationKey: func(t *testing.T) *signerverifier.SSLibKey {
				t.Helper()
				key, err := gpg.LoadGPGKeyFromBytes(artifacts.GPGKey1Public)
				require.Nil(t, err)
				return key
			},
		},
		"ssh": {
			signingKey: artifacts.SSHED25519Private,
			verificationKey: func(t *testing.T) *signerverifier.SSLibKey {
				t.Helper()
				keyPath := filepath.Join(t.TempDir(), "ssh-key.pub")
				require.Nil(t, os.WriteFile(keyPath, artifacts.SSHED25519PublicSSH, 0o600))
				key, err := ssh.NewKeyFromFile(keyPath)
				require.Nil(t, err)
				return key
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			tempDir := t.TempDir()
			repo := CreateTestGitRepository(t, tempDir, false)

			emptyTreeID, err := NewTreeBuilder(repo).WriteTreeFromEntries(nil)
			require.Nil(t, err)

			commitID, err := repo.Commit(emptyTreeID, "refs/heads/main", "Initial commit\n", false)
			require.Nil(t, err)

			goGitRepo, err := repo.GetGoGitRepository()
			require.Nil(t, err)

			gitConfig, err := repo.GetGitConfig()
			require.Nil(t, err)

			tag := &object.Tag{
				Name: "test-tag",
				Tagger: object.Signature{
					Name:  gitConfig["user.name"],
					Email: gitConfig["user.email"],
					When:  repo.clock.Now(),
				},
				Message:    "test-tag\n",
				TargetType: plumbing.CommitObject,
				Target:     plumbing.NewHash(commitID.String()),
			}

			tagContents, err := getTagBytesWithoutSignature(tag)
			require.Nil(t, err)

			sig, err := signGitObjectUsingKey(tagContents, test.signingKey)
			require.Nil(t, err)

			// Two (individually valid) signature blocks, each on their own
			// lines, must not verify against either block.
			block := strings.TrimRight(sig, "\n") + "\n"
			tag.Signature = block + block

			tagEncoded := goGitRepo.Storer.NewEncodedObject()
			require.Nil(t, tag.Encode(tagEncoded))
			tagID, err := goGitRepo.Storer.SetEncodedObject(tagEncoded)
			require.Nil(t, err)
			tagHash, err := NewHash(tagID.String())
			require.Nil(t, err)

			err = repo.verifyTagSignature(context.Background(), tagHash, test.verificationKey(t))
			assert.ErrorIs(t, err, ErrIncorrectVerificationKey)
		})
	}
}

func TestRepositoryVerifyTagSHA256(t *testing.T) {
	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir, false, WithSHA256Format())

	treeBuilder := NewTreeBuilder(repo)

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

func TestTagUsingSpecificKeySignatureHeader(t *testing.T) {
	for _, objectFormat := range []ObjectFormat{ObjectFormatSHA1, ObjectFormatSHA256} {
		t.Run(string(objectFormat), func(t *testing.T) {
			tmpDir := t.TempDir()
			repo := CreateTestGitRepository(t, tmpDir, false, WithObjectFormat(objectFormat))

			emptyTreeID, err := NewTreeBuilder(repo).WriteTreeFromEntries(nil)
			require.Nil(t, err)
			commitID, err := repo.Commit(emptyTreeID, "refs/heads/main", "Initial commit\n", false)
			require.Nil(t, err)

			tagID, err := repo.TagUsingSpecificKey(commitID, "v1", "v1\n", artifacts.SSHED25519Private)
			require.Nil(t, err)

			raw, err := repo.executor("cat-file", "tag", tagID.String()).executeString()
			require.Nil(t, err)

			// Git appends tag signatures to the tag payload regardless of
			// the object format; the `gpgsig-sha256` header is only used for
			// compat signatures in dual-hash interop repositories.
			assert.NotContains(t, raw, "gpgsig-sha256")
			assert.Contains(t, raw, "v1\n-----BEGIN SSH SIGNATURE-----")

			keyDir := t.TempDir()
			keyPath := filepath.Join(keyDir, "ssh-key.pub")
			require.Nil(t, os.WriteFile(keyPath, artifacts.SSHED25519PublicSSH, 0o600))
			sshKey, err := ssh.NewKeyFromFile(keyPath)
			require.Nil(t, err)

			assert.Nil(t, repo.verifyTagSignature(context.Background(), tagID, sshKey))
		})
	}
}

func TestVerifyTagSignatureGitCreatedTag(t *testing.T) {
	for _, objectFormat := range []ObjectFormat{ObjectFormatSHA1, ObjectFormatSHA256} {
		t.Run(string(objectFormat), func(t *testing.T) {
			tmpDir := t.TempDir()
			repo := CreateTestGitRepository(t, tmpDir, false, WithObjectFormat(objectFormat))

			emptyTreeID, err := NewTreeBuilder(repo).WriteTreeFromEntries(nil)
			require.Nil(t, err)
			commitID, err := repo.Commit(emptyTreeID, "refs/heads/main", "Initial commit\n", false)
			require.Nil(t, err)

			// Sign the tag with Git itself, using the repository's configured
			// SSH signing key, to ensure verification handles tags exactly as
			// Git creates them.
			_, err = repo.executor("tag", "-s", "-m", "v1", "v1", commitID.String()).executeString()
			require.Nil(t, err)

			tagID, err := repo.GetReference("refs/tags/v1")
			require.Nil(t, err)

			keyDir := t.TempDir()
			keyPath := filepath.Join(keyDir, "ssh-key.pub")
			require.Nil(t, os.WriteFile(keyPath, artifacts.SSHRSAPublicSSH, 0o600))
			sshKey, err := ssh.NewKeyFromFile(keyPath)
			require.Nil(t, err)

			assert.Nil(t, repo.verifyTagSignature(context.Background(), tagID, sshKey))
		})
	}
}

// A tag message may itself contain an armored signature block (e.g. quoted
// release notes). Git treats only the trailing block as the tag's signature
// (parse_signed_buffer in gpg-interface.c returns the last block start) and
// everything before it as signed payload, so `git verify-tag` accepts such
// tags. Verification must match and not reject them as multiple signatures.
func TestVerifyTagSignatureArmorBlockInMessage(t *testing.T) {
	tmpDir := t.TempDir()
	repo := CreateTestGitRepository(t, tmpDir, false)

	emptyTreeID, err := NewTreeBuilder(repo).WriteTreeFromEntries(nil)
	require.Nil(t, err)
	commitID, err := repo.Commit(emptyTreeID, "refs/heads/main", "Initial commit\n", false)
	require.Nil(t, err)

	messageFile := filepath.Join(t.TempDir(), "message")
	message := "release notes quoting a signature:\n-----BEGIN SSH SIGNATURE-----\nU1NIU0lHZHVtbXk=\n-----END SSH SIGNATURE-----\nend of notes\n"
	require.Nil(t, os.WriteFile(messageFile, []byte(message), 0o600))

	_, err = repo.executor("tag", "-s", "-F", messageFile, "v1", commitID.String()).executeString()
	require.Nil(t, err)

	tagID, err := repo.GetReference("refs/tags/v1")
	require.Nil(t, err)

	keyDir := t.TempDir()
	keyPath := filepath.Join(keyDir, "ssh-key.pub")
	require.Nil(t, os.WriteFile(keyPath, artifacts.SSHRSAPublicSSH, 0o600))
	sshKey, err := ssh.NewKeyFromFile(keyPath)
	require.Nil(t, err)

	assert.Nil(t, repo.verifyTagSignature(context.Background(), tagID, sshKey))
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
