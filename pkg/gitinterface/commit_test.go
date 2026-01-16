// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

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
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
)

func TestRepositoryCommit(t *testing.T) {
	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir, false)

	refName := "refs/heads/main"
	treeBuilder := NewTreeBuilder(repo)

	// Write empty tree
	emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
	if err != nil {
		t.Fatal(err)
	}

	// Write second tree
	blobID, err := repo.WriteBlob([]byte("Hello, world!\n"))
	if err != nil {
		t.Fatal(err)
	}
	treeWithContentsID, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("README.md", blobID)})
	if err != nil {
		t.Fatal(err)
	}

	// Create initial commit with no tree
	expectedInitialCommitID := "648c569f3958b899e832f04750de52cf5d0db2fa"
	commitID, err := repo.Commit(emptyTreeID, refName, "Initial commit\n", false)
	assert.Nil(t, err)
	assert.Equal(t, expectedInitialCommitID, commitID.String())

	refHead, err := repo.GetReference(refName)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, expectedInitialCommitID, refHead.String())

	// Create second commit with tree
	expectedSecondCommitID := "3d7200c158ccfedf35a68a7d24842d60cac4ec0d"
	commitID, err = repo.Commit(treeWithContentsID, refName, "Add README\n", false)
	assert.Nil(t, err)
	assert.Equal(t, expectedSecondCommitID, commitID.String())

	refHead, err = repo.GetReference(refName)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, expectedSecondCommitID, refHead.String())

	// Create third commit with same tree but sign this time
	expectedThirdCommitID := "eed43c23f781ddc10359ce25e0fc486a000a8c9f"
	commitID, err = repo.Commit(treeWithContentsID, refName, "Signing this commit\n", true)
	assert.Nil(t, err)
	assert.Equal(t, expectedThirdCommitID, commitID.String())

	refHead, err = repo.GetReference(refName)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, expectedThirdCommitID, refHead.String())
}

func TestRepositoryCommitUsingSpecificKey(t *testing.T) {
	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir, false)

	refName := "refs/heads/main"
	treeBuilder := NewTreeBuilder(repo)

	// Write empty tree
	emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
	if err != nil {
		t.Fatal(err)
	}

	// Write second tree
	blobID, err := repo.WriteBlob([]byte("Hello, world!\n"))
	if err != nil {
		t.Fatal(err)
	}
	treeWithContentsID, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("README.md", blobID)})
	if err != nil {
		t.Fatal(err)
	}

	// Create initial commit with no tree
	expectedInitialCommitID := "b218890d607cdcea53ebf6c640748b4b1c8015ca"
	commitID, err := repo.CommitUsingSpecificKey(emptyTreeID, refName, "Initial commit\n", artifacts.SSHED25519Private)
	assert.Nil(t, err)
	assert.Equal(t, expectedInitialCommitID, commitID.String())

	refHead, err := repo.GetReference(refName)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, expectedInitialCommitID, refHead.String())

	// Create second commit with tree
	expectedSecondCommitID := "2b3f8b1f6af0d0d3c37130ba4d054ff4c2e95a3a"
	commitID, err = repo.CommitUsingSpecificKey(treeWithContentsID, refName, "Add README\n", artifacts.SSHED25519Private)
	assert.Nil(t, err)
	assert.Equal(t, expectedSecondCommitID, commitID.String())

	refHead, err = repo.GetReference(refName)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, expectedSecondCommitID, refHead.String())
}

