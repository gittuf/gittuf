// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"fmt"
	"testing"

	"github.com/gittuf/gittuf/internal/common"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
)

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
