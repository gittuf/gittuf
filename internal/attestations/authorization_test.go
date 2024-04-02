// SPDX-License-Identifier: Apache-2.0

package attestations

import (
	"testing"

	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
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
	assert.Contains(t, authorization.Subject[0].Digest, digestGitCommitKey)
	assert.Equal(t, authorization.Subject[0].Digest[digestGitCommitKey], testID)

	// Check predicate type
	assert.Equal(t, ReferenceAuthorizationPredicateType, authorization.PredicateType)

	// Check predicate
	predicate := authorization.Predicate.AsMap()
	assert.Equal(t, predicate["targetRef"], testRef)
	assert.Equal(t, predicate["toTargetID"], testID)
	assert.Equal(t, predicate["fromTargetID"], testID)
}

func TestSetReferenceAuthorization(t *testing.T) {
	testRef := "refs/heads/main"
	testAnotherRef := "refs/heads/feature"
	testID := plumbing.ZeroHash.String()
	mainZeroZero := createReferenceAuthorizationAttestationEnvelopes(t, testRef, testID, testID)
	featureZeroZero := createReferenceAuthorizationAttestationEnvelopes(t, testAnotherRef, testID, testID)

	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	attestations := &Attestations{}

	// Add auth for first branch
	err = attestations.SetReferenceAuthorization(repo, mainZeroZero, testRef, testID, testID)
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
	testID := plumbing.ZeroHash.String()
	mainZeroZero := createReferenceAuthorizationAttestationEnvelopes(t, testRef, testID, testID)
	featureZeroZero := createReferenceAuthorizationAttestationEnvelopes(t, testAnotherRef, testID, testID)

	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	attestations := &Attestations{}

	err = attestations.SetReferenceAuthorization(repo, mainZeroZero, testRef, testID, testID)
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
	testID := plumbing.ZeroHash.String()
	mainZeroZero := createReferenceAuthorizationAttestationEnvelopes(t, testRef, testID, testID)
	featureZeroZero := createReferenceAuthorizationAttestationEnvelopes(t, testAnotherRef, testID, testID)

	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	attestations := &Attestations{}

	err = attestations.SetReferenceAuthorization(repo, mainZeroZero, testRef, testID, testID)
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
	testID := plumbing.ZeroHash.String()
	mainZeroZero := createReferenceAuthorizationAttestationEnvelopes(t, testRef, testID, testID)
	featureZeroZero := createReferenceAuthorizationAttestationEnvelopes(t, testAnotherRef, testID, testID)

	err := validateReferenceAuthorization(mainZeroZero, testRef, testID, testID)
	assert.Nil(t, err)

	err = validateReferenceAuthorization(featureZeroZero, testAnotherRef, testID, testID)
	assert.Nil(t, err)

	err = validateReferenceAuthorization(mainZeroZero, testAnotherRef, testID, testID)
	assert.ErrorIs(t, err, ErrInvalidAuthorization)
}

func TestNewAuthenticationEvidence(t *testing.T) {
	testRef := "refs/heads/main"
	testPushActor := "testPushActor"
	testID := plumbing.ZeroHash.String()

	authorization, err := NewAuthenticationEvidence(testRef, testID, testID, testPushActor)
	assert.Nil(t, err)

	// Check value of statement type
	assert.Equal(t, ita.StatementTypeUri, authorization.Type)

	// Check subject contents
	assert.Equal(t, 1, len(authorization.Subject))
	assert.Contains(t, authorization.Subject[0].Digest, digestGitCommitKey)
	assert.Equal(t, authorization.Subject[0].Digest[digestGitCommitKey], testID)

	// Check predicate type
	assert.Equal(t, AuthenticationEvidencePredicateType, authorization.PredicateType)

	// Check predicate
	predicate := authorization.Predicate.AsMap()
	assert.Equal(t, predicate["targetRef"], testRef)
	assert.Equal(t, predicate["toTargetID"], testID)
	assert.Equal(t, predicate["fromTargetID"], testID)
	assert.Equal(t, predicate["pushActor"], testPushActor)
}

func TestSetAuthenticationEvidence(t *testing.T) {
	testRef := "refs/heads/main"
	testAnotherRef := "refs/heads/feature"
	pushActor := "testPushActor"
	testID := plumbing.ZeroHash.String()
	mainZeroZero := createAuthenticationEvidenceAttestationEnvelopes(t, testRef, testID, testID, pushActor)
	featureZeroZero := createAuthenticationEvidenceAttestationEnvelopes(t, testAnotherRef, testID, testID, pushActor)

	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	attestations := &Attestations{}

	// Add auth for first branch
	err = attestations.SetAuthenticationEvidence(repo, mainZeroZero, testRef, testID, testID)
	assert.Nil(t, err)
	assert.Contains(t, attestations.authenticationEvidence, AuthenticationEvidencePath(testRef, testID, testID))
	assert.NotContains(t, attestations.authenticationEvidence, AuthenticationEvidencePath(testAnotherRef, testID, testID))

	// Add auth for the other branch
	err = attestations.SetAuthenticationEvidence(repo, featureZeroZero, testAnotherRef, testID, testID)
	assert.Nil(t, err)
	assert.Contains(t, attestations.authenticationEvidence, AuthenticationEvidencePath(testRef, testID, testID))
	assert.Contains(t, attestations.authenticationEvidence, AuthenticationEvidencePath(testAnotherRef, testID, testID))
}