func TestCommitUsingSpecificKey(t *testing.T) {
	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir, false)

	refName := "refs/heads/main"
	treeBuilder := NewTreeBuilder(repo)

	// Write empty tree
	emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
	if err != nil {
		t.Fatal(err)
	}

	// Write second tree
	blobID, err := repo.WriteBlob([]byte("Hello, world!\n"))
	if err != nil {
		t.Fatal(err)
	}
	treeWithContentsID, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("README.md", blobID)})
	if err != nil {
		t.Fatal(err)
	}

	// Create initial commit with no tree
	expectedInitialCommitID := "648c569f3958b899e832f04750de52cf5d0db2fa"
	commitID, err := repo.Commit(emptyTreeID, refName, "Initial commit\n", false)
	assert.Nil(t, err)
	assert.Equal(t, expectedInitialCommitID, commitID.String())

	refHead, err := repo.GetReference(refName)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, expectedInitialCommitID, refHead.String())

	privateKey := artifacts.SSHRSAPrivate

	// Create publicKey
	keyPath := filepath.Join(tempDir, "ssh-key")
	if err := os.WriteFile(keyPath, artifacts.SSHRSAPublicSSH, 0o600); err != nil {
		t.Fatal(err)
	}
	publicKey, err := ssh.NewKeyFromFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	// Create second commit with tree
	expectedSecondCommitID := "11020a7c78c4f903d0592ec2e8f73d00a17ec47e"
	commitID, err = repo.CommitUsingSpecificKey(treeWithContentsID, refName, "Add README\n", privateKey)
	assert.Nil(t, err)

	// Verify commit signature using publicKey
	err = repo.verifyCommitSignature(context.Background(), commitID, publicKey)
	assert.Nil(t, err)
	assert.Equal(t, expectedSecondCommitID, commitID.String())
}

func TestRepositoryVerifyCommit(t *testing.T) {
	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir, false)

	treeBuilder := NewTreeBuilder(repo)

	// Write empty tree
	emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
	if err != nil {
		t.Fatal(err)
	}

	sshSignedCommitID, err := repo.Commit(emptyTreeID, "refs/heads/main", "Initial commit\n", true)
	if err != nil {
		t.Fatal(err)
	}

	gpgSignedCommitID := createTestGPGSignedCommit(t, repo)

	// FIXME: fix gitsign testing
	gitsignSignedCommitID := createTestSigstoreSignedCommit(t, repo)

	keyDir := t.TempDir()
	keyPath := filepath.Join(keyDir, "ssh-key")
	if err := os.WriteFile(keyPath, artifacts.SSHRSAPublicSSH, 0o600); err != nil {
		t.Fatal(err)
	}
	sshKey, err := ssh.NewKeyFromFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}

	gpgKey, err := gpg.LoadGPGKeyFromBytes(artifacts.GPGKey1Public)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("ssh signed commit, verify with ssh key", func(t *testing.T) {
		err = repo.verifyCommitSignature(context.Background(), sshSignedCommitID, sshKey)
		assert.Nil(t, err)
	})

	t.Run("ssh signed commit, verify with gpg key", func(t *testing.T) {
		err = repo.verifyCommitSignature(context.Background(), sshSignedCommitID, gpgKey)
		assert.ErrorIs(t, err, ErrIncorrectVerificationKey)
	})

	t.Run("gpg signed commit, verify with gpg key", func(t *testing.T) {
		err = repo.verifyCommitSignature(context.Background(), gpgSignedCommitID, gpgKey)
		assert.Nil(t, err)
	})

	t.Run("gpg signed commit, verify with ssh key", func(t *testing.T) {
		err = repo.verifyCommitSignature(context.Background(), gpgSignedCommitID, sshKey)
		assert.ErrorIs(t, err, ErrIncorrectVerificationKey)
	})

	t.Run("gitsign signed commit, verify with ssh key", func(t *testing.T) {
		err = repo.verifyCommitSignature(context.Background(), gitsignSignedCommitID, sshKey)
		assert.ErrorIs(t, err, ErrIncorrectVerificationKey)
	})
}

