// SPDX-License-Identifier: Apache-2.0

package v02

import (
	"testing"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
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
		assert.Equal(t, ReferenceAuthorizationPredicateType, authorization.PredicateType)

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
		assert.Equal(t, ReferenceAuthorizationPredicateType, authorization.PredicateType)

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
		mainZeroZero := CreateTestEnvelope(t, testRef, testID, testID, false)
		featureZeroZero := CreateTestEnvelope(t, testAnotherRef, testID, testID, false)

		err := Validate(mainZeroZero, testRef, testID, testID)
		assert.Nil(t, err)

		err = Validate(featureZeroZero, testAnotherRef, testID, testID)
		assert.Nil(t, err)

		err = Validate(mainZeroZero, testAnotherRef, testID, testID)
		assert.ErrorIs(t, err, ErrInvalidAuthorization)
	})

	t.Run("for tag", func(t *testing.T) {
		testRef := "refs/tags/v1"
		testID := gitinterface.ZeroHash.String()
		authorization := CreateTestEnvelope(t, testRef, testID, testID, true)

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
		assert.ErrorIs(t, err, ErrInvalidAuthorization)
	})

	t.Run("mismatch ref (non tag) and subject digest key (commit)", func(t *testing.T) {
		testRef := "refs/heads/main"
		testID := gitinterface.ZeroHash.String()
		authorization := CreateTestEnvelope(t, testRef, testID, testID, true)

		err := Validate(authorization, testRef, testID, testID)
		assert.ErrorIs(t, err, ErrInvalidAuthorization)
	})
}
