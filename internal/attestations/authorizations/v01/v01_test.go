// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v01

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
	testRef := "refs/heads/main"
	testID := gitinterface.ZeroHash.String()

	authorization, err := NewReferenceAuthorization(testRef, testID, testID)
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
	assert.Equal(t, predicate[targetTreeIDKey], testID)
	assert.Equal(t, predicate[fromRevisionIDKey], testID)
}

func TestValidate(t *testing.T) {
	testRef := "refs/heads/main"
	testAnotherRef := "refs/heads/feature"
	testID := gitinterface.ZeroHash.String()
	mainZeroZero := createTestEnvelope(t, testRef, testID, testID)
	featureZeroZero := createTestEnvelope(t, testAnotherRef, testID, testID)

	err := Validate(mainZeroZero, testRef, testID, testID)
	assert.Nil(t, err)

	err = Validate(featureZeroZero, testAnotherRef, testID, testID)
	assert.Nil(t, err)

	err = Validate(mainZeroZero, testAnotherRef, testID, testID)
	assert.ErrorIs(t, err, authorizations.ErrInvalidAuthorization)
}

func createTestEnvelope(t *testing.T, refName, fromID, toID string) *sslibdsse.Envelope {
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