func TestKnowsCommit(t *testing.T) {
	tmpDir := t.TempDir()
	repo := CreateTestGitRepository(t, tmpDir, false)

	refName := "refs/heads/main"

	treeBuilder := NewTreeBuilder(repo)

	// Write empty tree
	emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
	if err != nil {
		t.Fatal(err)
	}

	firstCommitID, err := repo.Commit(emptyTreeID, refName, "First commit", false)
	if err != nil {
		t.Fatal(err)
	}

	secondCommitID, err := repo.Commit(emptyTreeID, refName, "Second commit", false)
	if err != nil {
		t.Fatal(err)
	}

	unknownCommitID, err := repo.Commit(emptyTreeID, "refs/heads/unknown", "Unknown commit", false)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("check if second commit knows first", func(t *testing.T) {
		knows, err := repo.KnowsCommit(secondCommitID, firstCommitID)
		assert.Nil(t, err)
		assert.True(t, knows)
	})

	t.Run("check that first commit does not know second", func(t *testing.T) {
		knows, err := repo.KnowsCommit(firstCommitID, secondCommitID)
		assert.Nil(t, err)
		assert.False(t, knows)
	})

	t.Run("check that both commits know themselves", func(t *testing.T) {
		knows, err := repo.KnowsCommit(firstCommitID, firstCommitID)
		assert.Nil(t, err)
		assert.True(t, knows)

		knows, err = repo.KnowsCommit(secondCommitID, secondCommitID)
		assert.Nil(t, err)
		assert.True(t, knows)
	})

	t.Run("check that an unknown commit can't know a known commit", func(t *testing.T) {
		knows, _ := repo.KnowsCommit(unknownCommitID, firstCommitID)
		assert.False(t, knows)
	})
}

func createTestGPGSignedCommit(t *testing.T, repo *Repository) Hash {
	t.Helper()

	goGitRepo, err := repo.GetGoGitRepository()
	if err != nil {
		t.Fatal(err)
	}

	testCommit := &object.Commit{
		Author: object.Signature{
			Name:  testName,
			Email: testEmail,
			When:  testClock.Now(),
		},
		Committer: object.Signature{
			Name:  testName,
			Email: testEmail,
			When:  testClock.Now(),
		},
		Message:  "Test commit\n",
		TreeHash: plumbing.ZeroHash,
	}

	commitEncoded := goGitRepo.Storer.NewEncodedObject()
	if err := testCommit.EncodeWithoutSignature(commitEncoded); err != nil {
		t.Fatal(err)
	}
	r, err := commitEncoded.Reader()
	if err != nil {
		t.Fatal(err)
	}

	keyring, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(artifacts.GPGKey1Private))
	if err != nil {
		t.Fatal(err)
	}

	sig := new(strings.Builder)
	if err := openpgp.ArmoredDetachSign(sig, keyring[0], r, nil); err != nil {
		t.Fatal(err)
	}
	testCommit.PGPSignature = sig.String()

	// Re-encode with the signature
	commitEncoded = goGitRepo.Storer.NewEncodedObject()
	if err := testCommit.Encode(commitEncoded); err != nil {
		t.Fatal(err)
	}

	commitID, err := goGitRepo.Storer.SetEncodedObject(commitEncoded)
	if err != nil {
		t.Fatal(err)
	}

	commitHash, err := NewHash(commitID.String())
	if err != nil {
		t.Fatal(err)
	}

	return commitHash
}

