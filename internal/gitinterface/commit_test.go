// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/gittuf/gittuf/internal/signerverifier"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	sslibsv "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/signerverifier"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/hiddeco/sshsig"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/ssh"
)

var (
	rsaSSHPublicKeyBytes    = artifacts.SSHRSAPublic
	rsaSSHPrivateKeyBytes   = artifacts.SSHRSAPrivate
	ecdsaSSHPublicKeyBytes  = artifacts.SSHECDSAPublic
	ecdsaSSHPrivateKeyBytes = artifacts.SSHECDSAPrivate
	gpgPublicKey            = artifacts.GPGKey1Public
	gpgPrivateKey           = artifacts.GPGKey1Private
)

func TestRepositoryCommit(t *testing.T) {
	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir)

	refName := "refs/heads/main"
	treeBuilder := NewReplacementTreeBuilder(repo)

	// Write empty tree
	emptyTreeID, err := treeBuilder.WriteRootTreeFromBlobIDs(nil)
	if err != nil {
		t.Fatal(err)
	}

	// Write second tree
	blobID, err := repo.WriteBlob([]byte("Hello, world!\n"))
	if err != nil {
		t.Fatal(err)
	}
	treeWithContentsID, err := treeBuilder.WriteRootTreeFromBlobIDs(map[string]Hash{"README.md": blobID})
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
	expectedThirdCommitID := "821c6322a3637799591e355f92c3334134edc793"
	commitID, err = repo.Commit(treeWithContentsID, refName, "Signing this commit\n", true)
	assert.Nil(t, err)
	assert.Equal(t, expectedThirdCommitID, commitID.String())

	refHead, err = repo.GetReference(refName)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, expectedThirdCommitID, refHead.String())
}

func TestCreateCommitObject(t *testing.T) {
	t.Run("zero commit and zero parent", func(t *testing.T) {
		commit := CreateCommitObject(testGitConfig, plumbing.ZeroHash, []plumbing.Hash{plumbing.ZeroHash}, "Test commit", testClock)

		enc := memory.NewStorage().NewEncodedObject()
		if err := commit.Encode(enc); err != nil {
			t.Error(err)
		}

		assert.Equal(t, "22ddfd55fb5fba7b37b50b068d1527a1b0f9f561", enc.Hash().String())
	})

	t.Run("zero commit and single non-zero parent", func(t *testing.T) {
		pHashes := []plumbing.Hash{EmptyTree()}
		commit := CreateCommitObject(testGitConfig, plumbing.ZeroHash, pHashes, "Test commit", testClock)

		enc := memory.NewStorage().NewEncodedObject()
		if err := commit.Encode(enc); err != nil {
			t.Error(err)
		}

		for parentHashInd := range commit.ParentHashes {
			assert.Equal(t, pHashes[parentHashInd], commit.ParentHashes[parentHashInd])
		}
	})

	t.Run("zero commit and multiple parents", func(t *testing.T) {
		pHashes := []plumbing.Hash{EmptyTree(), EmptyTree()}

		commit := CreateCommitObject(testGitConfig, plumbing.ZeroHash, pHashes, "Test commit", testClock)

		enc := memory.NewStorage().NewEncodedObject()
		if err := commit.Encode(enc); err != nil {
			t.Error(err)
		}

		for parentHashInd := range commit.ParentHashes {
			assert.Equal(t, pHashes[parentHashInd], commit.ParentHashes[parentHashInd])
		}
	})
}

