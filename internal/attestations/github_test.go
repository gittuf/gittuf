// SPDX-License-Identifier: Apache-2.0

package attestations

import (
	"testing"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/signerverifier"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	sslibsv "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/signerverifier"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/go-git/go-git/v5/plumbing"
	ita "github.com/in-toto/attestation/go/v1"
	sslibdsse "github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/stretchr/testify/assert"
)

func TestNewGitHubPullRequestApprovalAttestation(t *testing.T) {
	testRef := "refs/heads/main"
	testID := plumbing.ZeroHash.String()

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

	approvalAttestation, err := NewGitHubPullRequestApprovalAttestation(testRef, testID, testID, approvers)
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

func TestSetGitHubPullRequestApprovalAttestation(t *testing.T) {
	testRef := "refs/heads/main"
	testAnotherRef := "refs/heads/feature"
	testID := plumbing.ZeroHash.String()

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

	mainZeroZero := createGitHubPullRequestApprovalAttestationEnvelope(t, testRef, testID, testID, approvers)
	featureZeroZero := createGitHubPullRequestApprovalAttestationEnvelope(t, testAnotherRef, testID, testID, approvers)

	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	attestations := &Attestations{}

	// Add auth for first branch
	err := attestations.SetGitHubPullRequestApprovalAttestation(repo, mainZeroZero, testRef, testID, testID)
	assert.Nil(t, err)
	assert.Contains(t, attestations.githubPullRequestApprovalAttestations, GitHubPullRequestApprovalAttestationPath(testRef, testID, testID))
	assert.NotContains(t, attestations.githubPullRequestApprovalAttestations, GitHubPullRequestApprovalAttestationPath(testAnotherRef, testID, testID))

	// Add auth for the other branch
	err = attestations.SetGitHubPullRequestApprovalAttestation(repo, featureZeroZero, testAnotherRef, testID, testID)
	assert.Nil(t, err)
	assert.Contains(t, attestations.githubPullRequestApprovalAttestations, GitHubPullRequestApprovalAttestationPath(testRef, testID, testID))
	assert.Contains(t, attestations.githubPullRequestApprovalAttestations, GitHubPullRequestApprovalAttestationPath(testAnotherRef, testID, testID))
}

func TestGetGitHubPullRequestApprovalAttestation(t *testing.T) {
	testRef := "refs/heads/main"
	testAnotherRef := "refs/heads/feature"
	testID := plumbing.ZeroHash.String()

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

	mainZeroZero := createGitHubPullRequestApprovalAttestationEnvelope(t, testRef, testID, testID, approvers)
	featureZeroZero := createGitHubPullRequestApprovalAttestationEnvelope(t, testAnotherRef, testID, testID, approvers)

	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	attestations := &Attestations{}

	err := attestations.SetGitHubPullRequestApprovalAttestation(repo, mainZeroZero, testRef, testID, testID)
	if err != nil {
		t.Fatal(err)
	}
	err = attestations.SetGitHubPullRequestApprovalAttestation(repo, featureZeroZero, testAnotherRef, testID, testID)
	if err != nil {
		t.Fatal(err)
	}

	mainAuth, err := attestations.GetGitHubPullRequestApprovalAttestationFor(repo, testRef, testID, testID)
	assert.Nil(t, err)
	assert.Equal(t, mainZeroZero, mainAuth)

	featureAuth, err := attestations.GetGitHubPullRequestApprovalAttestationFor(repo, testAnotherRef, testID, testID)
	assert.Nil(t, err)
	assert.Equal(t, featureZeroZero, featureAuth)
}

func TestValidateGitHubPullRequestApprovalAttestation(t *testing.T) {
	testRef := "refs/heads/main"
	testAnotherRef := "refs/heads/feature"
	testID := plumbing.ZeroHash.String()

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

	mainZeroZero := createGitHubPullRequestApprovalAttestationEnvelope(t, testRef, testID, testID, approvers)
	featureZeroZero := createGitHubPullRequestApprovalAttestationEnvelope(t, testAnotherRef, testID, testID, approvers)

	err := validateGitHubPullRequestApprovalAttestation(mainZeroZero, testRef, testID, testID)
	assert.Nil(t, err)

	err = validateGitHubPullRequestApprovalAttestation(featureZeroZero, testAnotherRef, testID, testID)
	assert.Nil(t, err)

	err = validateGitHubPullRequestApprovalAttestation(mainZeroZero, testAnotherRef, testID, testID)
	assert.ErrorIs(t, err, ErrInvalidAuthorization)
}

func createGitHubPullRequestApprovalAttestationEnvelope(t *testing.T, refName, fromID, toID string, approvers []*tuf.Key) *sslibdsse.Envelope {
	t.Helper()

	authorization, err := NewGitHubPullRequestApprovalAttestation(refName, fromID, toID, approvers)
	if err != nil {
		t.Fatal(err)
	}
	env, err := dsse.CreateEnvelope(authorization)
	if err != nil {
		t.Fatal(err)
	}

	return env
}