func createTestSigstoreSignedCommit(t *testing.T, repo *Repository) Hash {
	t.Helper()

	goGitRepo, err := repo.GetGoGitRepository()
	if err != nil {
		t.Fatal(err)
	}
	testCommit := &object.Commit{
		Hash: plumbing.NewHash("d6b230478965e25477263aa65f1ca6d23d0c0d97"),
		Author: object.Signature{
			Name:  "Aditya Sirish",
			Email: "aditya@saky.in",
			When:  time.Date(2023, time.August, 1, 15, 44, 23, 0, time.FixedZone("", -4*3600)),
		},
		Committer: object.Signature{
			Name:  "Aditya Sirish",
			Email: "aditya@saky.in",
			When:  time.Date(2023, time.August, 1, 15, 44, 23, 0, time.FixedZone("", -4*3600)),
		},
		PGPSignature: `-----BEGIN SIGNED MESSAGE-----
MIIEMAYJKoZIhvcNAQcCoIIEITCCBB0CAQExDTALBglghkgBZQMEAgEwCwYJKoZI
hvcNAQcBoIIC0DCCAswwggJToAMCAQICFHIJCrBVHxoHlGos++k1xJxcElGaMAoG
CCqGSM49BAMDMDcxFTATBgNVBAoTDHNpZ3N0b3JlLmRldjEeMBwGA1UEAxMVc2ln
c3RvcmUtaW50ZXJtZWRpYXRlMB4XDTIzMDgwMTE5NDQzMVoXDTIzMDgwMTE5NTQz
MVowADBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABD8d752TJfGtANVYoiJJn+o6
JPKj5NwEZs1AcVRT2qElikVun5t+bQ07iDFa/Xiun5ytZrEK2YJVgqdntLd6hSOj
ggFyMIIBbjAOBgNVHQ8BAf8EBAMCB4AwEwYDVR0lBAwwCgYIKwYBBQUHAwMwHQYD
VR0OBBYEFAuYzgyBA01YSSN1v0fYenGo7+PcMB8GA1UdIwQYMBaAFN/T6c9WJBGW
+ajY6ShVosYuGGQ/MBwGA1UdEQEB/wQSMBCBDmFkaXR5YUBzYWt5LmluMCwGCisG
AQQBg78wAQEEHmh0dHBzOi8vZ2l0aHViLmNvbS9sb2dpbi9vYXV0aDAuBgorBgEE
AYO/MAEIBCAMHmh0dHBzOi8vZ2l0aHViLmNvbS9sb2dpbi9vYXV0aDCBigYKKwYB
BAHWeQIEAgR8BHoAeAB2AN09MGrGxxEyYxkeHJlnNwKiSl643jyt/4eKcoAvKe6O
AAABibKhcJgAAAQDAEcwRQIgcWuz6NhFgdL0fNni6j0SOQnAgFpPEaN8jDH70mbD
uPMCIQCX8koEnIX4c9crMT1hfoBBf1Z/CHJ6HLLHpQwWfEUMIzAKBggqhkjOPQQD
AwNnADBkAjBozIBaBtEu7JUyYLH7Ly698E0o8DdIOmqcUMUYWNC6zyJVdrL5gAla
mQSxfObSQasCMHQuw8youTjmFJXT7pNOYX4DW25knt+6P+W/m6zwcRRe3dMjmUAB
gdBJb32+XXJMRDGCASYwggEiAgEBME8wNzEVMBMGA1UEChMMc2lnc3RvcmUuZGV2
MR4wHAYDVQQDExVzaWdzdG9yZS1pbnRlcm1lZGlhdGUCFHIJCrBVHxoHlGos++k1
xJxcElGaMAsGCWCGSAFlAwQCAaBpMBgGCSqGSIb3DQEJAzELBgkqhkiG9w0BBwEw
HAYJKoZIhvcNAQkFMQ8XDTIzMDgwMTE5NDQzMlowLwYJKoZIhvcNAQkEMSIEIBe6
VHcVlkO8jRm/fbUipwxwxNaI7UFDAL38Jl8eUj/5MAoGCCqGSM49BAMCBEgwRgIh
AIYiRbnVeWjjgX2XwljDryzQN5RhUQaVH/AcUj+tbvWxAiEAhm9l3BU58tQsgyJW
oYBpMWLgg6AUzpxx9mITZ2EKr4c=
-----END SIGNED MESSAGE-----
`,
		Message:  "Test commit\n",
		TreeHash: plumbing.NewHash("4b825dc642cb6eb9a060e54bf8d69288fbee4904"),
	}

	commitEncoded := goGitRepo.Storer.NewEncodedObject()
	if err := testCommit.EncodeWithoutSignature(commitEncoded); err != nil {
		t.Fatal(err)
	}

	commitID, err := goGitRepo.Storer.SetEncodedObject(commitEncoded)
	if err != nil {
		t.Fatal(err)
	}

	commitHash, err := NewHash(commitID.String())
	if err != nil {
		t.Fatal(err)
	}

	return commitHash
}

