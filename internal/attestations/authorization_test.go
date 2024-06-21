// SPDX-License-Identifier: Apache-2.0

package attestations

import (
	"testing"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/go-git/go-git/v5/plumbing"
	ita "github.com/in-toto/attestation/go/v1"
	sslibdsse "github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/stretchr/testify/assert"
)

func TestNewReferenceAuthorization(t *testing.T) {
	testRef := "refs/heads/main"
	testID := plumbing.ZeroHash.String()

	authorization, err := NewReferenceAuthorization(testRef, testID, testID)
	assert.Nil(t, err)

	// Check value of statement type
	assert.Equal(t, ita.StatementTypeUri, authorization.Type)

	// Check subject contents
	assert.Equal(t, 1, len(authorization.Subject))
	assert.Contains(t, authorization.Subject[0].Digest, digestGitTreeKey)
	assert.Equal(t, authorization.Subject[0].Digest[digestGitTreeKey], testID)

	// Check predicate type
	assert.Equal(t, ReferenceAuthorizationPredicateType, authorization.PredicateType)

	// Check predicate
	predicate := authorization.Predicate.AsMap()
	assert.Equal(t, predicate[targetRefKey], testRef)
	assert.Equal(t, predicate[targetTreeIDKey], testID)
	assert.Equal(t, predicate[fromRevisionIDKey], testID)
}

func TestSetReferenceAuthorization(t *testing.T) {
	testRef := "refs/heads/main"
	testAnotherRef := "refs/heads/feature"
	testID := gitinterface.ZeroHash.String()
	mainZeroZero := createReferenceAuthorizationAttestationEnvelopes(t, testRef, testID, testID)
	featureZeroZero := createReferenceAuthorizationAttestationEnvelopes(t, testAnotherRef, testID, testID)

	tempDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

	attestations := &Attestations{}

	// Add auth for first branch
	err := attestations.SetReferenceAuthorization(repo, mainZeroZero, testRef, testID, testID)
	assert.Nil(t, err)
	assert.Contains(t, attestations.referenceAuthorizations, ReferenceAuthorizationPath(testRef, testID, testID))
	assert.NotContains(t, attestations.referenceAuthorizations, ReferenceAuthorizationPath(testAnotherRef, testID, testID))

	// Add auth for the other branch
	err = attestations.SetReferenceAuthorization(repo, featureZeroZero, testAnotherRef, testID, testID)
	assert.Nil(t, err)
	assert.Contains(t, attestations.referenceAuthorizations, ReferenceAuthorizationPath(testRef, testID, testID))
	assert.Contains(t, attestations.referenceAuthorizations, ReferenceAuthorizationPath(testAnotherRef, testID, testID))
}

func TestRemoveReferenceAuthorization(t *testing.T) {
	testRef := "refs/heads/main"
	testAnotherRef := "refs/heads/feature"
	testID := gitinterface.ZeroHash.String()
	mainZeroZero := createReferenceAuthorizationAttestationEnvelopes(t, testRef, testID, testID)
	featureZeroZero := createReferenceAuthorizationAttestationEnvelopes(t, testAnotherRef, testID, testID)

	tempDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

	attestations := &Attestations{}

	err := attestations.SetReferenceAuthorization(repo, mainZeroZero, testRef, testID, testID)
	if err != nil {
		t.Fatal(err)
	}
	assert.Contains(t, attestations.referenceAuthorizations, ReferenceAuthorizationPath(testRef, testID, testID))
	assert.NotContains(t, attestations.referenceAuthorizations, ReferenceAuthorizationPath(testAnotherRef, testID, testID))

	err = attestations.SetReferenceAuthorization(repo, featureZeroZero, testAnotherRef, testID, testID)
	if err != nil {
		t.Fatal(err)
	}
	assert.Contains(t, attestations.referenceAuthorizations, ReferenceAuthorizationPath(testRef, testID, testID))
	assert.Contains(t, attestations.referenceAuthorizations, ReferenceAuthorizationPath(testAnotherRef, testID, testID))

	err = attestations.RemoveReferenceAuthorization(testAnotherRef, testID, testID)
	assert.Nil(t, err)
	assert.Contains(t, attestations.referenceAuthorizations, ReferenceAuthorizationPath(testRef, testID, testID))
	assert.NotContains(t, attestations.referenceAuthorizations, ReferenceAuthorizationPath(testAnotherRef, testID, testID))

	err = attestations.RemoveReferenceAuthorization(testRef, testID, testID)
	assert.Nil(t, err)
	assert.NotContains(t, attestations.referenceAuthorizations, ReferenceAuthorizationPath(testRef, testID, testID))
	assert.NotContains(t, attestations.referenceAuthorizations, ReferenceAuthorizationPath(testAnotherRef, testID, testID))
}

func TestGetReferenceAuthorizationFor(t *testing.T) {
	testRef := "refs/heads/main"
	testAnotherRef := "refs/heads/feature"
	testID := gitinterface.ZeroHash.String()
	mainZeroZero := createReferenceAuthorizationAttestationEnvelopes(t, testRef, testID, testID)
	featureZeroZero := createReferenceAuthorizationAttestationEnvelopes(t, testAnotherRef, testID, testID)

	tempDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

	attestations := &Attestations{}

	err := attestations.SetReferenceAuthorization(repo, mainZeroZero, testRef, testID, testID)
	if err != nil {
		t.Fatal(err)
	}
	err = attestations.SetReferenceAuthorization(repo, featureZeroZero, testAnotherRef, testID, testID)
	if err != nil {
		t.Fatal(err)
	}

	mainAuth, err := attestations.GetReferenceAuthorizationFor(repo, testRef, testID, testID)
	assert.Nil(t, err)
	assert.Equal(t, mainZeroZero, mainAuth)

	featureAuth, err := attestations.GetReferenceAuthorizationFor(repo, testAnotherRef, testID, testID)
	assert.Nil(t, err)
	assert.Equal(t, featureZeroZero, featureAuth)
}

func TestValidateReferenceAuthorization(t *testing.T) {
	testRef := "refs/heads/main"
	testAnotherRef := "refs/heads/feature"
	testID := gitinterface.ZeroHash.String()
	mainZeroZero := createReferenceAuthorizationAttestationEnvelopes(t, testRef, testID, testID)
	featureZeroZero := createReferenceAuthorizationAttestationEnvelopes(t, testAnotherRef, testID, testID)

	err := validateReferenceAuthorization(mainZeroZero, testRef, testID, testID)
	assert.Nil(t, err)

	err = validateReferenceAuthorization(featureZeroZero, testAnotherRef, testID, testID)
	assert.Nil(t, err)

	err = validateReferenceAuthorization(mainZeroZero, testAnotherRef, testID, testID)
	assert.ErrorIs(t, err, ErrInvalidAuthorization)
}

func createReferenceAuthorizationAttestationEnvelopes(t *testing.T, refName, fromID, toID string) *sslibdsse.Envelope {
	t.Helper()

	authorization, err := NewReferenceAuthorization(refName, fromID, toID)
	if err != nil {
		t.Fatal(err)
	}
	env, err := dsse.CreateEnvelope(authorization)
	if err != nil {
		t.Fatal(err)
	}

	return env
}
