// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"fmt"
	"os"
	"strings"
	"testing"

	attestopts "github.com/gittuf/gittuf/experimental/gittuf/options/attest"
	rslopts "github.com/gittuf/gittuf/experimental/gittuf/options/rsl"
	"github.com/gittuf/gittuf/internal/attestations"
	"github.com/gittuf/gittuf/internal/attestations/authorizations"
	authorizationsv01 "github.com/gittuf/gittuf/internal/attestations/authorizations/v01"
	githubv01 "github.com/gittuf/gittuf/internal/attestations/github/v01"
	"github.com/gittuf/gittuf/internal/common"
	"github.com/gittuf/gittuf/internal/common/set"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestAddAndRemoveReferenceAuthorization(t *testing.T) {
	t.Run("for commit", func(t *testing.T) {
		testDir := t.TempDir()
		r := gitinterface.CreateTestGitRepository(t, testDir, false)

		// We need to change the directory for this test because we `checkout`
		// for older Git versions, modifying the worktree. This chdir ensures
		// that the temporary directory is used as the worktree.
		pwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(testDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(pwd) //nolint:errcheck

		repo := &Repository{r: r}

		targetRef := "main"
		absTargetRef := "refs/heads/main"
		featureRef := "feature"
		absFeatureRef := "refs/heads/feature"

		// Create common base for main and feature branches
		treeBuilder := gitinterface.NewTreeBuilder(repo.r)
		emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
		if err != nil {
			t.Fatal(err)
		}
		initialCommitID, err := repo.r.Commit(emptyTreeID, absTargetRef, "Initial commit\n", false)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.r.SetReference(absFeatureRef, initialCommitID); err != nil {
			t.Fatal(err)
		}

		// Create main branch as the target branch with a Git commit
		// Add a single commit
		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, r, absTargetRef, 1, gpgKeyBytes)
		fromCommitID := commitIDs[0]
		if err := repo.RecordRSLEntryForReference(testCtx, targetRef, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		// Create feature branch with two Git commits
		// Add two commits
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, r, absFeatureRef, 2, gpgKeyBytes)
		featureCommitID := commitIDs[1]
		if err := repo.RecordRSLEntryForReference(testCtx, featureRef, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		targetTreeID, err := r.GetMergeTree(fromCommitID, featureCommitID)
		if err != nil {
			t.Fatal(err)
		}

		// Create signers
		firstSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
		firstKeyID, err := firstSigner.KeyID()
		if err != nil {
			t.Fatal(err)
		}

		secondSigner := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)
		secondKeyID, err := secondSigner.KeyID()
		if err != nil {
			t.Fatal(err)
		}

		// First authorization attestation signature
		err = repo.AddReferenceAuthorization(testCtx, firstSigner, absTargetRef, absFeatureRef, false, attestopts.WithRSLEntry())
		assert.Nil(t, err)

		allAttestations, err := attestations.LoadCurrentAttestations(r)
		if err != nil {
			t.Fatal(err)
		}

		env, err := allAttestations.GetReferenceAuthorizationFor(r, absTargetRef, fromCommitID.String(), targetTreeID.String())
		if err != nil {
			t.Fatal(err)
		}
		assert.Len(t, env.Signatures, 1)
		assert.Equal(t, firstKeyID, env.Signatures[0].KeyID)

		// Second authorization attestation signature
		err = repo.AddReferenceAuthorization(testCtx, secondSigner, absTargetRef, absFeatureRef, false, attestopts.WithRSLEntry())
		assert.Nil(t, err)

		allAttestations, err = attestations.LoadCurrentAttestations(r)
		if err != nil {
			t.Fatal(err)
		}

		env, err = allAttestations.GetReferenceAuthorizationFor(r, absTargetRef, fromCommitID.String(), targetTreeID.String())
		if err != nil {
			t.Fatal(err)
		}
		assert.Len(t, env.Signatures, 2)
		assert.Equal(t, firstKeyID, env.Signatures[0].KeyID)
		assert.Equal(t, secondKeyID, env.Signatures[1].KeyID)

		// Remove second authorization attestation signature
		err = repo.RemoveReferenceAuthorization(testCtx, secondSigner, absTargetRef, fromCommitID.String(), targetTreeID.String(), false, attestopts.WithRSLEntry())
		assert.Nil(t, err)

		allAttestations, err = attestations.LoadCurrentAttestations(r)
		if err != nil {
			t.Fatal(err)
		}

		env, err = allAttestations.GetReferenceAuthorizationFor(r, absTargetRef, fromCommitID.String(), targetTreeID.String())
		if err != nil {
			t.Fatal(err)
		}
		assert.Len(t, env.Signatures, 1)
		assert.Equal(t, firstKeyID, env.Signatures[0].KeyID)
	})

	t.Run("for tag", func(t *testing.T) {
		testDir := t.TempDir()
		r := gitinterface.CreateTestGitRepository(t, testDir, false)

		// We need to change the directory for this test because we `checkout`
		// for older Git versions, modifying the worktree. This chdir ensures
		// that the temporary directory is used as the worktree.
		pwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(testDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(pwd) //nolint:errcheck

		repo := &Repository{r: r}

		fromRef := "refs/heads/main"
		targetTagRef := "refs/tags/v1"

		// Create common base for main and feature branches
		treeBuilder := gitinterface.NewTreeBuilder(repo.r)
		emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
		if err != nil {
			t.Fatal(err)
		}
		initialCommitID, err := repo.r.Commit(emptyTreeID, fromRef, "Initial commit\n", false)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.RecordRSLEntryForReference(testCtx, fromRef, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		// Create signer
		signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
		keyID, err := signer.KeyID()
		if err != nil {
			t.Fatal(err)
		}

		err = repo.AddReferenceAuthorization(testCtx, signer, targetTagRef, fromRef, false, attestopts.WithRSLEntry(), attestopts.WithRSLEntry())
		assert.Nil(t, err)

		allAttestations, err := attestations.LoadCurrentAttestations(r)
		if err != nil {
			t.Fatal(err)
		}

		env, err := allAttestations.GetReferenceAuthorizationFor(repo.r, targetTagRef, gitinterface.ZeroHash.String(), initialCommitID.String())
		assert.Nil(t, err)
		assert.Len(t, env.Signatures, 1)
		assert.Equal(t, keyID, env.Signatures[0].KeyID)

		// Create tag
		_, err = repo.r.TagUsingSpecificKey(initialCommitID, strings.TrimPrefix(targetTagRef, gitinterface.TagRefPrefix), "v1", artifacts.SSHRSAPrivate)
		if err != nil {
			t.Fatal(err)
		}
		// Add it to RSL
		if err := repo.RecordRSLEntryForReference(testCtx, targetTagRef, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		// Trying to approve it now fails as we're approving a tag already seen in the RSL
		err = repo.AddReferenceAuthorization(testCtx, signer, targetTagRef, fromRef, false, attestopts.WithRSLEntry())
		assert.ErrorIs(t, err, gitinterface.ErrTagAlreadyExists)

		err = repo.RemoveReferenceAuthorization(testCtx, signer, targetTagRef, gitinterface.ZeroHash.String(), initialCommitID.String(), false, attestopts.WithRSLEntry())
		assert.Nil(t, err)

		allAttestations, err = attestations.LoadCurrentAttestations(r)
		if err != nil {
			t.Fatal(err)
		}

		_, err = allAttestations.GetReferenceAuthorizationFor(repo.r, targetTagRef, gitinterface.ZeroHash.String(), initialCommitID.String())
		assert.ErrorIs(t, err, authorizations.ErrAuthorizationNotFound)
	})
}

func TestGetGitHubPullRequestApprovalPredicateFromEnvelope(t *testing.T) {
	tests := map[string]struct {
		envelope          *dsse.Envelope
		expectedPredicate *githubv01.PullRequestApprovalAttestation
	}{
		"one approver, no dismissals": {
			envelope: &dsse.Envelope{
				PayloadType: "application/vnd.gittuf+json",
				Payload:     "eyJ0eXBlIjoiaHR0cHM6Ly9pbi10b3RvLmlvL1N0YXRlbWVudC92MSIsInN1YmplY3QiOlt7ImRpZ2VzdCI6eyJnaXRUcmVlIjoiZWUyNWIxYjZjMjc4NjJlYTFjYzQxOWMxNDQxMjcxMjNmZDZmNDdkMyJ9fV0sInByZWRpY2F0ZV90eXBlIjoiaHR0cHM6Ly9naXR0dWYuZGV2L2dpdGh1Yi1wdWxsLXJlcXVlc3QtYXBwcm92YWwvdjAuMSIsInByZWRpY2F0ZSI6eyJhcHByb3ZlcnMiOlsiYWxpY2UiXSwiZGlzbWlzc2VkQXBwcm92ZXJzIjpudWxsLCJmcm9tUmV2aXNpb25JRCI6IjJmNTkzZTMxOTVhNTk5ODM0MjNmNDVmZTZkNDMzNWYxNDhmZmVlY2YiLCJ0YXJnZXRSZWYiOiJyZWZzL2hlYWRzL21haW4iLCJ0YXJnZXRUcmVlSUQiOiJlZTI1YjFiNmMyNzg2MmVhMWNjNDE5YzE0NDEyNzEyM2ZkNmY0N2QzIn19Cg==",
				Signatures: []dsse.Signature{
					{
						KeyID: "kid",
						Sig:   "sig",
					},
				},
			},
			expectedPredicate: &githubv01.PullRequestApprovalAttestation{
				Approvers: set.NewSetFromItems("alice"),
				ReferenceAuthorization: &authorizationsv01.ReferenceAuthorization{
					FromRevisionID: "2f593e3195a59983423f45fe6d4335f148ffeecf",
					TargetRef:      "refs/heads/main",
					TargetTreeID:   "ee25b1b6c27862ea1cc419c144127123fd6f47d3",
				},
			},
		},
		"one approver, one dismissal": {
			envelope: &dsse.Envelope{
				PayloadType: "application/vnd.gittuf+json",
				Payload:     "eyJ0eXBlIjoiaHR0cHM6Ly9pbi10b3RvLmlvL1N0YXRlbWVudC92MSIsInN1YmplY3QiOlt7ImRpZ2VzdCI6eyJnaXRUcmVlIjoiZWUyNWIxYjZjMjc4NjJlYTFjYzQxOWMxNDQxMjcxMjNmZDZmNDdkMyJ9fV0sInByZWRpY2F0ZV90eXBlIjoiaHR0cHM6Ly9naXR0dWYuZGV2L2dpdGh1Yi1wdWxsLXJlcXVlc3QtYXBwcm92YWwvdjAuMSIsInByZWRpY2F0ZSI6eyJhcHByb3ZlcnMiOlsiYWxpY2UiXSwiZGlzbWlzc2VkQXBwcm92ZXJzIjpbImJvYiJdLCJmcm9tUmV2aXNpb25JRCI6IjJmNTkzZTMxOTVhNTk5ODM0MjNmNDVmZTZkNDMzNWYxNDhmZmVlY2YiLCJ0YXJnZXRSZWYiOiJyZWZzL2hlYWRzL21haW4iLCJ0YXJnZXRUcmVlSUQiOiJlZTI1YjFiNmMyNzg2MmVhMWNjNDE5YzE0NDEyNzEyM2ZkNmY0N2QzIn19Cg==",
				Signatures: []dsse.Signature{
					{
						KeyID: "kid",
						Sig:   "sig",
					},
				},
			},
			expectedPredicate: &githubv01.PullRequestApprovalAttestation{
				Approvers:          set.NewSetFromItems("alice"),
				DismissedApprovers: set.NewSetFromItems("bob"),
				ReferenceAuthorization: &authorizationsv01.ReferenceAuthorization{
					FromRevisionID: "2f593e3195a59983423f45fe6d4335f148ffeecf",
					TargetRef:      "refs/heads/main",
					TargetTreeID:   "ee25b1b6c27862ea1cc419c144127123fd6f47d3",
				},
			},
		},
		"no approvers, one dismissal": {
			envelope: &dsse.Envelope{
				PayloadType: "application/vnd.gittuf+json",
				Payload:     "eyJ0eXBlIjoiaHR0cHM6Ly9pbi10b3RvLmlvL1N0YXRlbWVudC92MSIsInN1YmplY3QiOlt7ImRpZ2VzdCI6eyJnaXRUcmVlIjoiZWUyNWIxYjZjMjc4NjJlYTFjYzQxOWMxNDQxMjcxMjNmZDZmNDdkMyJ9fV0sInByZWRpY2F0ZV90eXBlIjoiaHR0cHM6Ly9naXR0dWYuZGV2L2dpdGh1Yi1wdWxsLXJlcXVlc3QtYXBwcm92YWwvdjAuMSIsInByZWRpY2F0ZSI6eyJhcHByb3ZlcnMiOm51bGwsImRpc21pc3NlZEFwcHJvdmVycyI6WyJib2IiXSwiZnJvbVJldmlzaW9uSUQiOiIyZjU5M2UzMTk1YTU5OTgzNDIzZjQ1ZmU2ZDQzMzVmMTQ4ZmZlZWNmIiwidGFyZ2V0UmVmIjoicmVmcy9oZWFkcy9tYWluIiwidGFyZ2V0VHJlZUlEIjoiZWUyNWIxYjZjMjc4NjJlYTFjYzQxOWMxNDQxMjcxMjNmZDZmNDdkMyJ9fQo=",
				Signatures: []dsse.Signature{
					{
						KeyID: "kid",
						Sig:   "sig",
					},
				},
			},
			expectedPredicate: &githubv01.PullRequestApprovalAttestation{
				DismissedApprovers: set.NewSetFromItems("bob"),
				ReferenceAuthorization: &authorizationsv01.ReferenceAuthorization{
					FromRevisionID: "2f593e3195a59983423f45fe6d4335f148ffeecf",
					TargetRef:      "refs/heads/main",
					TargetTreeID:   "ee25b1b6c27862ea1cc419c144127123fd6f47d3",
				},
			},
		},
		"multiple approvers, multiple dismissals": {
			envelope: &dsse.Envelope{
				PayloadType: "application/vnd.gittuf+json",
				Payload:     "eyJ0eXBlIjoiaHR0cHM6Ly9pbi10b3RvLmlvL1N0YXRlbWVudC92MSIsInN1YmplY3QiOlt7ImRpZ2VzdCI6eyJnaXRUcmVlIjoiZWUyNWIxYjZjMjc4NjJlYTFjYzQxOWMxNDQxMjcxMjNmZDZmNDdkMyJ9fV0sInByZWRpY2F0ZV90eXBlIjoiaHR0cHM6Ly9naXR0dWYuZGV2L2dpdGh1Yi1wdWxsLXJlcXVlc3QtYXBwcm92YWwvdjAuMSIsInByZWRpY2F0ZSI6eyJhcHByb3ZlcnMiOlsiYWxpY2UiLCJib2IiXSwiZGlzbWlzc2VkQXBwcm92ZXJzIjpbImFsaWNlIiwiYm9iIl0sImZyb21SZXZpc2lvbklEIjoiMmY1OTNlMzE5NWE1OTk4MzQyM2Y0NWZlNmQ0MzM1ZjE0OGZmZWVjZiIsInRhcmdldFJlZiI6InJlZnMvaGVhZHMvbWFpbiIsInRhcmdldFRyZWVJRCI6ImVlMjViMWI2YzI3ODYyZWExY2M0MTljMTQ0MTI3MTIzZmQ2ZjQ3ZDMifX0K",
				Signatures: []dsse.Signature{
					{
						KeyID: "kid",
						Sig:   "sig",
					},
				},
			},
			expectedPredicate: &githubv01.PullRequestApprovalAttestation{
				Approvers:          set.NewSetFromItems("alice", "bob"),
				DismissedApprovers: set.NewSetFromItems("alice", "bob"),
				ReferenceAuthorization: &authorizationsv01.ReferenceAuthorization{
					FromRevisionID: "2f593e3195a59983423f45fe6d4335f148ffeecf",
					TargetRef:      "refs/heads/main",
					TargetTreeID:   "ee25b1b6c27862ea1cc419c144127123fd6f47d3",
				},
			},
		},
	}

	for name, test := range tests {
		predicate, err := getGitHubPullRequestApprovalPredicateFromEnvelope(test.envelope)
		assert.Nil(t, err, fmt.Sprintf("unexpected error in test '%s'", name))
		assert.Equal(t, test.expectedPredicate, predicate, fmt.Sprintf("unexpected predicate in test '%s'", name))
	}
}

func TestIndexPathToComponents(t *testing.T) {
	tests := map[string]struct {
		baseRef string
		from    string
		to      string
	}{
		"simple ref": {
			baseRef: "refs/heads/main",
			from:    gitinterface.ZeroHash.String(),
			to:      gitinterface.ZeroHash.String(),
		},
		"complicated ref": {
			baseRef: "refs/heads/jane.doe/feature-branch",
			from:    gitinterface.ZeroHash.String(),
			to:      gitinterface.ZeroHash.String(),
		},
	}

	for name, test := range tests {
		// construct indexPath programmatically to force breaking changes /
		// regressions to be detected here
		indexPath := attestations.GitHubPullRequestApprovalAttestationPath(test.baseRef, test.from, test.to)

		baseRef, from, to := indexPathToComponents(indexPath)
		assert.Equal(t, test.baseRef, baseRef, fmt.Sprintf("unexpected 'base ref' in test '%s'", name))
		assert.Equal(t, test.from, from, fmt.Sprintf("unexpected 'from' in test '%s'", name))
		assert.Equal(t, test.to, to, fmt.Sprintf("unexpected 'to' in test '%s'", name))
	}
}
