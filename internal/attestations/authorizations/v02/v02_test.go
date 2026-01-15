// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v02

import (
	"testing"

	"github.com/gittuf/gittuf/internal/attestations/authorizations"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/pkg/gitinterface"
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
		assert.Contains(t, authorization.Subject[0].Digest, digestGitTreeKey)
		assert.Equal(t, authorization.Subject[0].Digest[digestGitTreeKey], testID)

		// Check predicate type
		assert.Equal(t, PredicateType, authorization.PredicateType)

		// Check predicate
		predicate := authorization.Predicate.AsMap()
		assert.Equal(t, predicate[targetRefKey], testRef)
		assert.Equal(t, predicate[targetIDKey], testID)
		assert.Equal(t, predicate[fromIDKey], testID)
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
		assert.Contains(t, authorization.Subject[0].Digest, digestGitCommitKey)
		assert.Equal(t, authorization.Subject[0].Digest[digestGitCommitKey], testID)

		// Check predicate type
		assert.Equal(t, PredicateType, authorization.PredicateType)

		// Check predicate
		predicate := authorization.Predicate.AsMap()
		assert.Equal(t, predicate[targetRefKey], testRef)
		assert.Equal(t, predicate[targetIDKey], testID)
		assert.Equal(t, predicate[fromIDKey], testID)
	})
}

func TestValidate(t *testing.T) {
	t.Run("for commit", func(t *testing.T) {
		testRef := "refs/heads/main"
		testAnotherRef := "refs/heads/feature"
		testID := gitinterface.ZeroHash.String()
		mainZeroZero := createTestEnvelope(t, testRef, testID, testID, false)
		featureZeroZero := createTestEnvelope(t, testAnotherRef, testID, testID, false)

		err := Validate(mainZeroZero, testRef, testID, testID)
		assert.Nil(t, err)

		err = Validate(featureZeroZero, testAnotherRef, testID, testID)
		assert.Nil(t, err)

		err = Validate(mainZeroZero, testAnotherRef, testID, testID)
		assert.ErrorIs(t, err, authorizations.ErrInvalidAuthorization)
	})

	t.Run("for tag", func(t *testing.T) {
		testRef := "refs/tags/v1"
		testID := gitinterface.ZeroHash.String()
		authorization := createTestEnvelope(t, testRef, testID, testID, true)

		err := Validate(authorization, testRef, testID, testID)
		assert.Nil(t, err)
	})

	t.Run("invalid subject", func(t *testing.T) {
		testRef := "refs/heads/main"
		testID := gitinterface.ZeroHash.String()

		authorization, err := NewReferenceAuthorizationForCommit(testRef, testID, testID)
		if err != nil {
			t.Fatal(err)
		}

		authorization.Subject[0].Digest["garbage"] = authorization.Subject[0].Digest[digestGitTreeKey]
		delete(authorization.Subject[0].Digest, digestGitTreeKey)
		env, err := dsse.CreateEnvelope(authorization)
		if err != nil {
			t.Fatal(err)
		}

		err = Validate(env, testRef, testID, testID)
		assert.ErrorIs(t, err, authorizations.ErrInvalidAuthorization)
	})

	t.Run("mismatch ref (non tag) and subject digest key (commit)", func(t *testing.T) {
		testRef := "refs/heads/main"
		testID := gitinterface.ZeroHash.String()
		authorization := createTestEnvelope(t, testRef, testID, testID, true)

		err := Validate(authorization, testRef, testID, testID)
		assert.ErrorIs(t, err, authorizations.ErrInvalidAuthorization)
	})
}

func createTestEnvelope(t *testing.T, refName, fromID, toID string, tag bool) *sslibdsse.Envelope {
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
