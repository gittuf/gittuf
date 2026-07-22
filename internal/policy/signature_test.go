// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"errors"
	"fmt"
	"testing"

	"github.com/gittuf/gittuf/internal/common"
	svcommon "github.com/gittuf/gittuf/internal/signerverifier/common"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/gittuf/gittuf/internal/signerverifier/sigstore"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/gittuf/gittuf/pkg/gitstore"
	"github.com/secure-systems-lab/go-securesystemslib/signerverifier"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// overrideStorer wraps a real Storer, injecting failures or canned values for
// the methods the tests need to control while delegating everything else.
type overrideStorer struct {
	gitstore.Storer

	getObjectSignatureErr error
	gitConfig             map[string]string
	gitConfigErr          error
	writeBlobErr          error
}

func (o *overrideStorer) GetObjectSignature(objectID gitinterface.Hash) ([]byte, []byte, error) {
	if o.getObjectSignatureErr != nil {
		return nil, nil, o.getObjectSignatureErr
	}
	return o.Storer.GetObjectSignature(objectID)
}

func (o *overrideStorer) GetGitConfig() (map[string]string, error) {
	if o.gitConfigErr != nil {
		return nil, o.gitConfigErr
	}
	if o.gitConfig != nil {
		return o.gitConfig, nil
	}
	return o.Storer.GetGitConfig()
}

func (o *overrideStorer) WriteBlob(contents []byte) (gitinterface.Hash, error) {
	if o.writeBlobErr != nil {
		return nil, o.writeBlobErr
	}
	return o.Storer.WriteBlob(contents)
}

func TestSignatureVerifier(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	gpgKeyR, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	gpgKey := tufv01.NewKeyFromSSLibKey(gpgKeyR)

	rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	rootPubKeyR := rootSigner.MetadataKey()
	rootPubKey := tufv01.NewKeyFromSSLibKey(rootPubKeyR)

	targetsSigner := setupSSHKeysForSigning(t, targets1KeyBytes, targets1PubKeyBytes)
	targetsPubKeyR := targetsSigner.MetadataKey()
	targetsPubKey := tufv01.NewKeyFromSSLibKey(targetsPubKeyR)

	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, "refs/heads/main", 1, gpgKeyBytes)
	commitID := commitIDs[0]
	tagID := common.CreateTestSignedTag(t, repo, "test-tag", commitID, gpgKeyBytes)

	attestation, err := dsse.CreateEnvelope(nil)
	if err != nil {
		t.Fatal(err)
	}
	attestation, err = dsse.SignEnvelope(testCtx, attestation, rootSigner)
	if err != nil {
		t.Fatal(err)
	}

	invalidAttestation, err := dsse.CreateEnvelope(nil)
	if err != nil {
		t.Fatal(err)
	}
	invalidAttestation, err = dsse.SignEnvelope(testCtx, invalidAttestation, targetsSigner)
	if err != nil {
		t.Fatal(err)
	}

	attestationWithTwoSigs, err := dsse.CreateEnvelope(nil)
	if err != nil {
		t.Fatal(err)
	}
	attestationWithTwoSigs, err = dsse.SignEnvelope(testCtx, attestationWithTwoSigs, rootSigner)
	if err != nil {
		t.Fatal(err)
	}
	attestationWithTwoSigs, err = dsse.SignEnvelope(testCtx, attestationWithTwoSigs, targetsSigner)
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]struct {
		principals  []tuf.Principal
		threshold   int
		gitObjectID gitinterface.Hash
		attestation *sslibdsse.Envelope

		expectedError error
	}{
		"commit, no attestation, valid key, threshold 1": {
			principals:  []tuf.Principal{gpgKey},
			threshold:   1,
			gitObjectID: commitID,
		},
		"commit, no attestation, valid key, threshold 2": {
			principals:    []tuf.Principal{gpgKey},
			threshold:     2,
			gitObjectID:   commitID,
			expectedError: ErrVerifierConditionsUnmet,
		},
		"commit, attestation, valid key, threshold 1": {
			principals:  []tuf.Principal{gpgKey},
			threshold:   1,
			gitObjectID: commitID,
			attestation: attestation,
		},
		"commit, attestation, valid keys, threshold 2": {
			principals:  []tuf.Principal{gpgKey, rootPubKey},
			threshold:   2,
			gitObjectID: commitID,
			attestation: attestation,
		},
		"commit, invalid signed attestation, threshold 2": {
			principals:    []tuf.Principal{gpgKey, rootPubKey},
			threshold:     2,
			gitObjectID:   commitID,
			attestation:   invalidAttestation,
			expectedError: ErrVerifierConditionsUnmet,
		},
		"commit, attestation, valid keys, threshold 3": {
			principals:  []tuf.Principal{gpgKey, rootPubKey, targetsPubKey},
			threshold:   3,
			gitObjectID: commitID,
			attestation: attestationWithTwoSigs,
		},
		"tag, no attestation, valid key, threshold 1": {
			principals:  []tuf.Principal{gpgKey},
			threshold:   1,
			gitObjectID: tagID,
		},
		"tag, no attestation, valid key, threshold 2": {
			principals:    []tuf.Principal{gpgKey},
			threshold:     2,
			gitObjectID:   tagID,
			expectedError: ErrVerifierConditionsUnmet,
		},
		"tag, attestation, valid key, threshold 1": {
			principals:  []tuf.Principal{gpgKey},
			threshold:   1,
			gitObjectID: tagID,
			attestation: attestation,
		},
		"tag, attestation, valid keys, threshold 2": {
			principals:  []tuf.Principal{gpgKey, rootPubKey},
			threshold:   2,
			gitObjectID: tagID,
			attestation: attestation,
		},
		"tag, invalid signed attestation, threshold 2": {
			principals:    []tuf.Principal{gpgKey, rootPubKey},
			threshold:     2,
			gitObjectID:   tagID,
			attestation:   invalidAttestation,
			expectedError: ErrVerifierConditionsUnmet,
		},
		"tag, attestation, valid keys, threshold 3": {
			principals:  []tuf.Principal{gpgKey, rootPubKey, targetsPubKey},
			threshold:   3,
			gitObjectID: tagID,
			attestation: attestationWithTwoSigs,
		},
	}

	for name, test := range tests {
		verifier := &SignatureVerifier{
			repository: repo,
			name:       "test-verifier",
			principals: test.principals,
			threshold:  test.threshold,
		}

		_, err := verifier.Verify(testCtx, test.gitObjectID, test.attestation)
		if test.expectedError == nil {
			assert.Nil(t, err, fmt.Sprintf("unexpected error in test '%s'", name))
		} else {
			assert.ErrorIs(t, err, test.expectedError, fmt.Sprintf("incorrect error received in test '%s'", name))
		}
	}
}

