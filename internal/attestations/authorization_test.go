// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package attestations

import (
	"testing"
	"time"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
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

func TestCheckAuthorizationExpiration(t *testing.T) {
	refName := "refs/heads/main"
	fromID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	toID := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	t.Run("no expiration", func(t *testing.T) {
		// Create authorization without expiration
		att, err := NewReferenceAuthorizationForCommit(refName, fromID, toID, "")
		assert.Nil(t, err)

		env, err := dsse.CreateEnvelope(att)
		assert.Nil(t, err)

		// Check with any time
		err = CheckAuthorizationExpiration(env, time.Now())
		assert.Nil(t, err)
	})

	t.Run("future expiration", func(t *testing.T) {
		// Expires in 1 hour
		expires := time.Now().Add(time.Hour).Format(time.RFC3339)
		att, err := NewReferenceAuthorizationForCommit(refName, fromID, toID, expires)
		assert.Nil(t, err)

		env, err := dsse.CreateEnvelope(att)
		assert.Nil(t, err)

		// Check now (should pass)
		err = CheckAuthorizationExpiration(env, time.Now())
		assert.Nil(t, err)
	})

	t.Run("past expiration", func(t *testing.T) {
		// Expired 1 hour ago
		expires := time.Now().Add(-time.Hour).Format(time.RFC3339)
		att, err := NewReferenceAuthorizationForCommit(refName, fromID, toID, expires)
		assert.Nil(t, err)

		env, err := dsse.CreateEnvelope(att)
		assert.Nil(t, err)

		// Check now (should fail)
		err = CheckAuthorizationExpiration(env, time.Now())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "reference authorization expired")
	})

	t.Run("invalid expiration format", func(t *testing.T) {
		att, err := NewReferenceAuthorizationForCommit(refName, fromID, toID, "invalid-time")
		if err != nil {
			t.Log("NewReferenceAuthorizationForCommit might fail validation")
		}

		// If creation succeeds, verify check fails
		if att != nil {
			env, err := dsse.CreateEnvelope(att)
			assert.Nil(t, err)

			err = CheckAuthorizationExpiration(env, time.Now())
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid expiration timestamp")
		}
	})
}

func createReferenceAuthorizationAttestationEnvelopes(t *testing.T, refName, fromID, toID string, tag bool) *sslibdsse.Envelope {
	t.Helper()

	var (
		authorization *ita.Statement
		err           error
	)
	if tag {
		authorization, err = NewReferenceAuthorizationForTag(refName, fromID, toID, "")
	} else {
		authorization, err = NewReferenceAuthorizationForCommit(refName, fromID, toID, "")
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
