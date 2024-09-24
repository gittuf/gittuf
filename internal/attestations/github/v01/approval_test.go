// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v01

import (
	"testing"

	authorizationsv01 "github.com/gittuf/gittuf/internal/attestations/authorizations/v01"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/signerverifier"
	sslibsv "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/signerverifier"
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

	approvers := []*sslibsv.SSLibKey{
		{
			KeyID:   "jane.doe@example.com::https://oidc.example.com",
			KeyType: signerverifier.FulcioKeyType,
			Scheme:  signerverifier.FulcioKeyScheme,
			KeyVal: sslibsv.KeyVal{
				Identity: "jane.doe@example.com",
				Issuer:   "https://oidc.example.com",
			},
		},
	}

	_, err := NewGitHubPullRequestApprovalAttestation(testRef, testID, testID, nil, nil)
	assert.ErrorIs(t, err, ErrInvalidGitHubPullRequestApprovalAttestation)

	approvalAttestation, err := NewGitHubPullRequestApprovalAttestation(testRef, testID, testID, approvers, nil)
	assert.Nil(t, err)

	// Check value of statement type
	assert.Equal(t, ita.StatementTypeUri, approvalAttestation.Type)

	// Check subject contents
	assert.Equal(t, 1, len(approvalAttestation.Subject))
	assert.Contains(t, approvalAttestation.Subject[0].Digest, digestGitTreeKey)
	assert.Equal(t, approvalAttestation.Subject[0].Digest[digestGitTreeKey], testID)

	// Check predicate type
	assert.Equal(t, GitHubPullRequestApprovalPredicateType, approvalAttestation.PredicateType)

	// Check predicate
	predicate := approvalAttestation.Predicate.AsMap()
	assert.Equal(t, predicate[targetRefKey], testRef)
	assert.Equal(t, predicate[targetTreeIDKey], testID)
	assert.Equal(t, predicate[fromRevisionIDKey], testID)
	// FIXME: this is a really messy assertion
	assert.Equal(t, approvers[0].KeyID, predicate["approvers"].([]any)[0].(map[string]any)["keyid"])
}

func TestValidatePullRequestApproval(t *testing.T) {
	testRef := "refs/heads/main"
	testAnotherRef := "refs/heads/feature"
	testID := gitinterface.ZeroHash.String()

	approvers := []*sslibsv.SSLibKey{
		{
			KeyID:   "jane.doe@example.com::https://oidc.example.com",
			KeyType: signerverifier.FulcioKeyType,
			Scheme:  signerverifier.FulcioKeyScheme,
			KeyVal: sslibsv.KeyVal{
				Identity: "jane.doe@example.com",
				Issuer:   "https://oidc.example.com",
			},
		},
	}

	mainZeroZero := CreateTestPullRequestApprovalEnvelope(t, testRef, testID, testID, approvers)
	featureZeroZero := CreateTestPullRequestApprovalEnvelope(t, testAnotherRef, testID, testID, approvers)

	err := ValidatePullRequestApproval(mainZeroZero, testRef, testID, testID)
	assert.Nil(t, err)

	err = ValidatePullRequestApproval(featureZeroZero, testAnotherRef, testID, testID)
	assert.Nil(t, err)

	err = ValidatePullRequestApproval(mainZeroZero, testAnotherRef, testID, testID)
	assert.ErrorIs(t, err, authorizationsv01.ErrInvalidAuthorization)
}