func TestVerifyCommitSignature(t *testing.T) {
	gpgSignedCommit := createTestSignedCommit(t)

	// FIXME: fix gitsign testing
	gitsignSignedCommit := &object.Commit{
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

	sshCommits := createTestSSHSignedCommits(t)

	gpgKey, err := gpg.LoadGPGKeyFromBytes(gpgPublicKey)
	if err != nil {
		t.Fatal(err)
	}

	fulcioKey := &sslibsv.SSLibKey{
		KeyType: signerverifier.FulcioKeyType,
		Scheme:  "fulcio",
		KeyVal: sslibsv.KeyVal{
			Identity: "aditya@saky.in",
			Issuer:   "https://github.com/login/oauth",
		},
	}

	rsaKey, err := sslibsv.LoadKey(rsaSSHPublicKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	ecdsaKey, err := sslibsv.LoadKey(ecdsaSSHPublicKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("gpg signed commit", func(t *testing.T) {
		err = VerifyCommitSignature(context.Background(), gpgSignedCommit, gpgKey)
		assert.Nil(t, err)
	})

	// FIXME: fix gitsign testing
	// t.Run("gitsign signed commit", func(t *testing.T) {
	// 	err := VerifyCommitSignature(context.Background(), gitsignSignedCommit, fulcioKey)
	// 	assert.Nil(t, err)
	// })

	t.Run("use gpg signed commit with gitsign key", func(t *testing.T) {
		err := VerifyCommitSignature(context.Background(), gpgSignedCommit, fulcioKey)
		assert.ErrorIs(t, err, ErrIncorrectVerificationKey)
	})

	t.Run("use gitsign signed commit with gpg key", func(t *testing.T) {
		err := VerifyCommitSignature(context.Background(), gitsignSignedCommit, gpgKey)
		assert.ErrorIs(t, err, ErrIncorrectVerificationKey)
	})

	t.Run("use ssh signed commits with corresponding keys", func(t *testing.T) {
		err := VerifyCommitSignature(context.Background(), sshCommits[0], rsaKey)
		assert.Nil(t, err)

		err = VerifyCommitSignature(context.Background(), sshCommits[1], ecdsaKey)
		assert.Nil(t, err)
	})

	t.Run("use ssh signed commits with wrong keys", func(t *testing.T) {
		err := VerifyCommitSignature(context.Background(), sshCommits[0], ecdsaKey)
		assert.ErrorIs(t, err, ErrIncorrectVerificationKey)

		err = VerifyCommitSignature(context.Background(), sshCommits[1], rsaKey)
		assert.ErrorIs(t, err, ErrIncorrectVerificationKey)
	})
}

func TestRepositoryVerifyCommit(t *testing.T) {
	// TODO: support multiple signing types

	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir)

	treeBuilder := NewReplacementTreeBuilder(repo)

	// Write empty tree
	emptyTreeID, err := treeBuilder.WriteRootTreeFromBlobIDs(nil)
	if err != nil {
		t.Fatal(err)
	}

	commitID, err := repo.Commit(emptyTreeID, "refs/heads/main", "Initial commit\n", true)
	if err != nil {
		t.Fatal(err)
	}

	key, err := tuf.LoadKeyFromBytes(artifacts.SSHED25519Public)
	if err != nil {
		t.Fatal(err)
	}

	err = repo.verifyCommitSignature(context.Background(), commitID, key)
	assert.Nil(t, err)
}

func TestKnowsCommit(t *testing.T) {
	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	refName := "refs/heads/main"
	if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(refName), plumbing.ZeroHash)); err != nil {
		t.Fatal(err)
	}

	emptyTreeHash, err := WriteTree(repo, nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := Commit(repo, emptyTreeHash, refName, "First commit", false); err != nil {
		t.Fatal(err)
	}
	ref, err := repo.Reference(plumbing.ReferenceName(refName), true)
	if err != nil {
		t.Fatal(err)
	}
	firstCommitID := ref.Hash()
	firstCommit, err := GetCommit(repo, firstCommitID)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := Commit(repo, emptyTreeHash, refName, "Second commit", false); err != nil {
		t.Fatal(err)
	}
	ref, err = repo.Reference(plumbing.ReferenceName(refName), true)
	if err != nil {
		t.Fatal(err)
	}
	secondCommitID := ref.Hash()
	secondCommit, err := GetCommit(repo, secondCommitID)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("check if second commit knows first", func(t *testing.T) {
		knows, err := KnowsCommit(repo, secondCommitID, firstCommit)
		assert.Nil(t, err)
		assert.True(t, knows)
	})

	t.Run("check that first commit does not know second", func(t *testing.T) {
		knows, err := KnowsCommit(repo, firstCommitID, secondCommit)
		assert.Nil(t, err)
		assert.False(t, knows)
	})

	t.Run("check that both commits know themselves", func(t *testing.T) {
		knows, err := KnowsCommit(repo, firstCommitID, firstCommit)
		assert.Nil(t, err)
		assert.True(t, knows)

		knows, err = KnowsCommit(repo, secondCommitID, secondCommit)
		assert.Nil(t, err)
		assert.True(t, knows)
	})

	t.Run("check that an unknown commit can't know a known commit", func(t *testing.T) {
		knows, err := KnowsCommit(repo, plumbing.ZeroHash, firstCommit)
		assert.ErrorIs(t, err, plumbing.ErrObjectNotFound)
		assert.False(t, knows)
	})
}

func createTestSignedCommit(t *testing.T) *object.Commit {
	t.Helper()

	repo, err := git.Init(memory.NewStorage(), memfs.New())
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
		Message:  "Test commit",
		TreeHash: plumbing.ZeroHash,
	}

	commitEncoded := repo.Storer.NewEncodedObject()
	if err := testCommit.EncodeWithoutSignature(commitEncoded); err != nil {
		t.Fatal(err)
	}
	r, err := commitEncoded.Reader()
	if err != nil {
		t.Fatal(err)
	}

	keyring, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(gpgPrivateKey))
	if err != nil {
		t.Fatal(err)
	}

	sig := new(strings.Builder)
	if err := openpgp.ArmoredDetachSign(sig, keyring[0], r, nil); err != nil {
		t.Fatal(err)
	}
	testCommit.PGPSignature = sig.String()

	return testCommit
}

