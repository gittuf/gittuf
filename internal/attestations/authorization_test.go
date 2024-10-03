// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package attestations

import (
	"testing"

	authorizations "github.com/gittuf/gittuf/internal/attestations/authorizations/v02"
	"github.com/gittuf/gittuf/internal/gitinterface"
	ita "github.com/in-toto/attestation/go/v1"
	"github.com/stretchr/testify/assert"
)

func TestNewReferenceAuthorization(t *testing.T) {
	t.Run("for commit", func(t *testing.T) {
		testRef := "refs/heads/main"
		testID := gitinterface.ZeroHash.String()

		authorization, err := NewReferenceAuthorizationForCommit(testRef, testID, testID)
		assert.Nil(t, err)

		// Check value of statement type
		assert.Equal(t, ita.StatementTypeUri, authorization.Type)

		// Check subject contents
		assert.Equal(t, 1, len(authorization.Subject))

		// Check predicate type
		assert.Equal(t, authorizations.ReferenceAuthorizationPredicateType, authorization.PredicateType)
	})

	t.Run("for tag", func(t *testing.T) {
		testRef := "refs/heads/main"
		testID := gitinterface.ZeroHash.String()

		authorization, err := NewReferenceAuthorizationForTag(testRef, testID, testID)
		assert.Nil(t, err)

		// Check value of statement type
		assert.Equal(t, ita.StatementTypeUri, authorization.Type)

		// Check subject contents
		assert.Equal(t, 1, len(authorization.Subject))

		// Check predicate type
		assert.Equal(t, authorizations.ReferenceAuthorizationPredicateType, authorization.PredicateType)
	})
}

func TestSetReferenceAuthorization(t *testing.T) {
	testRef := "refs/heads/main"
	testAnotherRef := "refs/heads/feature"
	testID := gitinterface.ZeroHash.String()
	mainZeroZero := authorizations.CreateTestEnvelope(t, testRef, testID, testID, false)
	featureZeroZero := authorizations.CreateTestEnvelope(t, testAnotherRef, testID, testID, false)

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
	mainZeroZero := authorizations.CreateTestEnvelope(t, testRef, testID, testID, false)
	featureZeroZero := authorizations.CreateTestEnvelope(t, testAnotherRef, testID, testID, false)

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
	mainZeroZero := authorizations.CreateTestEnvelope(t, testRef, testID, testID, false)
	featureZeroZero := authorizations.CreateTestEnvelope(t, testAnotherRef, testID, testID, false)

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
