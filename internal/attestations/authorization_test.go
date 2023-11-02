// SPDX-License-Identifier: Apache-2.0

package attestations

import (
	"testing"

	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/third_party/go-git"
	"github.com/gittuf/gittuf/internal/third_party/go-git/plumbing"
	"github.com/gittuf/gittuf/internal/third_party/go-git/storage/memory"
	"github.com/go-git/go-billy/v5/memfs"
	ita "github.com/in-toto/attestation/go/v1"
	sslibdsse "github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/stretchr/testify/assert"
)

func TestNewAuthorizationAttestation(t *testing.T) {
	testRef := "refs/heads/main"
	testID := plumbing.ZeroHash.String()

	authorization, err := NewAuthorizationAttestation(testRef, testID, testID)
	assert.Nil(t, err)

	// Check value of statement type
	assert.Equal(t, ita.StatementTypeUri, authorization.Type)

	// Check subject contents
	assert.Equal(t, 1, len(authorization.Subject))
	assert.Contains(t, authorization.Subject[0].Digest, digestGitCommitKey)
	assert.Equal(t, authorization.Subject[0].Digest[digestGitCommitKey], testID)

	// Check predicate type
	assert.Equal(t, AuthorizationPredicateType, authorization.PredicateType)

	// Check predicate
	predicate := authorization.Predicate.AsMap()
	assert.Equal(t, predicate["targetRef"], testRef)
	assert.Equal(t, predicate["toTargetID"], testID)
	assert.Equal(t, predicate["fromTargetID"], testID)
}

func TestAddAuthorizationAttestation(t *testing.T) {
	testRef := "refs/heads/main"
	testAnotherRef := "refs/heads/feature"
	testID := plumbing.ZeroHash.String()
	mainZeroZero := createUnsignedAuthorizationAttestationEnvelopes(t, testRef, testID, testID)
	featureZeroZero := createUnsignedAuthorizationAttestationEnvelopes(t, testAnotherRef, testID, testID)

	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	attestations := &Attestations{}

	err = attestations.AddAuthorizationAttestation(repo, mainZeroZero, testRef, testID, testID)
	assert.Nil(t, err)
	assert.Contains(t, attestations.authorizations, authorizationPath(testRef, testID, testID))
	assert.NotContains(t, attestations.authorizations, authorizationPath(testAnotherRef, testID, testID))

	err = attestations.AddAuthorizationAttestation(repo, featureZeroZero, testAnotherRef, testID, testID)
	assert.Nil(t, err)
	assert.Contains(t, attestations.authorizations, authorizationPath(testRef, testID, testID))
	assert.Contains(t, attestations.authorizations, authorizationPath(testAnotherRef, testID, testID))
}

func createUnsignedAuthorizationAttestationEnvelopes(t *testing.T, refName, fromID, toID string) *sslibdsse.Envelope {
	t.Helper()

	authorization, err := NewAuthorizationAttestation(refName, fromID, toID)
	if err != nil {
		t.Fatal(err)
	}
	env, err := dsse.CreateEnvelope(authorization)
	if err != nil {
		t.Fatal(err)
	}

	return env
}
