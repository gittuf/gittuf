// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package attestations

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path"
	"testing"

	"github.com/gittuf/gittuf/internal/attestations/common"
	"github.com/gittuf/gittuf/internal/attestations/github"
	v01 "github.com/gittuf/gittuf/internal/attestations/github/v01"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	ita "github.com/in-toto/attestation/go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetGitHubPullRequestApprovalAttestation(t *testing.T) {
	testRef := "refs/heads/main"
	testAnotherRef := "refs/heads/feature"
	testID := gitinterface.ZeroHash.String()
	baseURL := "https://github.com"
	baseHost := "github.com"
	appName := "github"

	approvers := []string{"jane.doe@example.com"}

	mainZeroZero := createGitHubPullRequestApprovalAttestationEnvelope(t, testRef, testID, testID, approvers, nil)
	featureZeroZero := createGitHubPullRequestApprovalAttestationEnvelope(t, testAnotherRef, testID, testID, approvers, nil)

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

	approvers := []string{"jane.doe@example.com"}

	mainZeroZero := createGitHubPullRequestApprovalAttestationEnvelope(t, testRef, testID, testID, approvers, nil)
	featureZeroZero := createGitHubPullRequestApprovalAttestationEnvelope(t, testAnotherRef, testID, testID, approvers, nil)

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

func TestGitHubPullRequestApprovalAttestation_WithDismissedApprovers(t *testing.T) {
	// Setup: Define refs and commit hash IDs for the test
	testRef := "refs/heads/main"
	testID := gitinterface.ZeroHash.String()
	baseURL := "https://github.com"
	appName := "github"

	// Define test approvers and dismissed approvers
	approvers := []string{"alice@example.com", "bob@example.com"}
	dismissedApprovers := []string{"charlie@example.com"}

	// Create a pull request approval DSSE envelope with both approvers and dismissed approvers
	env := createGitHubPullRequestApprovalAttestationEnvelope(t, testRef, testID, testID, approvers, dismissedApprovers)

	// Create a temporary Git repository for the test
	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	// Create a new Attestations instance
	attestations := &Attestations{}

	// Set (store) the GitHub PR approval attestation in the Attestations object
	err := attestations.SetGitHubPullRequestApprovalAttestation(repo, env, baseURL, 1, appName, testRef, testID, testID)
	assert.Nil(t, err)

	// Retrieve the stored attestation
	retrieved, err := attestations.GetGitHubPullRequestApprovalAttestationFor(repo, appName, testRef, testID, testID)
	assert.Nil(t, err)

	// Extract the statement from the envelope
	var statement ita.Statement
	payloadBytes, err := base64.StdEncoding.DecodeString(retrieved.Payload)
	require.NoError(t, err)
	err = json.Unmarshal(payloadBytes, &statement)
	require.NoError(t, err)

	// Now unmarshal the Predicate from structpb.Struct into the custom Go struct
	var parsed v01.PullRequestApprovalAttestation
	err = common.UnmarshalPBStruct(statement.Predicate, &parsed)
	require.NoError(t, err)

	// Assertions
	assert.ElementsMatch(t, []string{"alice", "bob"}, stripEmail(parsed.GetApprovers()))
	assert.ElementsMatch(t, []string{"charlie"}, stripEmail(parsed.GetDismissedApprovers()))
}

// stripEmail returns the username part before '@' for each email in the slice.
func stripEmail(emails []string) []string {
	result := make([]string, len(emails))
	for i, email := range emails {
		at := 0
		for j, c := range email {
			if c == '@' {
				at = j
				break
			}
		}
		if at > 0 {
			result[i] = email[:at]
		} else {
			result[i] = email
		}
	}
	return result
}

/*
TestNewPullRequestApprovalAttestation_ErrorOnEmptyLists verifies that
NewGitHubPullRequestApprovalAttestation returns an error when both
approvers and dismissedApprovers are empty.
*/
func TestNewPullRequestApprovalAttestation_ErrorOnEmptyLists(t *testing.T) {
	ref := "refs/heads/main"
	fromID := gitinterface.ZeroHash.String()
	treeID := gitinterface.ZeroHash.String()

	// Create an attestation with no approvers or dismissed approvers
	attestation, err := NewGitHubPullRequestApprovalAttestation(ref, fromID, treeID, nil, nil)

	// The attestation should be nil and an error should be returned
	assert.Nil(t, attestation)
	assert.Error(t, err)
}

func TestGitHubPullRequestApproval_WithDismissedApprovers(t *testing.T) {
	// Set up fake refs and revision IDs
	testRef := "refs/heads/test-branch"
	testID := gitinterface.ZeroHash.String()

	// Define approvers and dismissed approvers
	approvers := []string{}
	dismissedApprovers := []string{"bob@example.com"}

	// Create the attestation envelope
	env := createGitHubPullRequestApprovalAttestationEnvelope(t, testRef, testID, testID, approvers, dismissedApprovers)

	// Initialize a temporary repo and attestations store
	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)
	attestations := &Attestations{}

	// Set the attestation for this PR
	err := attestations.SetGitHubPullRequestApprovalAttestation(repo, env, "https://github.com", 99, "github", testRef, testID, testID)
	assert.Nil(t, err)

	// Retrieve the same attestation and verify it matches
	retrieved, err := attestations.GetGitHubPullRequestApprovalAttestationFor(repo, "github", testRef, testID, testID)
	assert.Nil(t, err)
	assert.Equal(t, env, retrieved)
}

func TestGitHubPullRequestApproval_InvalidNoApprovers(t *testing.T) {
	// Both approvers and dismissedApprovers are empty
	refName := "refs/heads/test"
	fromID := gitinterface.ZeroHash.String()
	toID := gitinterface.ZeroHash.String()

	// Expect an error due to invalid input
	_, err := NewGitHubPullRequestApprovalAttestation(refName, fromID, toID, nil, nil)

	// Assert that error matches the expected invalid attestation error
	assert.ErrorIs(t, err, github.ErrInvalidPullRequestApprovalAttestation)
}

// TestGitHubPullRequestApproval_WithApproversAndDismissed tests that
// an attestation is successfully created when both approvers and
// dismissed approvers are provided.
func TestGitHubPullRequestApproval_WithApproversAndDismissed(t *testing.T) {
	ref := "refs/heads/main"
	fromID := gitinterface.ZeroHash.String()
	toID := gitinterface.ZeroHash.String()
	approvers := []string{"alice@example.com"}
	dismissedApprovers := []string{"bob@example.com"}

	// Create an attestation envelope with both approvers and dismissedApprovers
	env := createGitHubPullRequestApprovalAttestationEnvelope(t, ref, fromID, toID, approvers, dismissedApprovers)

	// Verify that the envelope and payload are valid
	assert.NotNil(t, env)
	assert.NotEmpty(t, env.Payload)
}

// createGitHubPullRequestApprovalAttestationEnvelope creates a GitHub Pull Request Approval attestation envelope
func createGitHubPullRequestApprovalAttestationEnvelope(t *testing.T, refName, fromID, toID string, approvers, dismissedApprovers []string) *sslibdsse.Envelope {
	t.Helper()
	// Adding dismissed approvers to the attestation
	authorization, err := NewGitHubPullRequestApprovalAttestation(refName, fromID, toID, approvers, dismissedApprovers)
	if err != nil {
		t.Fatal(err)
	}
	env, err := dsse.CreateEnvelope(authorization)
	if err != nil {
		t.Fatal(err)
	}

	return env
}
