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
	"github.com/gittuf/gittuf/internal/signerverifier/sigstore"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/pkg/githash"
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
func createGPGSignedCommit(t *testing.T, repo *gitinterface.Repository) githash.Hash {
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

func TestVerifySigstore(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	commitID := createSigstoreSignedCommit(t, repo)

	payload, signature, err := repo.GetObjectSignature(commitID)
	require.Nil(t, err)

	sigstoreKey := &signerverifier.SSLibKey{
		KeyID:   "aditya@saky.in::https://github.com/login/oauth",
		KeyType: sigstore.KeyType,
		Scheme:  sigstore.KeyScheme,
		KeyVal: signerverifier.KeyVal{
			Identity: "aditya@saky.in",
			Issuer:   "https://github.com/login/oauth",
		},
	}

	// A payload the signature does not cover makes gitsign's CMS verification
	// fail locally, before any Rekor access. This exercises the Sigstore
	// branch of Verify and the setup portion of verifyGitsignSignature without
	// depending on the network.
	t.Run("sigstore signature verification fails", func(t *testing.T) {
		t.Parallel()
		err := Verify(context.Background(), sigstoreKey, []byte("mismatched payload"), signature)
		assert.ErrorIs(t, err, ErrIncorrectVerificationKey)
	})

	t.Run("doubled sigstore signature rejected", func(t *testing.T) {
		t.Parallel()
		block := strings.TrimRight(string(signature), "\n") + "\n"
		doubled := []byte(block + block)
		err := Verify(context.Background(), sigstoreKey, payload, doubled)
		assert.ErrorIs(t, err, ErrMultipleSignatures)
		assert.ErrorIs(t, err, ErrIncorrectVerificationKey)
	})
}

func TestSignatureBlockCount(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		signature string
		expected  int
	}{
		"no blocks": {
			signature: "not a signature",
			expected:  0,
		},
		"single pgp signature": {
			signature: "-----BEGIN PGP SIGNATURE-----\nabc\n-----END PGP SIGNATURE-----",
			expected:  1,
		},
		"single ssh signature": {
			signature: "-----BEGIN SSH SIGNATURE-----\nabc\n-----END SSH SIGNATURE-----",
			expected:  1,
		},
		"single gitsign signature": {
			signature: "-----BEGIN SIGNED MESSAGE-----\nabc\n-----END SIGNED MESSAGE-----",
			expected:  1,
		},
		"doubled gitsign signature": {
			signature: "-----BEGIN SIGNED MESSAGE-----\nabc\n-----END SIGNED MESSAGE-----\n-----BEGIN SIGNED MESSAGE-----\nabc\n-----END SIGNED MESSAGE-----",
			expected:  2,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.expected, signatureBlockCount(test.signature))
		})
	}
}

// createSigstoreSignedCommit builds a commit object carrying a gitsign
// (Sigstore) signature directly via go-git's storer and returns its hash. It
// mirrors createTestSigstoreSignedCommit in pkg/gitinterface/commit_test.go.
func createSigstoreSignedCommit(t *testing.T, repo *gitinterface.Repository) githash.Hash {
	t.Helper()

	goGitRepo, err := repo.GetGoGitRepository()
	require.Nil(t, err)

	testCommit := &object.Commit{
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
		Signature: `-----BEGIN SIGNED MESSAGE-----
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

	// Encode with the signature retained so GetObjectSignature returns the
	// gitsign block. EncodeWithoutSignature would drop it, leaving nothing for
	// the Sigstore path to verify.
	commitEncoded := goGitRepo.Storer.NewEncodedObject()
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
