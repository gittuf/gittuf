// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/gittuf/gittuf/internal/attestations"
	authorizationsv01 "github.com/gittuf/gittuf/internal/attestations/authorizations/v01"
	"github.com/gittuf/gittuf/internal/attestations/github"
	githubv01 "github.com/gittuf/gittuf/internal/attestations/github/v01"
	"github.com/gittuf/gittuf/internal/common"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/signerverifier"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	sslibsv "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/signerverifier"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/stretchr/testify/assert"
)

func TestAddAndRemoveReferenceAuthorization(t *testing.T) {
	t.Setenv(dev.DevModeKey, "1")

	t.Run("for commits", func(t *testing.T) {
		testDir := t.TempDir()
		r := gitinterface.CreateTestGitRepository(t, testDir, false)

		// We meed to change the directory for this test because we `checkout`
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
		emptyTreeID, err := treeBuilder.WriteRootTreeFromBlobIDs(nil)
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
		if err := repo.RecordRSLEntryForReference(targetRef, false); err != nil {
			t.Fatal(err)
		}

		// Create feature branch with two Git commits
		// Add two commits
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, r, absFeatureRef, 2, gpgKeyBytes)
		featureCommitID := commitIDs[1]
		if err := repo.RecordRSLEntryForReference(featureRef, false); err != nil {
			t.Fatal(err)
		}

		targetTreeID, err := r.GetMergeTree(fromCommitID, featureCommitID)
		if err != nil {
			t.Fatal(err)
		}

		// Create signers
		firstSigner, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(rootKeyBytes) //nolint:staticcheck
		if err != nil {
			t.Fatal(err)
		}
		firstKeyID, err := firstSigner.KeyID()
		if err != nil {
			t.Fatal(err)
		}
		secondSigner, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(targetsKeyBytes) //nolint:staticcheck
		if err != nil {
			t.Fatal(err)
		}
		secondKeyID, err := secondSigner.KeyID()
		if err != nil {
			t.Fatal(err)
		}

		// First authorization attestation signature
		err = repo.AddReferenceAuthorization(testCtx, firstSigner, absTargetRef, absFeatureRef, false)
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
		err = repo.AddReferenceAuthorization(testCtx, secondSigner, absTargetRef, absFeatureRef, false)
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
		err = repo.RemoveReferenceAuthorization(testCtx, secondSigner, absTargetRef, fromCommitID.String(), targetTreeID.String(), false)
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

		// We meed to change the directory for this test because we `checkout`
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
		emptyTreeID, err := treeBuilder.WriteRootTreeFromBlobIDs(nil)
		if err != nil {
			t.Fatal(err)
		}
		initialCommitID, err := repo.r.Commit(emptyTreeID, fromRef, "Initial commit\n", false)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.RecordRSLEntryForReference(fromRef, false); err != nil {
			t.Fatal(err)
		}

		// Create signer
		signer, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(rootKeyBytes) //nolint:staticcheck
		if err != nil {
			t.Fatal(err)
		}
		keyID, err := signer.KeyID()
		if err != nil {
			t.Fatal(err)
		}

		err = repo.AddReferenceAuthorization(testCtx, signer, targetTagRef, fromRef, false)
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
		if err := repo.RecordRSLEntryForReference(targetTagRef, false); err != nil {
			t.Fatal(err)
		}

		// Trying to approve it now fails as we're approving a tag already seen in the RSL
		err = repo.AddReferenceAuthorization(testCtx, signer, targetTagRef, fromRef, false)
		assert.ErrorIs(t, err, gitinterface.ErrTagAlreadyExists)

		err = repo.RemoveReferenceAuthorization(testCtx, signer, targetTagRef, gitinterface.ZeroHash.String(), initialCommitID.String(), false)
		assert.Nil(t, err)

		allAttestations, err = attestations.LoadCurrentAttestations(r)
		if err != nil {
			t.Fatal(err)
		}

		_, err = allAttestations.GetReferenceAuthorizationFor(repo.r, targetTagRef, gitinterface.ZeroHash.String(), initialCommitID.String())
		assert.ErrorIs(t, err, attestations.ErrAuthorizationNotFound)
	})
}

