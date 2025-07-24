// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package attestations

import (
	"encoding/base64"
	"fmt"
	"path"
	"testing"

	"github.com/gittuf/gittuf/internal/attestations/github"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
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

	t.Run("normal case", func(t *testing.T) {
		approvers := []string{"jane.doe@example.com"}

		mainZeroZero := createGitHubPullRequestApprovalAttestationEnvelope(t, testRef, testID, testID, approvers)
		featureZeroZero := createGitHubPullRequestApprovalAttestationEnvelope(t, testAnotherRef, testID, testID, approvers)

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
	})

	t.Run("validation error", func(t *testing.T) {
		// Create an invalid envelope (empty envelope)
		invalidEnv := &sslibdsse.Envelope{}

		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		attestations := &Attestations{}

		// This should fail validation
		err := attestations.SetGitHubPullRequestApprovalAttestation(repo, invalidEnv, baseURL, 1, appName, testRef, testID, testID)
		assert.NotNil(t, err)
		assert.ErrorIs(t, err, github.ErrInvalidPullRequestApprovalAttestation)
	})
}

func TestGetGitHubPullRequestApprovalAttestation(t *testing.T) {
	testRef := "refs/heads/main"
	testAnotherRef := "refs/heads/feature"
	testID := gitinterface.ZeroHash.String()
	baseURL := "https://github.com"
	appName := "github"

	t.Run("normal case", func(t *testing.T) {
		approvers := []string{"jane.doe@example.com"}

		mainZeroZero := createGitHubPullRequestApprovalAttestationEnvelope(t, testRef, testID, testID, approvers)
		featureZeroZero := createGitHubPullRequestApprovalAttestationEnvelope(t, testAnotherRef, testID, testID, approvers)

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
	})

	t.Run("conflicting index path", func(t *testing.T) {
		approvers := []string{"jane.doe@example.com"}

		mainZeroZero := createGitHubPullRequestApprovalAttestationEnvelope(t, testRef, testID, testID, approvers)
		featureZeroZero := createGitHubPullRequestApprovalAttestationEnvelope(t, testAnotherRef, testID, testID, approvers)

		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		attestations := &Attestations{}

		// Add first attestation
		err := attestations.SetGitHubPullRequestApprovalAttestation(repo, mainZeroZero, baseURL, 1, appName, testRef, testID, testID)
		require.Nil(t, err)

		// Try to add another attestation with same review ID but different index path
		// This should fail because the same review ID is being used for different index paths
		err = attestations.SetGitHubPullRequestApprovalAttestation(repo, featureZeroZero, baseURL, 1, appName, testAnotherRef, testID, testID)
		assert.NotNil(t, err)
		assert.ErrorIs(t, err, github.ErrInvalidPullRequestApprovalAttestation)

		// Confirm the state of attestations
		mainAuthApp, err := attestations.GetGitHubPullRequestApprovalAttestationFor(repo, appName, testRef, testID, testID)
		assert.Nil(t, err)
		assert.Equal(t, mainZeroZero, mainAuthApp)
	})

	t.Run("same review ID, same index path", func(t *testing.T) {
		anotherAppName := "another-app"
		approvers := []string{"jane.doe@example.com"}

		mainZeroZero := createGitHubPullRequestApprovalAttestationEnvelope(t, testRef, testID, testID, approvers)

		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		attestations := &Attestations{}

		// Add first attestation
		err := attestations.SetGitHubPullRequestApprovalAttestation(repo, mainZeroZero, baseURL, 1, appName, testRef, testID, testID)
		require.Nil(t, err)

		// Add another attestation with same review ID and same index path but different app
		// This should succeed because the same review ID can be observed by more than one app
		err = attestations.SetGitHubPullRequestApprovalAttestation(repo, mainZeroZero, baseURL, 1, anotherAppName, testRef, testID, testID)
		assert.Nil(t, err)

		// Confirm the state of attestations
		mainAuthApp, err := attestations.GetGitHubPullRequestApprovalAttestationFor(repo, appName, testRef, testID, testID)
		assert.Nil(t, err)
		assert.Equal(t, mainZeroZero, mainAuthApp)

		mainAuthAnother, err := attestations.GetGitHubPullRequestApprovalAttestationFor(repo, anotherAppName, testRef, testID, testID)
		assert.Nil(t, err)
		assert.Equal(t, mainZeroZero, mainAuthAnother)
	})
}

func TestGitHubReviewID(t *testing.T) {
	reviewID, err := GitHubReviewID("https://github.com", 123)
	assert.Nil(t, err)
	assert.Equal(t, "github.com::123", reviewID)
}