func TestRemoveAuthenticationEvidence(t *testing.T) {
	testRef := "refs/heads/main"
	testAnotherRef := "refs/heads/feature"
	pushActor := "testPushActor"
	testID := plumbing.ZeroHash.String()
	mainZeroZero := createAuthenticationEvidenceAttestationEnvelopes(t, testRef, testID, testID, pushActor)
	featureZeroZero := createAuthenticationEvidenceAttestationEnvelopes(t, testAnotherRef, testID, testID, pushActor)

	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	attestations := &Attestations{}

	err = attestations.SetAuthenticationEvidence(repo, mainZeroZero, testRef, testID, testID)
	assert.Nil(t, err)
	assert.Contains(t, attestations.authenticationEvidence, AuthenticationEvidencePath(testRef, testID, testID))
	assert.NotContains(t, attestations.authenticationEvidence, AuthenticationEvidencePath(testAnotherRef, testID, testID))

	err = attestations.SetAuthenticationEvidence(repo, featureZeroZero, testAnotherRef, testID, testID)
	assert.Nil(t, err)
	assert.Contains(t, attestations.authenticationEvidence, AuthenticationEvidencePath(testRef, testID, testID))
	assert.Contains(t, attestations.authenticationEvidence, AuthenticationEvidencePath(testAnotherRef, testID, testID))

	err = attestations.RemoveAuthenticationEvidence(testAnotherRef, testID, testID)
	assert.Nil(t, err)
	assert.Contains(t, attestations.authenticationEvidence, AuthenticationEvidencePath(testRef, testID, testID))
	assert.NotContains(t, attestations.authenticationEvidence, AuthenticationEvidencePath(testAnotherRef, testID, testID))

	err = attestations.RemoveAuthenticationEvidence(testRef, testID, testID)
	assert.Nil(t, err)
	assert.NotContains(t, attestations.authenticationEvidence, AuthenticationEvidencePath(testRef, testID, testID))
	assert.NotContains(t, attestations.authenticationEvidence, AuthenticationEvidencePath(testAnotherRef, testID, testID))
}

func TestGetAuthenticationEvidenceFor(t *testing.T) {
	testRef := "refs/heads/main"
	testAnotherRef := "refs/heads/feature"
	testPushActor := "testPushActor"
	testID := plumbing.ZeroHash.String()
	mainZeroZero := createAuthenticationEvidenceAttestationEnvelopes(t, testRef, testID, testID, testPushActor)
	featureZeroZero := createAuthenticationEvidenceAttestationEnvelopes(t, testAnotherRef, testID, testID, testPushActor)

	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	attestations := &Attestations{}

	err = attestations.SetAuthenticationEvidence(repo, mainZeroZero, testRef, testID, testID)
	if err != nil {
		t.Fatal(err)
	}
	err = attestations.SetAuthenticationEvidence(repo, featureZeroZero, testAnotherRef, testID, testID)
	if err != nil {
		t.Fatal(err)
	}

	mainAuth, err := attestations.GetAuthenticationEvidenceFor(repo, testRef, testID, testID)
	assert.Nil(t, err)
	assert.Equal(t, mainZeroZero, mainAuth)

	featureAuth, err := attestations.GetAuthenticationEvidenceFor(repo, testAnotherRef, testID, testID)
	assert.Nil(t, err)
	assert.Equal(t, featureZeroZero, featureAuth)
}

func TestValidateAuthorizationEvidence(t *testing.T) {
	testRef := "refs/heads/main"
	testAnotherRef := "refs/heads/feature"
	testPushActor := "testPushActor"
	testID := plumbing.ZeroHash.String()
	mainZeroZero := createAuthenticationEvidenceAttestationEnvelopes(t, testRef, testID, testID, testPushActor)
	featureZeroZero := createAuthenticationEvidenceAttestationEnvelopes(t, testAnotherRef, testID, testID, testPushActor)

	err := validateAuthenticationEvidence(mainZeroZero, testRef, testID, testID)
	assert.Nil(t, err)

	err = validateAuthenticationEvidence(featureZeroZero, testAnotherRef, testID, testID)
	assert.Nil(t, err)

	err = validateAuthenticationEvidence(mainZeroZero, testAnotherRef, testID, testID)
	assert.ErrorIs(t, err, ErrInvalidEvidence)
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

func createAuthenticationEvidenceAttestationEnvelopes(t *testing.T, refName, fromID, toID, pushActor string) *sslibdsse.Envelope {
	t.Helper()

	authorization, err := NewAuthenticationEvidence(refName, fromID, toID, pushActor)
	if err != nil {
		t.Fatal(err)
	}
	env, err := dsse.CreateEnvelope(authorization)
	if err != nil {
		t.Fatal(err)
	}

	return env
}
