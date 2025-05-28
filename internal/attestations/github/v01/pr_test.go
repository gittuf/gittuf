// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v01

import (
	"fmt"
	"testing"
	"time"

	gogithub "github.com/google/go-github/v61/github"
	ita "github.com/in-toto/attestation/go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPullRequestAttestation(t *testing.T) {
	// Test setup with sample data
	testOwner := "example"
	testRepo := "example"
	testPRNumber := 123
	testCommitID := "a1b2c3d4e5f6"
	// Create test date for PR timestamps
	testTime := time.Date(2023, 5, 10, 15, 30, 0, 0, time.UTC)

	// Define test cases
	tests := []struct {
		name        string
		pr          *gogithub.PullRequest
		expectError bool
	}{
		{
			name: "successful attestation creation",
			pr: &gogithub.PullRequest{
				Number:    gogithub.Int(testPRNumber),
				Title:     gogithub.String("Test PR"),
				Body:      gogithub.String("This is a test PR"),
				State:     gogithub.String("open"),
				HTMLURL:   gogithub.String(fmt.Sprintf("https://github.com/%s/%s/pull/%d", testOwner, testRepo, testPRNumber)),
				CreatedAt: &gogithub.Timestamp{Time: testTime},
				UpdatedAt: &gogithub.Timestamp{Time: testTime},
				User:      &gogithub.User{Login: gogithub.String("Jane Doe"), ID: gogithub.Int64(12345)},
				Merged:    gogithub.Bool(false),
				Mergeable: gogithub.Bool(true),
				Base: &gogithub.PullRequestBranch{
					Ref:  gogithub.String("main"),
					SHA:  gogithub.String("base-sha-123"),
					Repo: &gogithub.Repository{Name: gogithub.String(testRepo), Owner: &gogithub.User{Login: gogithub.String(testOwner)}},
				},
				Head: &gogithub.PullRequestBranch{
					Ref:  gogithub.String("feature-branch"),
					SHA:  gogithub.String(testCommitID),
					Repo: &gogithub.Repository{Name: gogithub.String(testRepo), Owner: &gogithub.User{Login: gogithub.String(testOwner)}},
				},
			},
			expectError: false,
		},
	}

	// Run all test cases
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Create attestation using the function under test
			attestation, err := NewPullRequestAttestation(testOwner, testRepo, testPRNumber, testCommitID, test.pr)

			// Verify attestation was created without errors
			assert.Nil(t, err)
			assert.NotNil(t, attestation)

			// Verify the statement type is correct
			assert.Equal(t, ita.StatementTypeUri, attestation.Type)

			// Verify subject contains the PR URL and commit ID
			require.Len(t, attestation.Subject, 1)
			assert.Equal(t, fmt.Sprintf("https://github.com/%s/%s/pull/%d", testOwner, testRepo, testPRNumber), attestation.Subject[0].Uri)
			assert.Equal(t, testCommitID, attestation.Subject[0].Digest[digestGitCommitKey])

			// Verify predicate type is correct
			assert.Equal(t, PullRequestPredicateType, attestation.PredicateType)

			// Additional tests for non-nil PR data
			if test.pr != nil {
				// Extract and validate predicate fields
				predFields := attestation.Predicate.AsMap()

				// Verify basic PR fields
				assert.Equal(t, float64(testPRNumber), predFields["number"])
				assert.Equal(t, "Test PR", predFields["title"])
				assert.Equal(t, "open", predFields["state"])
				assert.Equal(t, fmt.Sprintf("https://github.com/%s/%s/pull/%d", testOwner, testRepo, testPRNumber), predFields["html_url"])

				// Verify head branch information
				headMap := predFields["head"].(map[string]interface{})
				assert.Equal(t, "feature-branch", headMap["ref"])
				assert.Equal(t, testCommitID, headMap["sha"])

				// Verify base branch information
				baseMap := predFields["base"].(map[string]interface{})
				assert.Equal(t, "main", baseMap["ref"])
				assert.Equal(t, "base-sha-123", baseMap["sha"])
			}
		})
	}

	// Parameter handling tests
	t.Run("parameter handling", func(t *testing.T) {
		// Test setup with sample data
		tests := []struct {
			name     string
			owner    string
			repo     string
			prNumber int
			commitID string
			pr       *gogithub.PullRequest
			expected struct {
				uri    string
				digest string
			}
		}{
			{
				name:     "all parameters valid",
				owner:    "example",
				repo:     "example",
				prNumber: 123,
				commitID: "abcdef123456",
				pr:       &gogithub.PullRequest{Number: gogithub.Int(123)},
				expected: struct {
					uri    string
					digest string
				}{
					uri:    "https://github.com/example/example/pull/123",
					digest: "abcdef123456",
				},
			},
			{
				name:     "empty owner",
				owner:    "",
				repo:     "example",
				prNumber: 123,
				commitID: "abcdef123456",
				pr:       &gogithub.PullRequest{},
				expected: struct {
					uri    string
					digest string
				}{
					uri:    "https://github.com//example/pull/123",
					digest: "abcdef123456",
				},
			},
			{
				name:     "empty repo",
				owner:    "example",
				repo:     "",
				prNumber: 123,
				commitID: "abcdef123456",
				pr:       &gogithub.PullRequest{},
				expected: struct {
					uri    string
					digest string
				}{
					uri:    "https://github.com/example//pull/123",
					digest: "abcdef123456",
				},
			},
			{
				name:     "empty commit ID",
				owner:    "example",
				repo:     "example",
				prNumber: 123,
				commitID: "",
				pr:       &gogithub.PullRequest{},
				expected: struct {
					uri    string
					digest string
				}{
					uri:    "https://github.com/example/example/pull/123",
					digest: "",
				},
			},
			{
				name:     "zero PR number",
				owner:    "example",
				repo:     "example",
				prNumber: 0,
				commitID: "abcdef123456",
				pr:       &gogithub.PullRequest{},
				expected: struct {
					uri    string
					digest string
				}{
					uri:    "https://github.com/example/example/pull/0",
					digest: "abcdef123456",
				},
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				attestation, err := NewPullRequestAttestation(test.owner, test.repo, test.prNumber, test.commitID, test.pr)

				assert.Nil(t, err)
				assert.NotNil(t, attestation)

				// Verify subject contains expected values
				require.Len(t, attestation.Subject, 1)
				assert.Equal(t, test.expected.uri, attestation.Subject[0].Uri)
				assert.Equal(t, test.expected.digest, attestation.Subject[0].Digest[digestGitCommitKey])
			})
		}
	})

	// Edge cases tests
	t.Run("edge cases", func(t *testing.T) {
		// Test how the function handles special PR structures
		tests := []struct {
			name string
			pr   *gogithub.PullRequest
		}{
			{
				name: "PR with nil fields",
				pr: &gogithub.PullRequest{
					// Minimal structure with nil fields
					Number: gogithub.Int(123),
					// All other fields nil
				},
			},
			{
				name: "PR with empty string fields",
				pr: &gogithub.PullRequest{
					Number:  gogithub.Int(123),
					Title:   gogithub.String(""),
					Body:    gogithub.String(""),
					State:   gogithub.String(""),
					HTMLURL: gogithub.String(""),
				},
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				attestation, err := NewPullRequestAttestation("example", "example", 123, "commit", test.pr)

				assert.Nil(t, err)
				assert.NotNil(t, attestation)

				// Basic validation of the attestation structure
				assert.Equal(t, ita.StatementTypeUri, attestation.Type)
				assert.Equal(t, PullRequestPredicateType, attestation.PredicateType)
				assert.NotNil(t, attestation.Predicate, "Predicate should not be nil")
			})
		}
	})
}