func TestSignatureVerifierSkipsGitVerificationForZeroAndNilIDs(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	rootPubKey := tufv01.NewKeyFromSSLibKey(rootSigner.MetadataKey())

	// Build a DSSE envelope signed by rootSigner with threshold 1.
	env, err := dsse.CreateEnvelope(nil)
	if err != nil {
		t.Fatal(err)
	}
	env, err = dsse.SignEnvelope(testCtx, env, rootSigner)
	if err != nil {
		t.Fatal(err)
	}

	verifier := &SignatureVerifier{
		repository: repo,
		name:       "test-verifier",
		principals: []tuf.Principal{rootPubKey},
		threshold:  1,
	}

	t.Run("nil object ID skips git verification", func(t *testing.T) {
		t.Parallel()
		_, err := verifier.Verify(testCtx, nil, env)
		assert.Nil(t, err)
	})

	t.Run("zero object ID skips git verification", func(t *testing.T) {
		t.Parallel()
		_, err := verifier.Verify(testCtx, gitinterface.ZeroHash, env)
		assert.Nil(t, err)
	})
}

func TestSignatureVerifierInvalidVerifier(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	gpgKeyR, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
	require.Nil(t, err)
	gpgKey := tufv01.NewKeyFromSSLibKey(gpgKeyR)

	t.Run("zero threshold", func(t *testing.T) {
		t.Parallel()
		verifier := &SignatureVerifier{
			repository: repo,
			name:       "test-verifier",
			principals: []tuf.Principal{gpgKey},
			threshold:  0,
		}

		_, err := verifier.Verify(testCtx, nil, nil)
		assert.ErrorIs(t, err, ErrInvalidVerifier)
	})

	t.Run("no principals", func(t *testing.T) {
		t.Parallel()
		verifier := &SignatureVerifier{
			repository: repo,
			name:       "test-verifier",
			threshold:  1,
		}

		_, err := verifier.Verify(testCtx, nil, nil)
		assert.ErrorIs(t, err, ErrInvalidVerifier)
	})
}