func TestRepositoryGetCommitMessage(t *testing.T) {
	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir, false)

	refName := "refs/heads/main"
	treeBuilder := NewTreeBuilder(repo)

	// Write empty tree
	emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
	if err != nil {
		t.Fatal(err)
	}

	message := "Initial commit"
	commit, err := repo.Commit(emptyTreeID, refName, message, false)
	if err != nil {
		t.Fatal(err)
	}

	commitMessage, err := repo.GetCommitMessage(commit)
	assert.Nil(t, err)
	assert.Equal(t, message, commitMessage)
}

func TestGetCommitTreeID(t *testing.T) {
	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir, false)

	refName := "refs/heads/main"
	treeBuilder := NewTreeBuilder(repo)

	// Write empty tree
	emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
	if err != nil {
		t.Fatal(err)
	}

	// Write second tree
	blobID, err := repo.WriteBlob([]byte("Hello, world!\n"))
	if err != nil {
		t.Fatal(err)
	}
	treeWithContentsID, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("README.md", blobID)})
	if err != nil {
		t.Fatal(err)
	}

	// Create initial commit with no tree
	initialCommitID, err := repo.Commit(emptyTreeID, refName, "Initial commit\n", false)
	if err != nil {
		t.Fatal(err)
	}

	initialCommitTreeID, err := repo.GetCommitTreeID(initialCommitID)
	assert.Nil(t, err)
	assert.Equal(t, emptyTreeID, initialCommitTreeID)

	// Create second commit with tree
	secondCommitID, err := repo.Commit(treeWithContentsID, refName, "Add README\n", false)
	if err != nil {
		t.Fatal(err)
	}

	secondCommitTreeID, err := repo.GetCommitTreeID(secondCommitID)
	assert.Nil(t, err)
	assert.Equal(t, treeWithContentsID, secondCommitTreeID)
}

func TestGetCommitParentIDs(t *testing.T) {
	// TODO: test with merge commit

	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir, false)

	refName := "refs/heads/main"
	treeBuilder := NewTreeBuilder(repo)

	// Write empty tree
	emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create initial commit
	initialCommitID, err := repo.Commit(emptyTreeID, refName, "Initial commit\n", false)
	if err != nil {
		t.Fatal(err)
	}

	initialCommitParentIDs, err := repo.GetCommitParentIDs(initialCommitID)
	assert.Nil(t, err)
	assert.Empty(t, initialCommitParentIDs)

	// Create second commit
	secondCommitID, err := repo.Commit(emptyTreeID, refName, "Add README\n", false)
	if err != nil {
		t.Fatal(err)
	}

	secondCommitParentIDs, err := repo.GetCommitParentIDs(secondCommitID)
	assert.Nil(t, err)
	assert.Equal(t, []Hash{initialCommitID}, secondCommitParentIDs)
}

func TestGetCommonAncestor(t *testing.T) {
	tmpDir := t.TempDir()
	repo := CreateTestGitRepository(t, tmpDir, false)

	refName := "refs/heads/main"

	treeBuilder := NewTreeBuilder(repo)

	// Write empty tree
	emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
	if err != nil {
		t.Fatal(err)
	}

	initialCommitID, err := repo.Commit(emptyTreeID, refName, "Initial commit\n", false)
	if err != nil {
		t.Fatal(err)
	}

	// Add child commit A
	commitA, err := repo.Commit(emptyTreeID, refName, "Second commit A\n", false)
	if err != nil {
		t.Fatal(err)
	}

	// Add child commit B
	commitB := repo.commitWithParents(t, emptyTreeID, []Hash{initialCommitID}, "Second commit B\n", false)

	// Test commits, ensure we get back initial commit
	commonAncestor, err := repo.GetCommonAncestor(commitA, commitB)
	assert.Nil(t, err)
	assert.Equal(t, initialCommitID, commonAncestor)

	// Test with disjoint commit histories
	commitDisconnected := repo.commitWithParents(t, emptyTreeID, nil, "Disconnected initial commit\n", false)

	_, err = repo.GetCommonAncestor(commitDisconnected, commitA)
	assert.NotNil(t, err)
}
