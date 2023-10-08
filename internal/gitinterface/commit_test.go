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
	"github.com/gittuf/gittuf/internal/signerverifier"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	sslibsv "github.com/secure-systems-lab/go-securesystemslib/signerverifier"
	"github.com/stretchr/testify/assert"
)

func TestCreateCommitObject(t *testing.T) {
	commit := CreateCommitObject(testGitConfig, plumbing.ZeroHash, plumbing.ZeroHash, "Test commit", testClock)

	enc := memory.NewStorage().NewEncodedObject()
	if err := commit.Encode(enc); err != nil {
		t.Error(err)
	}

	assert.Equal(t, "22ddfd55fb5fba7b37b50b068d1527a1b0f9f561", enc.Hash().String())
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

	keyBytes, err := os.ReadFile(filepath.Join("test-data", "gpg-pubkey.asc"))
	if err != nil {
		t.Fatal(err)
	}

	gpgKey, err := gpg.LoadGPGKeyFromBytes(keyBytes)
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
	firstCommit, err := repo.CommitObject(firstCommitID)
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
	secondCommit, err := repo.CommitObject(secondCommitID)
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

	signingKeyBytes, err := os.ReadFile(filepath.Join("test-data", "gpg-privkey.asc"))
	if err != nil {
		t.Fatal(err)
	}

	keyring, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(signingKeyBytes))
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
