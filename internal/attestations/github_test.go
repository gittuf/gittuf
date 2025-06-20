// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package attestations

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path"
	"testing"

	"github.com/gittuf/gittuf/internal/attestations/github"
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

	mainEnv := createGitHubPullRequestApprovalAttestationEnvelope(t, testRef, testID, testID, approvers, nil)
	featureEnv := createGitHubPullRequestApprovalAttestationEnvelope(t, testAnotherRef, testID, testID, approvers, nil)

	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)
	attestations := &Attestations{}

	err := attestations.SetGitHubPullRequestApprovalAttestation(repo, mainEnv, baseURL, 1, appName, testRef, testID, testID)
	assert.NoError(t, err)
	assert.Contains(t, attestations.codeReviewApprovalAttestations,
		path.Join(GitHubPullRequestApprovalAttestationPath(testRef, testID, testID),
			base64.URLEncoding.EncodeToString([]byte(appName))))
	assert.NotContains(t, attestations.codeReviewApprovalAttestations,
		path.Join(GitHubPullRequestApprovalAttestationPath(testAnotherRef, testID, testID),
			base64.URLEncoding.EncodeToString([]byte(appName))))
	assert.Equal(t, GitHubPullRequestApprovalAttestationPath(testRef, testID, testID),
		attestations.codeReviewApprovalIndex[fmt.Sprintf("%s::%d", baseHost, 1)])

	err = attestations.SetGitHubPullRequestApprovalAttestation(repo, featureEnv, baseURL, 2, appName, testAnotherRef, testID, testID)
	assert.NoError(t, err)
	assert.Contains(t, attestations.codeReviewApprovalAttestations,
		path.Join(GitHubPullRequestApprovalAttestationPath(testRef, testID, testID),
			base64.URLEncoding.EncodeToString([]byte(appName))))
	assert.Contains(t, attestations.codeReviewApprovalAttestations,
		path.Join(GitHubPullRequestApprovalAttestationPath(testAnotherRef, testID, testID),
			base64.URLEncoding.EncodeToString([]byte(appName))))
	assert.Equal(t, GitHubPullRequestApprovalAttestationPath(testRef, testID, testID),
		attestations.codeReviewApprovalIndex[fmt.Sprintf("%s::%d", baseHost, 1)])
	assert.Equal(t, GitHubPullRequestApprovalAttestationPath(testAnotherRef, testID, testID),
		attestations.codeReviewApprovalIndex[fmt.Sprintf("%s::%d", baseHost, 2)])
}

func TestGetGitHubPullRequestApprovalAttestationFor(t *testing.T) {
	testRef := "refs/heads/main"
	testAnotherRef := "refs/heads/feature"
	testID := gitinterface.ZeroHash.String()
	baseURL := "https://github.com"
	appName := "github"
	approvers := []string{"jane.doe@example.com"}

	mainEnv := createGitHubPullRequestApprovalAttestationEnvelope(t, testRef, testID, testID, approvers, nil)
	featureEnv := createGitHubPullRequestApprovalAttestationEnvelope(t, testAnotherRef, testID, testID, approvers, nil)

	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)
	attestations := &Attestations{}

	require.Nil(t, attestations.SetGitHubPullRequestApprovalAttestation(repo, mainEnv, baseURL, 1, appName, testRef, testID, testID))
	require.Nil(t, attestations.SetGitHubPullRequestApprovalAttestation(repo, featureEnv, baseURL, 2, appName, testAnotherRef, testID, testID))

	mainGot, err := attestations.GetGitHubPullRequestApprovalAttestationFor(repo, appName, testRef, testID, testID)
	assert.NoError(t, err)
	assert.Equal(t, mainEnv, mainGot)

	featureGot, err := attestations.GetGitHubPullRequestApprovalAttestationFor(repo, appName, testAnotherRef, testID, testID)
	assert.NoError(t, err)
	assert.Equal(t, featureEnv, featureGot)
}