func TestSignatureVerifierGitObjectStorerErrors(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	gpgKeyR, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
	require.Nil(t, err)
	gpgKey := tufv01.NewKeyFromSSLibKey(gpgKeyR)

	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, "refs/heads/main", 1, gpgKeyBytes)
	commitID := commitIDs[0]

	t.Run("get object signature error", func(t *testing.T) {
		t.Parallel()
		injected := errors.New("get object signature failure")
		verifier := &SignatureVerifier{
			repository: &overrideStorer{Storer: repo, getObjectSignatureErr: injected},
			name:       "test-verifier",
			principals: []tuf.Principal{gpgKey},
			threshold:  1,
		}

		_, err := verifier.Verify(testCtx, commitID, nil)
		assert.ErrorIs(t, err, injected)
	})

	t.Run("git config error", func(t *testing.T) {
		t.Parallel()
		injected := errors.New("git config failure")
		verifier := &SignatureVerifier{
			repository: &overrideStorer{Storer: repo, gitConfigErr: injected},
			name:       "test-verifier",
			principals: []tuf.Principal{gpgKey},
			threshold:  1,
		}

		_, err := verifier.Verify(testCtx, commitID, nil)
		assert.ErrorIs(t, err, injected)
	})

	t.Run("rekor URL in git config", func(t *testing.T) {
		t.Parallel()
		verifier := &SignatureVerifier{
			repository: &overrideStorer{Storer: repo, gitConfig: map[string]string{sigstore.GitConfigRekor: "https://rekor.example.com"}},
			name:       "test-verifier",
			principals: []tuf.Principal{gpgKey},
			threshold:  1,
		}

		usedPrincipalIDs, err := verifier.Verify(testCtx, commitID, nil)
		assert.Nil(t, err)
		assert.True(t, usedPrincipalIDs.Has(gpgKey.ID()))
	})
}

func TestSignatureVerifierUnknownSigningMethod(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	unknownKey := tufv01.NewKeyFromSSLibKey(&signerverifier.SSLibKey{
		KeyID:   "unknown-key",
		KeyType: "unknown-method",
		Scheme:  "unknown-scheme",
	})

	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, "refs/heads/main", 1, gpgKeyBytes)
	commitID := commitIDs[0]

	env, err := dsse.CreateEnvelope(nil)
	require.Nil(t, err)

	t.Run("git object signature skipped", func(t *testing.T) {
		t.Parallel()
		verifier := &SignatureVerifier{
			repository: repo,
			name:       "test-verifier",
			principals: []tuf.Principal{unknownKey},
			threshold:  1,
		}

		_, err := verifier.Verify(testCtx, commitID, nil)
		assert.ErrorIs(t, err, ErrVerifierConditionsUnmet)
	})

	t.Run("envelope verification rejects unknown key type", func(t *testing.T) {
		t.Parallel()
		verifier := &SignatureVerifier{
			repository: repo,
			name:       "test-verifier",
			principals: []tuf.Principal{unknownKey},
			threshold:  1,
		}

		_, err := verifier.Verify(testCtx, nil, env)
		assert.ErrorIs(t, err, svcommon.ErrUnknownKeyType)
	})
}

func TestSignatureVerifierSharedKeyAcrossPrincipals(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	gpgKeyR, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
	require.Nil(t, err)
	gpgKey := tufv01.NewKeyFromSSLibKey(gpgKeyR)
	person := &tufv02.Person{
		PersonID:   "jane.doe@example.com",
		PublicKeys: map[string]*tufv02.Key{gpgKey.KeyID: gpgKey},
	}

	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, "refs/heads/main", 1, gpgKeyBytes)
	commitID := commitIDs[0]

	env, err := dsse.CreateEnvelope(nil)
	require.Nil(t, err)

	verifier := &SignatureVerifier{
		repository: repo,
		name:       "test-verifier",
		principals: []tuf.Principal{gpgKey, person},
		threshold:  2,
	}

	// The person shares the key already counted for the Git signature, so
	// they cannot be counted a second time towards the threshold.
	usedPrincipalIDs, err := verifier.Verify(testCtx, commitID, env)
	assert.ErrorIs(t, err, ErrVerifierConditionsUnmet)
	assert.True(t, usedPrincipalIDs.Has(gpgKey.ID()))
	assert.False(t, usedPrincipalIDs.Has(person.ID()))
}