func createTestSSHSignedCommits(t *testing.T) []*object.Commit {
	t.Helper()

	testCommits := []*object.Commit{}

	signingKeys := [][]byte{rsaSSHPrivateKeyBytes, ecdsaSSHPrivateKeyBytes}

	for _, keyBytes := range signingKeys {
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
			Message:  "Test commit",
			TreeHash: EmptyTree(),
		}

		commitBytes, err := getCommitBytesWithoutSignature(testCommit)
		if err != nil {
			t.Fatal(err)
		}
		signer, err := ssh.ParsePrivateKey(keyBytes)
		if err != nil {
			t.Fatal(err)
		}

		sshSig, err := sshsig.Sign(bytes.NewReader(commitBytes), signer, sshsig.HashSHA512, namespaceSSHSignature)
		if err != nil {
			t.Fatal(err)
		}

		sigBytes := sshsig.Armor(sshSig)
		testCommit.PGPSignature = string(sigBytes)

		testCommits = append(testCommits, testCommit)
	}

	return testCommits
}

func TestRepositoryGetCommitMessage(t *testing.T) {
	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir)

	refName := "refs/heads/main"
	treeBuilder := NewReplacementTreeBuilder(repo)

	// Write empty tree
	emptyTreeID, err := treeBuilder.WriteRootTreeFromBlobIDs(nil)
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
	repo := CreateTestGitRepository(t, tempDir)

	refName := "refs/heads/main"
	treeBuilder := NewReplacementTreeBuilder(repo)

	// Write empty tree
	emptyTreeID, err := treeBuilder.WriteRootTreeFromBlobIDs(nil)
	if err != nil {
		t.Fatal(err)
	}

	// Write second tree
	blobID, err := repo.WriteBlob([]byte("Hello, world!\n"))
	if err != nil {
		t.Fatal(err)
	}
	treeWithContentsID, err := treeBuilder.WriteRootTreeFromBlobIDs(map[string]Hash{"README.md": blobID})
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
	repo := CreateTestGitRepository(t, tempDir)

	refName := "refs/heads/main"
	treeBuilder := NewReplacementTreeBuilder(repo)

	// Write empty tree
	emptyTreeID, err := treeBuilder.WriteRootTreeFromBlobIDs(nil)
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
