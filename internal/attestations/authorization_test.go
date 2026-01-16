// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package attestations

import (
	"testing"

	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	ita "github.com/in-toto/attestation/go/v1"
	"github.com/stretchr/testify/assert"
)

func TestSetReferenceAuthorization(t *testing.T) {
	t.Run("for commit", func(t *testing.T) {
		testRef := "refs/heads/main"
		testAnotherRef := "refs/heads/feature"
		testID := gitinterface.ZeroHash.String()
		mainZeroZero := createReferenceAuthorizationAttestationEnvelopes(t, testRef, testID, testID, false)
		featureZeroZero := createReferenceAuthorizationAttestationEnvelopes(t, testAnotherRef, testID, testID, false)

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
	})

	t.Run("for tag", func(t *testing.T) {
		tagRef := "refs/tags/v1"
		testID := gitinterface.ZeroHash.String()
		tagApproval := createReferenceAuthorizationAttestationEnvelopes(t, tagRef, testID, testID, true)

		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		attestations := &Attestations{}

		err := attestations.SetReferenceAuthorization(repo, tagApproval, tagRef, testID, testID)
		assert.Nil(t, err)
		assert.Contains(t, attestations.referenceAuthorizations, ReferenceAuthorizationPath(tagRef, testID, testID))
	})
}

func TestRemoveReferenceAuthorization(t *testing.T) {
	t.Run("for commit", func(t *testing.T) {
		testRef := "refs/heads/main"
		testAnotherRef := "refs/heads/feature"
		testID := gitinterface.ZeroHash.String()
		mainZeroZero := createReferenceAuthorizationAttestationEnvelopes(t, testRef, testID, testID, false)
		featureZeroZero := createReferenceAuthorizationAttestationEnvelopes(t, testAnotherRef, testID, testID, false)

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
	})

	t.Run("for tag", func(t *testing.T) {
		tagRef := "refs/tags/v1"
		testID := gitinterface.ZeroHash.String()
		tagApproval := createReferenceAuthorizationAttestationEnvelopes(t, tagRef, testID, testID, true)

		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		attestations := &Attestations{}

		err := attestations.SetReferenceAuthorization(repo, tagApproval, tagRef, testID, testID)
		if err != nil {
			t.Fatal(err)
		}
		assert.Contains(t, attestations.referenceAuthorizations, ReferenceAuthorizationPath(tagRef, testID, testID))

		err = attestations.RemoveReferenceAuthorization(tagRef, testID, testID)
		assert.Nil(t, err)
		assert.Empty(t, attestations.referenceAuthorizations)
	})
}

func TestGetReferenceAuthorizationFor(t *testing.T) {
	t.Run("for commit", func(t *testing.T) {
		testRef := "refs/heads/main"
		testAnotherRef := "refs/heads/feature"
		testID := gitinterface.ZeroHash.String()
		mainZeroZero := createReferenceAuthorizationAttestationEnvelopes(t, testRef, testID, testID, false)
		featureZeroZero := createReferenceAuthorizationAttestationEnvelopes(t, testAnotherRef, testID, testID, false)

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
	})

	t.Run("for tag", func(t *testing.T) {
		tagRef := "refs/tags/v1"
		testID := gitinterface.ZeroHash.String()
		tagApproval := createReferenceAuthorizationAttestationEnvelopes(t, tagRef, testID, testID, true)

		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		attestations := &Attestations{}

		err := attestations.SetReferenceAuthorization(repo, tagApproval, tagRef, testID, testID)
		if err != nil {
			t.Fatal(err)
		}
		assert.Contains(t, attestations.referenceAuthorizations, ReferenceAuthorizationPath(tagRef, testID, testID))

		tagApprovalFetched, err := attestations.GetReferenceAuthorizationFor(repo, tagRef, testID, testID)
		assert.Nil(t, err)
		assert.Equal(t, tagApproval, tagApprovalFetched)
	})
}

func createReferenceAuthorizationAttestationEnvelopes(t *testing.T, refName, fromID, toID string, tag bool) *sslibdsse.Envelope {
	t.Helper()

	var (
		authorization *ita.Statement
		err           error
	)
	if tag {
		authorization, err = NewReferenceAuthorizationForTag(refName, fromID, toID)
	} else {
		authorization, err = NewReferenceAuthorizationForCommit(refName, fromID, toID)
	}
	if err != nil {
		t.Fatal(err)
	}

	env, err := dsse.CreateEnvelope(authorization)
	if err != nil {
		t.Fatal(err)
	}

	return env
}