func TestGetGitHubPullRequestApprovalAttestationForReviewID(t *testing.T) {
	testRef := "refs/heads/main"
	testID := gitinterface.ZeroHash.String()
	baseURL := "https://github.com"
	appName := "github"

	t.Run("success", func(t *testing.T) {
		reviewID := int64(123)

		approvers := []string{"jane.doe@example.com"}

		attestationEnv := createGitHubPullRequestApprovalAttestationEnvelope(t, testRef, testID, testID, approvers)

		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		attestations := &Attestations{}

		// Set the attestation
		err := attestations.SetGitHubPullRequestApprovalAttestation(repo, attestationEnv, baseURL, reviewID, appName, testRef, testID, testID)
		require.Nil(t, err)

		// Get the attestation by review ID
		retrievedEnv, err := attestations.GetGitHubPullRequestApprovalAttestationForReviewID(repo, baseURL, reviewID, appName)
		assert.Nil(t, err)
		assert.Equal(t, attestationEnv, retrievedEnv)
	})

	t.Run("not found", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		attestations := &Attestations{}

		// Try to get attestation for non-existent review ID
		_, err := attestations.GetGitHubPullRequestApprovalAttestationForReviewID(repo, "https://github.com", 999, "github")
		assert.NotNil(t, err)
		assert.ErrorIs(t, err, github.ErrGitHubReviewIDNotFound)
	})

	t.Run("invalid URL", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		attestations := &Attestations{}

		// Try to get attestation with invalid URL
		_, err := attestations.GetGitHubPullRequestApprovalAttestationForReviewID(repo, "invalid-url", 123, "github")
		assert.ErrorIs(t, err, github.ErrGitHubReviewIDNotFound)
	})
}

func TestGetGitHubPullRequestApprovalAttestationForIndexPath(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		attestations := &Attestations{}

		// Try to get attestation for non-existent path
		_, err := attestations.GetGitHubPullRequestApprovalAttestationForIndexPath(repo, "github", "non/existent/path")
		assert.NotNil(t, err)
		assert.ErrorIs(t, err, github.ErrPullRequestApprovalAttestationNotFound)
	})
}

func TestGetGitHubPullRequestApprovalIndexPathForReviewID(t *testing.T) {
	testRef := "refs/heads/main"
	testID := gitinterface.ZeroHash.String()
	baseURL := "https://github.com"
	appName := "github"
	reviewID := int64(123)

	t.Run("success", func(t *testing.T) {
		approvers := []string{"jane.doe@example.com"}

		attestationEnv := createGitHubPullRequestApprovalAttestationEnvelope(t, testRef, testID, testID, approvers)

		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		attestations := &Attestations{}

		// Set the attestation
		err := attestations.SetGitHubPullRequestApprovalAttestation(repo, attestationEnv, baseURL, reviewID, appName, testRef, testID, testID)
		require.Nil(t, err)

		// Get the index path by review ID
		indexPath, has, err := attestations.GetGitHubPullRequestApprovalIndexPathForReviewID(baseURL, reviewID)
		assert.Nil(t, err)
		assert.True(t, has)
		assert.Equal(t, GitHubPullRequestApprovalAttestationPath(testRef, testID, testID), indexPath)
	})

	t.Run("not found", func(t *testing.T) {
		attestations := &Attestations{}

		// Try to get index path for non-existent review ID
		indexPath, has, err := attestations.GetGitHubPullRequestApprovalIndexPathForReviewID("https://github.com", 999)
		assert.Nil(t, err)
		assert.False(t, has)
		assert.Equal(t, "", indexPath)
	})
}

func TestSetGitHubPullRequestAuthorization(t *testing.T) {
	testRef := "refs/heads/main"
	testID := gitinterface.ZeroHash.String()

	// Create a simple envelope for testing using the same approach as existing tests
	approvers := []string{"jane.doe@example.com"}
	env := createGitHubPullRequestApprovalAttestationEnvelope(t, testRef, testID, testID, approvers)

	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	attestations := &Attestations{}

	err := attestations.SetGitHubPullRequestAuthorization(repo, env, testRef, testID)
	assert.Nil(t, err)

	expectedPath := GitHubPullRequestAttestationPath(testRef, testID)
	assert.Contains(t, attestations.githubPullRequestAttestations, expectedPath)
}

func TestGitHubPullRequestAttestationPath(t *testing.T) {
	refName := "refs/heads/main"
	commitID := "abc123"

	expectedPath := path.Join(refName, commitID)
	actualPath := GitHubPullRequestAttestationPath(refName, commitID)

	assert.Equal(t, expectedPath, actualPath)
}

func TestGitHubPullRequestApprovalAttestationPath(t *testing.T) {
	refName := "refs/heads/main"
	fromID := "abc123"
	toID := "def456"

	expectedPath := path.Join(ReferenceAuthorizationPath(refName, fromID, toID), githubPullRequestApprovalSystemName)
	actualPath := GitHubPullRequestApprovalAttestationPath(refName, fromID, toID)

	assert.Equal(t, expectedPath, actualPath)
}

func createGitHubPullRequestApprovalAttestationEnvelope(t *testing.T, refName, fromID, toID string, approvers []string) *sslibdsse.Envelope {
	t.Helper()

	authorization, err := NewGitHubPullRequestApprovalAttestation(refName, fromID, toID, approvers, nil)
	if err != nil {
		t.Fatal(err)
	}
	env, err := dsse.CreateEnvelope(authorization)
	if err != nil {
		t.Fatal(err)
	}

	return env
}
