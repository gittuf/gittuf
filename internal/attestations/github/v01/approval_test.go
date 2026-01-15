// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v01

import (
	"testing"

	"github.com/gittuf/gittuf/internal/attestations/authorizations"
	"github.com/gittuf/gittuf/internal/attestations/github"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	ita "github.com/in-toto/attestation/go/v1"
	"github.com/stretchr/testify/assert"
)

const (
	targetRefKey      = "targetRef"
	fromRevisionIDKey = "fromRevisionID"
	targetTreeIDKey   = "targetTreeID"
)

func TestNewGitHubPullRequestApprovalAttestation(t *testing.T) {
	testRef := "refs/heads/main"
	testID := gitinterface.ZeroHash.String()

	approvers := []string{"jane.doe@example.com"}

	_, err := NewPullRequestApprovalAttestation(testRef, testID, testID, nil, nil)
	assert.ErrorIs(t, err, github.ErrInvalidPullRequestApprovalAttestation)

	approvalAttestation, err := NewPullRequestApprovalAttestation(testRef, testID, testID, approvers, nil)
	assert.Nil(t, err)

	// Check value of statement type
	assert.Equal(t, ita.StatementTypeUri, approvalAttestation.Type)

	// Check subject contents
	assert.Equal(t, 1, len(approvalAttestation.Subject))
	assert.Contains(t, approvalAttestation.Subject[0].Digest, digestGitTreeKey)
	assert.Equal(t, approvalAttestation.Subject[0].Digest[digestGitTreeKey], testID)

	// Check predicate type
	assert.Equal(t, PullRequestApprovalPredicateType, approvalAttestation.PredicateType)

	// Check predicate
	predicate := approvalAttestation.Predicate.AsMap()
	assert.Equal(t, predicate[targetRefKey], testRef)
	assert.Equal(t, predicate[targetTreeIDKey], testID)
	assert.Equal(t, predicate[fromRevisionIDKey], testID)
	// FIXME: this is a really messy assertion
	assert.Equal(t, approvers[0], predicate["approvers"].([]any)[0])
}

func TestValidatePullRequestApproval(t *testing.T) {
	testRef := "refs/heads/main"
	testAnotherRef := "refs/heads/feature"
	testID := gitinterface.ZeroHash.String()

	approvers := []string{"jane.doe@example.com"}

	mainZeroZero := createTestPullRequestApprovalEnvelope(t, testRef, testID, testID, approvers)
	featureZeroZero := createTestPullRequestApprovalEnvelope(t, testAnotherRef, testID, testID, approvers)

	err := ValidatePullRequestApproval(mainZeroZero, testRef, testID, testID)
	assert.Nil(t, err)

	err = ValidatePullRequestApproval(featureZeroZero, testAnotherRef, testID, testID)
	assert.Nil(t, err)

	err = ValidatePullRequestApproval(mainZeroZero, testAnotherRef, testID, testID)
	assert.ErrorIs(t, err, authorizations.ErrInvalidAuthorization)
}

func createTestPullRequestApprovalEnvelope(t *testing.T, refName, fromID, toID string, approvers []string) *sslibdsse.Envelope {
	t.Helper()

	authorization, err := NewPullRequestApprovalAttestation(refName, fromID, toID, approvers, nil)
	if err != nil {
		t.Fatal(err)
	}
	env, err := dsse.CreateEnvelope(authorization)
	if err != nil {
		t.Fatal(err)
	}

	return env
}