func TestNewGitHubPullRequestApprovalAttestation(t *testing.T) {
	ref := "refs/heads/main"
	fromID := gitinterface.ZeroHash.String()
	toID := gitinterface.ZeroHash.String()

	t.Run("error when approvers and dismissedApprovers empty", func(t *testing.T) {
		att, err := NewGitHubPullRequestApprovalAttestation(ref, fromID, toID, nil, nil)
		assert.Nil(t, att)
		assert.ErrorIs(t, err, github.ErrInvalidPullRequestApprovalAttestation)
	})

	t.Run("success with approvers only", func(t *testing.T) {
		approvers := []string{"alice@example.com"}
		att, err := NewGitHubPullRequestApprovalAttestation(ref, fromID, toID, approvers, nil)
		assert.NoError(t, err)
		assert.NotNil(t, att)
	})

	t.Run("success with dismissedApprovers only", func(t *testing.T) {
		dismissed := []string{"bob@example.com"}
		att, err := NewGitHubPullRequestApprovalAttestation(ref, fromID, toID, nil, dismissed)
		assert.NoError(t, err)
		assert.NotNil(t, att)
	})

	t.Run("success with both approvers and dismissedApprovers", func(t *testing.T) {
		approvers := []string{"alice@example.com"}
		dismissed := []string{"bob@example.com"}
		att, err := NewGitHubPullRequestApprovalAttestation(ref, fromID, toID, approvers, dismissed)
		assert.NoError(t, err)
		assert.NotNil(t, att)
	})
}

func TestGitHubPullRequestApprovalAttestation_WithDismissedApprovers(t *testing.T) {
	testRef := "refs/heads/main"
	testID := gitinterface.ZeroHash.String()
	baseURL := "https://github.com"
	appName := "github"

	approvers := []string{"alice@example.com", "bob@example.com"}
	dismissed := []string{"charlie@example.com"}

	env := createGitHubPullRequestApprovalAttestationEnvelope(t, testRef, testID, testID, approvers, dismissed)

	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)
	attestations := &Attestations{}

	require.Nil(t, attestations.SetGitHubPullRequestApprovalAttestation(repo, env, baseURL, 1, appName, testRef, testID, testID))

	got, err := attestations.GetGitHubPullRequestApprovalAttestationFor(repo, appName, testRef, testID, testID)
	assert.NoError(t, err)

	payloadBytes, err := base64.StdEncoding.DecodeString(got.Payload)
	require.Nil(t, err)

	var statement ita.Statement
	require.Nil(t, json.Unmarshal(payloadBytes, &statement))

	predicate := statement.Predicate.AsMap()

	// Extract and assert approvers
	approverVals, ok := predicate["approvers"].([]interface{})
	require.True(t, ok, "approvers should be a list")

	gotApprovers := make([]string, len(approverVals))
	for i, a := range approverVals {
		gotApprovers[i] = a.(string)
	}
	expectedApprovers := []string{"alice@example.com", "bob@example.com"}
	assert.Equal(t, expectedApprovers, gotApprovers)

	// Extract and assert dismissedApprovers
	dismissedVals, ok := predicate["dismissedApprovers"].([]interface{})
	require.True(t, ok, "dismissedApprovers should be a list")

	gotDismissed := make([]string, len(dismissedVals))
	for i, d := range dismissedVals {
		gotDismissed[i] = d.(string)
	}
	expectedDismissed := []string{"charlie@example.com"}
	assert.Equal(t, expectedDismissed, gotDismissed)
}

func createGitHubPullRequestApprovalAttestationEnvelope(t *testing.T, refName, fromID, toID string, approvers, dismissedApprovers []string) *sslibdsse.Envelope {
	t.Helper()

	att, err := NewGitHubPullRequestApprovalAttestation(refName, fromID, toID, approvers, dismissedApprovers)
	if err != nil {
		t.Fatal(err)
	}
	env, err := dsse.CreateEnvelope(att)
	if err != nil {
		t.Fatal(err)
	}
	return env
}