func TestSignatureVerifierEnvelopeVerifierErrors(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	env, err := dsse.CreateEnvelope(nil)
	require.Nil(t, err)

	t.Run("malformed ssh key", func(t *testing.T) {
		t.Parallel()
		badSSHKey := tufv01.NewKeyFromSSLibKey(&signerverifier.SSLibKey{
			KeyID:   "bad-ssh-key",
			KeyType: ssh.KeyType,
			Scheme:  "ssh-ed25519",
			KeyVal:  signerverifier.KeyVal{Public: "not a valid ssh key"},
		})
		verifier := &SignatureVerifier{
			repository: repo,
			name:       "test-verifier",
			principals: []tuf.Principal{badSSHKey},
			threshold:  1,
		}

		_, err := verifier.Verify(testCtx, nil, env)
		assert.ErrorContains(t, err, "failed to parse ssh public key material")
	})

	t.Run("malformed gpg key", func(t *testing.T) {
		t.Parallel()
		badGPGKey := tufv01.NewKeyFromSSLibKey(&signerverifier.SSLibKey{
			KeyID:   "bad-gpg-key",
			KeyType: gpg.KeyType,
			Scheme:  gpg.KeyType,
			KeyVal:  signerverifier.KeyVal{Public: "not a valid gpg key"},
		})
		verifier := &SignatureVerifier{
			repository: repo,
			name:       "test-verifier",
			principals: []tuf.Principal{badGPGKey},
			threshold:  1,
		}

		_, err := verifier.Verify(testCtx, nil, env)
		assert.ErrorContains(t, err, "failed to parse gpg key")
	})

	t.Run("sigstore git config error", func(t *testing.T) {
		t.Parallel()
		sigstoreKey := tufv01.NewKeyFromSSLibKey(&signerverifier.SSLibKey{
			KeyID:   "jane.doe@example.com::https://oidc.example.com",
			KeyType: sigstore.KeyType,
			Scheme:  sigstore.KeyScheme,
			KeyVal:  signerverifier.KeyVal{Identity: "jane.doe@example.com", Issuer: "https://oidc.example.com"},
		})
		injected := errors.New("git config failure")
		verifier := &SignatureVerifier{
			repository: &overrideStorer{Storer: repo, gitConfigErr: injected},
			name:       "test-verifier",
			principals: []tuf.Principal{sigstoreKey},
			threshold:  1,
		}

		_, err := verifier.Verify(testCtx, nil, env)
		assert.ErrorIs(t, err, injected)
	})

	t.Run("sigstore rekor config with unsigned envelope", func(t *testing.T) {
		t.Parallel()
		sigstoreKey := tufv01.NewKeyFromSSLibKey(&signerverifier.SSLibKey{
			KeyID:   "jane.doe@example.com::https://oidc.example.com",
			KeyType: sigstore.KeyType,
			Scheme:  sigstore.KeyScheme,
			KeyVal:  signerverifier.KeyVal{Identity: "jane.doe@example.com", Issuer: "https://oidc.example.com"},
		})
		verifier := &SignatureVerifier{
			repository: &overrideStorer{Storer: repo, gitConfig: map[string]string{sigstore.GitConfigRekor: "https://rekor.example.com"}},
			name:       "test-verifier",
			principals: []tuf.Principal{sigstoreKey},
			threshold:  1,
		}

		// The unsigned envelope makes DSSE verification fail before any
		// network access. The sigstore verifier setup (including the Rekor
		// URL option) still executes.
		_, err := verifier.Verify(testCtx, nil, env)
		assert.ErrorIs(t, err, sslibdsse.ErrNoSignature)
	})
}
