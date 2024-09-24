// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package attestations

import (
	"encoding/base64"
	"fmt"
	"path"
	"testing"

	githubv01 "github.com/gittuf/gittuf/internal/attestations/github/v01"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/signerverifier"
	sslibsv "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/signerverifier"
	ita "github.com/in-toto/attestation/go/v1"
	"github.com/stretchr/testify/assert"
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

	approvalAttestation, err := NewGitHubPullRequestApprovalAttestation(testRef, testID, testID, approvers, nil)
	assert.Nil(t, err)

	// Check value of statement type
	assert.Equal(t, ita.StatementTypeUri, approvalAttestation.Type)

	// Check subject contents
	assert.Equal(t, 1, len(approvalAttestation.Subject))

	// Check predicate type
	assert.Equal(t, githubv01.GitHubPullRequestApprovalPredicateType, approvalAttestation.PredicateType)
}

func TestSetGitHubPullRequestApprovalAttestation(t *testing.T) {
	testRef := "refs/heads/main"
	testAnotherRef := "refs/heads/feature"
	testID := gitinterface.ZeroHash.String()
	baseURL := "https://github.com"
	baseHost := "github.com"
	appName := "github"

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

	mainZeroZero := githubv01.CreateTestPullRequestApprovalEnvelope(t, testRef, testID, testID, approvers)
	featureZeroZero := githubv01.CreateTestPullRequestApprovalEnvelope(t, testAnotherRef, testID, testID, approvers)

	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	attestations := &Attestations{}

	// Add auth for first branch
	err := attestations.SetGitHubPullRequestApprovalAttestation(repo, mainZeroZero, baseURL, 1, appName, testRef, testID, testID)
	assert.Nil(t, err)
	assert.Contains(t, attestations.codeReviewApprovalAttestations, path.Join(GitHubPullRequestApprovalAttestationPath(testRef, testID, testID), base64.URLEncoding.EncodeToString([]byte(appName))))
	assert.NotContains(t, attestations.codeReviewApprovalAttestations, path.Join(GitHubPullRequestApprovalAttestationPath(testAnotherRef, testID, testID), base64.URLEncoding.EncodeToString([]byte(appName))))
	assert.Equal(t, GitHubPullRequestApprovalAttestationPath(testRef, testID, testID), attestations.codeReviewApprovalIndex[fmt.Sprintf("%s::%d", baseHost, 1)])

	// Add auth for the other branch
	err = attestations.SetGitHubPullRequestApprovalAttestation(repo, featureZeroZero, baseURL, 2, appName, testAnotherRef, testID, testID)
	assert.Nil(t, err)
	assert.Contains(t, attestations.codeReviewApprovalAttestations, path.Join(GitHubPullRequestApprovalAttestationPath(testRef, testID, testID), base64.URLEncoding.EncodeToString([]byte(appName))))
	assert.Contains(t, attestations.codeReviewApprovalAttestations, path.Join(GitHubPullRequestApprovalAttestationPath(testAnotherRef, testID, testID), base64.URLEncoding.EncodeToString([]byte(appName))))
	assert.Equal(t, GitHubPullRequestApprovalAttestationPath(testRef, testID, testID), attestations.codeReviewApprovalIndex[fmt.Sprintf("%s::%d", baseHost, 1)])
	assert.Equal(t, GitHubPullRequestApprovalAttestationPath(testAnotherRef, testID, testID), attestations.codeReviewApprovalIndex[fmt.Sprintf("%s::%d", baseHost, 2)])
}

func TestGetGitHubPullRequestApprovalAttestation(t *testing.T) {
	testRef := "refs/heads/main"
	testAnotherRef := "refs/heads/feature"
	testID := gitinterface.ZeroHash.String()
	baseURL := "https://github.com"
	appName := "github"

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

	mainZeroZero := githubv01.CreateTestPullRequestApprovalEnvelope(t, testRef, testID, testID, approvers)
	featureZeroZero := githubv01.CreateTestPullRequestApprovalEnvelope(t, testAnotherRef, testID, testID, approvers)

	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	attestations := &Attestations{}

	err := attestations.SetGitHubPullRequestApprovalAttestation(repo, mainZeroZero, baseURL, 1, appName, testRef, testID, testID)
	if err != nil {
		t.Fatal(err)
	}
	err = attestations.SetGitHubPullRequestApprovalAttestation(repo, featureZeroZero, baseURL, 2, appName, testAnotherRef, testID, testID)
	if err != nil {
		t.Fatal(err)
	}

	mainAuth, err := attestations.GetGitHubPullRequestApprovalAttestationFor(repo, appName, testRef, testID, testID)
	assert.Nil(t, err)
	assert.Equal(t, mainZeroZero, mainAuth)

	featureAuth, err := attestations.GetGitHubPullRequestApprovalAttestationFor(repo, appName, testAnotherRef, testID, testID)
	assert.Nil(t, err)
	assert.Equal(t, featureZeroZero, featureAuth)
}