func TestGetGitHubPullRequestApprovalPredicateFromEnvelope(t *testing.T) {
	tests := map[string]struct {
		envelope          *dsse.Envelope
		expectedPredicate github.PullRequestApprovalAttestation
	}{
		"one approver, no dismissals": {
			envelope: &dsse.Envelope{
				PayloadType: "application/vnd.gittuf+json",
				Payload:     "eyJ0eXBlIjoiaHR0cHM6Ly9pbi10b3RvLmlvL1N0YXRlbWVudC92MSIsInN1YmplY3QiOlt7ImRpZ2VzdCI6eyJnaXRUcmVlIjoiZWUyNWIxYjZjMjc4NjJlYTFjYzQxOWMxNDQxMjcxMjNmZDZmNDdkMyJ9fV0sInByZWRpY2F0ZV90eXBlIjoiaHR0cHM6Ly9naXR0dWYuZGV2L2dpdGh1Yi1wdWxsLXJlcXVlc3QtYXBwcm92YWwvdjAuMSIsInByZWRpY2F0ZSI6eyJhcHByb3ZlcnMiOlt7ImtleWlkIjoiYWxpY2U6Omh0dHBzOi8vZ2l0aHViLmNvbS9sb2dpbi9vYXV0aCIsImtleWlkX2hhc2hfYWxnb3JpdGhtcyI6bnVsbCwia2V5dHlwZSI6InNpZ3N0b3JlLW9pZGMiLCJrZXl2YWwiOnsiaWRlbnRpdHkiOiJhbGljZSIsImlzc3VlciI6Imh0dHBzOi8vZ2l0aHViLmNvbS9sb2dpbi9vYXV0aCJ9LCJzY2hlbWUiOiJmdWxjaW8ifV0sImRpc21pc3NlZEFwcHJvdmVycyI6bnVsbCwiZnJvbVJldmlzaW9uSUQiOiIyZjU5M2UzMTk1YTU5OTgzNDIzZjQ1ZmU2ZDQzMzVmMTQ4ZmZlZWNmIiwidGFyZ2V0UmVmIjoicmVmcy9oZWFkcy9tYWluIiwidGFyZ2V0VHJlZUlEIjoiZWUyNWIxYjZjMjc4NjJlYTFjYzQxOWMxNDQxMjcxMjNmZDZmNDdkMyJ9fQo=",
				Signatures: []dsse.Signature{
					{
						KeyID: "kid",
						Sig:   "sig",
					},
				},
			},
			expectedPredicate: &githubv01.GitHubPullRequestApprovalAttestation{
				Approvers: []*tuf.Key{
					{
						KeyType: signerverifier.FulcioKeyType,
						KeyID:   "alice::https://github.com/login/oauth",
						KeyVal: sslibsv.KeyVal{
							Identity: "alice",
							Issuer:   "https://github.com/login/oauth",
						},
						Scheme: signerverifier.FulcioKeyScheme,
					},
				},
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
				Payload:     "eyJ0eXBlIjoiaHR0cHM6Ly9pbi10b3RvLmlvL1N0YXRlbWVudC92MSIsInN1YmplY3QiOlt7ImRpZ2VzdCI6eyJnaXRUcmVlIjoiZWUyNWIxYjZjMjc4NjJlYTFjYzQxOWMxNDQxMjcxMjNmZDZmNDdkMyJ9fV0sInByZWRpY2F0ZV90eXBlIjoiaHR0cHM6Ly9naXR0dWYuZGV2L2dpdGh1Yi1wdWxsLXJlcXVlc3QtYXBwcm92YWwvdjAuMSIsInByZWRpY2F0ZSI6eyJhcHByb3ZlcnMiOlt7ImtleWlkIjoiYWxpY2U6Omh0dHBzOi8vZ2l0aHViLmNvbS9sb2dpbi9vYXV0aCIsImtleWlkX2hhc2hfYWxnb3JpdGhtcyI6bnVsbCwia2V5dHlwZSI6InNpZ3N0b3JlLW9pZGMiLCJrZXl2YWwiOnsiaWRlbnRpdHkiOiJhbGljZSIsImlzc3VlciI6Imh0dHBzOi8vZ2l0aHViLmNvbS9sb2dpbi9vYXV0aCJ9LCJzY2hlbWUiOiJmdWxjaW8ifV0sImRpc21pc3NlZEFwcHJvdmVycyI6W3sia2V5aWQiOiJib2I6Omh0dHBzOi8vZ2l0aHViLmNvbS9sb2dpbi9vYXV0aCIsImtleWlkX2hhc2hfYWxnb3JpdGhtcyI6bnVsbCwia2V5dHlwZSI6InNpZ3N0b3JlLW9pZGMiLCJrZXl2YWwiOnsiaWRlbnRpdHkiOiJib2IiLCJpc3N1ZXIiOiJodHRwczovL2dpdGh1Yi5jb20vbG9naW4vb2F1dGgifSwic2NoZW1lIjoiZnVsY2lvIn1dLCJmcm9tUmV2aXNpb25JRCI6IjJmNTkzZTMxOTVhNTk5ODM0MjNmNDVmZTZkNDMzNWYxNDhmZmVlY2YiLCJ0YXJnZXRSZWYiOiJyZWZzL2hlYWRzL21haW4iLCJ0YXJnZXRUcmVlSUQiOiJlZTI1YjFiNmMyNzg2MmVhMWNjNDE5YzE0NDEyNzEyM2ZkNmY0N2QzIn19Cg==",
				Signatures: []dsse.Signature{
					{
						KeyID: "kid",
						Sig:   "sig",
					},
				},
			},
			expectedPredicate: &githubv01.GitHubPullRequestApprovalAttestation{
				Approvers: []*tuf.Key{
					{
						KeyType: signerverifier.FulcioKeyType,
						KeyID:   "alice::https://github.com/login/oauth",
						KeyVal: sslibsv.KeyVal{
							Identity: "alice",
							Issuer:   "https://github.com/login/oauth",
						},
						Scheme: signerverifier.FulcioKeyScheme,
					},
				},
				DismissedApprovers: []*tuf.Key{
					{
						KeyType: signerverifier.FulcioKeyType,
						KeyID:   "bob::https://github.com/login/oauth",
						KeyVal: sslibsv.KeyVal{
							Identity: "bob",
							Issuer:   "https://github.com/login/oauth",
						},
						Scheme: signerverifier.FulcioKeyScheme,
					},
				},
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
				Payload:     "eyJ0eXBlIjoiaHR0cHM6Ly9pbi10b3RvLmlvL1N0YXRlbWVudC92MSIsInN1YmplY3QiOlt7ImRpZ2VzdCI6eyJnaXRUcmVlIjoiZWUyNWIxYjZjMjc4NjJlYTFjYzQxOWMxNDQxMjcxMjNmZDZmNDdkMyJ9fV0sInByZWRpY2F0ZV90eXBlIjoiaHR0cHM6Ly9naXR0dWYuZGV2L2dpdGh1Yi1wdWxsLXJlcXVlc3QtYXBwcm92YWwvdjAuMSIsInByZWRpY2F0ZSI6eyJhcHByb3ZlcnMiOm51bGwsImRpc21pc3NlZEFwcHJvdmVycyI6W3sia2V5aWQiOiJib2I6Omh0dHBzOi8vZ2l0aHViLmNvbS9sb2dpbi9vYXV0aCIsImtleWlkX2hhc2hfYWxnb3JpdGhtcyI6bnVsbCwia2V5dHlwZSI6InNpZ3N0b3JlLW9pZGMiLCJrZXl2YWwiOnsiaWRlbnRpdHkiOiJib2IiLCJpc3N1ZXIiOiJodHRwczovL2dpdGh1Yi5jb20vbG9naW4vb2F1dGgifSwic2NoZW1lIjoiZnVsY2lvIn1dLCJmcm9tUmV2aXNpb25JRCI6IjJmNTkzZTMxOTVhNTk5ODM0MjNmNDVmZTZkNDMzNWYxNDhmZmVlY2YiLCJ0YXJnZXRSZWYiOiJyZWZzL2hlYWRzL21haW4iLCJ0YXJnZXRUcmVlSUQiOiJlZTI1YjFiNmMyNzg2MmVhMWNjNDE5YzE0NDEyNzEyM2ZkNmY0N2QzIn19Cg==",
				Signatures: []dsse.Signature{
					{
						KeyID: "kid",
						Sig:   "sig",
					},
				},
			},
			expectedPredicate: &githubv01.GitHubPullRequestApprovalAttestation{
				DismissedApprovers: []*tuf.Key{
					{
						KeyType: signerverifier.FulcioKeyType,
						KeyID:   "bob::https://github.com/login/oauth",
						KeyVal: sslibsv.KeyVal{
							Identity: "bob",
							Issuer:   "https://github.com/login/oauth",
						},
						Scheme: signerverifier.FulcioKeyScheme,
					},
				},
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
				Payload:     "eyJ0eXBlIjoiaHR0cHM6Ly9pbi10b3RvLmlvL1N0YXRlbWVudC92MSIsInN1YmplY3QiOlt7ImRpZ2VzdCI6eyJnaXRUcmVlIjoiZWUyNWIxYjZjMjc4NjJlYTFjYzQxOWMxNDQxMjcxMjNmZDZmNDdkMyJ9fV0sInByZWRpY2F0ZV90eXBlIjoiaHR0cHM6Ly9naXR0dWYuZGV2L2dpdGh1Yi1wdWxsLXJlcXVlc3QtYXBwcm92YWwvdjAuMSIsInByZWRpY2F0ZSI6eyJhcHByb3ZlcnMiOlt7ImtleWlkIjoiYWxpY2U6Omh0dHBzOi8vZ2l0aHViLmNvbS9sb2dpbi9vYXV0aCIsImtleWlkX2hhc2hfYWxnb3JpdGhtcyI6bnVsbCwia2V5dHlwZSI6InNpZ3N0b3JlLW9pZGMiLCJrZXl2YWwiOnsiaWRlbnRpdHkiOiJhbGljZSIsImlzc3VlciI6Imh0dHBzOi8vZ2l0aHViLmNvbS9sb2dpbi9vYXV0aCJ9LCJzY2hlbWUiOiJmdWxjaW8ifSx7ImtleWlkIjoiYm9iOjpodHRwczovL2dpdGh1Yi5jb20vbG9naW4vb2F1dGgiLCJrZXlpZF9oYXNoX2FsZ29yaXRobXMiOm51bGwsImtleXR5cGUiOiJzaWdzdG9yZS1vaWRjIiwia2V5dmFsIjp7ImlkZW50aXR5IjoiYm9iIiwiaXNzdWVyIjoiaHR0cHM6Ly9naXRodWIuY29tL2xvZ2luL29hdXRoIn0sInNjaGVtZSI6ImZ1bGNpbyJ9XSwiZGlzbWlzc2VkQXBwcm92ZXJzIjpbeyJrZXlpZCI6ImFsaWNlOjpodHRwczovL2dpdGh1Yi5jb20vbG9naW4vb2F1dGgiLCJrZXlpZF9oYXNoX2FsZ29yaXRobXMiOm51bGwsImtleXR5cGUiOiJzaWdzdG9yZS1vaWRjIiwia2V5dmFsIjp7ImlkZW50aXR5IjoiYWxpY2UiLCJpc3N1ZXIiOiJodHRwczovL2dpdGh1Yi5jb20vbG9naW4vb2F1dGgifSwic2NoZW1lIjoiZnVsY2lvIn0seyJrZXlpZCI6ImJvYjo6aHR0cHM6Ly9naXRodWIuY29tL2xvZ2luL29hdXRoIiwia2V5aWRfaGFzaF9hbGdvcml0aG1zIjpudWxsLCJrZXl0eXBlIjoic2lnc3RvcmUtb2lkYyIsImtleXZhbCI6eyJpZGVudGl0eSI6ImJvYiIsImlzc3VlciI6Imh0dHBzOi8vZ2l0aHViLmNvbS9sb2dpbi9vYXV0aCJ9LCJzY2hlbWUiOiJmdWxjaW8ifV0sImZyb21SZXZpc2lvbklEIjoiMmY1OTNlMzE5NWE1OTk4MzQyM2Y0NWZlNmQ0MzM1ZjE0OGZmZWVjZiIsInRhcmdldFJlZiI6InJlZnMvaGVhZHMvbWFpbiIsInRhcmdldFRyZWVJRCI6ImVlMjViMWI2YzI3ODYyZWExY2M0MTljMTQ0MTI3MTIzZmQ2ZjQ3ZDMifX0K",
				Signatures: []dsse.Signature{
					{
						KeyID: "kid",
						Sig:   "sig",
					},
				},
			},
			expectedPredicate: &githubv01.GitHubPullRequestApprovalAttestation{
				Approvers: []*tuf.Key{
					{
						KeyType: signerverifier.FulcioKeyType,
						KeyID:   "alice::https://github.com/login/oauth",
						KeyVal: sslibsv.KeyVal{
							Identity: "alice",
							Issuer:   "https://github.com/login/oauth",
						},
						Scheme: signerverifier.FulcioKeyScheme,
					},
					{
						KeyType: signerverifier.FulcioKeyType,
						KeyID:   "bob::https://github.com/login/oauth",
						KeyVal: sslibsv.KeyVal{
							Identity: "bob",
							Issuer:   "https://github.com/login/oauth",
						},
						Scheme: signerverifier.FulcioKeyScheme,
					},
				},
				DismissedApprovers: []*tuf.Key{
					{
						KeyType: signerverifier.FulcioKeyType,
						KeyID:   "alice::https://github.com/login/oauth",
						KeyVal: sslibsv.KeyVal{
							Identity: "alice",
							Issuer:   "https://github.com/login/oauth",
						},
						Scheme: signerverifier.FulcioKeyScheme,
					},
					{
						KeyType: signerverifier.FulcioKeyType,
						KeyID:   "bob::https://github.com/login/oauth",
						KeyVal: sslibsv.KeyVal{
							Identity: "bob",
							Issuer:   "https://github.com/login/oauth",
						},
						Scheme: signerverifier.FulcioKeyScheme,
					},
				},
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
