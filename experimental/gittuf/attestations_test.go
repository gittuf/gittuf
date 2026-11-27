// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	attestopts "github.com/gittuf/gittuf/experimental/gittuf/options/attest"
	githubopts "github.com/gittuf/gittuf/experimental/gittuf/options/github"
	rslopts "github.com/gittuf/gittuf/experimental/gittuf/options/rsl"
	"github.com/gittuf/gittuf/internal/attestations"
	"github.com/gittuf/gittuf/internal/attestations/authorizations"
	authorizationsv01 "github.com/gittuf/gittuf/internal/attestations/authorizations/v01"
	"github.com/gittuf/gittuf/internal/attestations/github"
	githubv01 "github.com/gittuf/gittuf/internal/attestations/github/v01"
	"github.com/gittuf/gittuf/internal/common"
	"github.com/gittuf/gittuf/internal/common/set"
	signingdsse "github.com/gittuf/gittuf/internal/signerverifier/dsse"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	tuf "github.com/gittuf/gittuf/internal/tuf"
	"github.com/gittuf/gittuf/pkg/githash"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/gittuf/gittuf/pkg/rsl"
	gogithub "github.com/google/go-github/v61/github"
	ita "github.com/in-toto/attestation/go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyAttestations(t *testing.T) {
	remoteName := "origin"
	testDir := t.TempDir()
	r := gitinterface.CreateTestGitRepository(t, testDir, false)
	repo := &Repository{r: newStorer(r)}

	fromRef := "refs/heads/main"
	targetTagRef := "refs/tags/v1"

	treeBuilder := gitinterface.NewTreeBuilder(repo.r.Repository)
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

	signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

	if err := repo.AddReferenceAuthorization(testCtx, signer, targetTagRef, fromRef, false); err != nil {
		t.Fatal(err)
	}

	attestationsCommitID, err := repo.r.GetReference(attestations.Ref)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = rsl.GetLatestReferenceUpdaterEntry(repo.r, rsl.ForReference(attestations.Ref))
	assert.ErrorIs(t, err, rsl.ErrRSLEntryNotFound)

	err = repo.ApplyAttestations(testCtx, "", true, false)
	assert.NoError(t, err)

	entry, _, err := rsl.GetLatestReferenceUpdaterEntry(repo.r, rsl.ForReference(attestations.Ref))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, attestations.Ref, entry.GetRefName())
	assert.Equal(t, attestationsCommitID, entry.GetTargetID())

	allAttestations, err := attestations.LoadCurrentAttestations(repo.r)
	if err != nil {
		t.Fatal(err)
	}
	env, err := allAttestations.GetReferenceAuthorizationFor(repo.r, targetTagRef, gitinterface.ZeroHash.String(), initialCommitID.String())
	if err != nil {
		t.Fatal(err)
	}
	assert.Len(t, env.Signatures, 1)

	t.Run("miscellaneous error checking", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		nr := &Repository{r: newStorer(repo)}

		// Test signCommit
		err := repo.SetGitConfig("user.signingkey", "")
		if err != nil {
			t.Fatal(err)
		}

		err = nr.ApplyAttestations(testCtx, remoteName, false, true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)
	})
}

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

		repo := &Repository{r: newStorer(r)}

		targetRef := "main"
		absTargetRef := "refs/heads/main"
		featureRef := "feature"
		absFeatureRef := "refs/heads/feature"

		// Create common base for main and feature branches
		treeBuilder := gitinterface.NewTreeBuilder(repo.r.Repository)
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
		assert.NoError(t, err)

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
		assert.NoError(t, err)

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
		assert.NoError(t, err)

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

		repo := &Repository{r: newStorer(r)}

		fromRef := "refs/heads/main"
		targetTagRef := "refs/tags/v1"

		// Create common base for main and feature branches
		treeBuilder := gitinterface.NewTreeBuilder(repo.r.Repository)
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
		assert.NoError(t, err)

		allAttestations, err := attestations.LoadCurrentAttestations(r)
		if err != nil {
			t.Fatal(err)
		}

		env, err := allAttestations.GetReferenceAuthorizationFor(repo.r, targetTagRef, gitinterface.ZeroHash.String(), initialCommitID.String())
		assert.NoError(t, err)
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
		assert.NoError(t, err)

		allAttestations, err = attestations.LoadCurrentAttestations(r)
		if err != nil {
			t.Fatal(err)
		}

		_, err = allAttestations.GetReferenceAuthorizationFor(repo.r, targetTagRef, gitinterface.ZeroHash.String(), initialCommitID.String())
		assert.ErrorIs(t, err, authorizations.ErrAuthorizationNotFound)
	})

	t.Run("miscellaneous error checking", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		nr := &Repository{r: newStorer(repo)}

		targetsSigner := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)

		// Test signCommit
		err := repo.SetGitConfig("user.signingkey", "")
		if err != nil {
			t.Fatal(err)
		}

		err = nr.AddReferenceAuthorization(testCtx, nil, "", "", true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)

		err = nr.RemoveReferenceAuthorization(testCtx, nil, "", "", "", true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)

		// Test nonexistent target ref
		err = nr.AddReferenceAuthorization(testCtx, nil, "nonexistent", "", false)
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)

		err = nr.RemoveReferenceAuthorization(testCtx, targetsSigner, "nonexistent", "", "", false)
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)
	})
}

func TestAddGitHubPullRequestAttestationForCommit(t *testing.T) {
	t.Run("miscellaneous error checking", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		nr := &Repository{r: newStorer(repo)}

		// Test signCommit
		err := repo.SetGitConfig("user.signingkey", "")
		if err != nil {
			t.Fatal(err)
		}

		err = nr.AddGitHubPullRequestAttestationForCommit(testCtx, nil, "", "", "", "", true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)

		// Test no GitHub token
		err = nr.AddGitHubPullRequestAttestationForCommit(testCtx, nil, "", "", "", "", false, githubopts.WithGitHubTokenSource(staticTokenSource("")))
		assert.ErrorIs(t, err, ErrNoGitHubToken)
	})

	t.Run("test mocked API", func(t *testing.T) {
		const mergedCommitID = "0123456789012345678901234567890123456789"
		server := newTestGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/exampleorg/example/commits/"+mergedCommitID+"/pulls" {
				writeJSONResponse(t, w, []*gogithub.PullRequest{
					{
						ID:             gogithub.Int64(1),
						Number:         gogithub.Int(1),
						MergeCommitSHA: gogithub.String(mergedCommitID),
						Base: &gogithub.PullRequestBranch{
							Ref:  gogithub.String("main"),
							User: &gogithub.User{Login: gogithub.String("exampleorg"), ID: gogithub.Int64(42)},
							SHA:  gogithub.String("a"),
						},
						MergedAt: &gogithub.Timestamp{Time: time.Now()},
					},
				})
				return
			}
			http.NotFound(w, r)
		})

		testDir := t.TempDir()
		r := gitinterface.CreateTestGitRepository(t, testDir, false)

		pwd, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(testDir))
		defer os.Chdir(pwd) //nolint:errcheck

		repo := &Repository{r: newStorer(r)}

		targetRef := "main"
		absTargetRef := "refs/heads/main"
		featureRef := "feature"
		absFeatureRef := "refs/heads/feature"

		treeBuilder := gitinterface.NewTreeBuilder(repo.GetGitRepository())
		emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
		require.NoError(t, err)
		initialCommitID, err := repo.r.Commit(emptyTreeID, absTargetRef, "Initial commit\n", false)
		require.NoError(t, err)
		require.NoError(t, repo.r.SetReference(absFeatureRef, initialCommitID))

		_ = common.AddNTestCommitsToSpecifiedRef(t, r, absTargetRef, 1, gpgKeyBytes)
		require.NoError(t, repo.RecordRSLEntryForReference(testCtx, targetRef, false, rslopts.WithRecordLocalOnly()))

		_ = common.AddNTestCommitsToSpecifiedRef(t, r, absFeatureRef, 2, gpgKeyBytes)
		require.NoError(t, repo.RecordRSLEntryForReference(testCtx, featureRef, false, rslopts.WithRecordLocalOnly()))

		signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

		err = repo.AddGitHubPullRequestAttestationForCommit(context.Background(), signer, "exampleorg", "example", mergedCommitID, "main", false, testGitHubOptions(server.URL)...)
		require.NoError(t, err)

		assertStoredGitHubPullRequest(t, repo, signer.Verifier, mergedCommitID)
	})
}

func TestAddGitHubPullRequestAttestationForNumber(t *testing.T) {
	t.Run("miscellaneous error checking", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		nr := &Repository{r: newStorer(repo)}

		// Test signCommit
		err := repo.SetGitConfig("user.signingkey", "")
		if err != nil {
			t.Fatal(err)
		}

		err = nr.AddGitHubPullRequestAttestationForNumber(testCtx, nil, "", "", 1, true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)

		// Test no GitHub token
		err = nr.AddGitHubPullRequestAttestationForNumber(testCtx, nil, "", "", 1, false, githubopts.WithGitHubTokenSource(staticTokenSource("")))
		assert.ErrorIs(t, err, ErrNoGitHubToken)
	})

	t.Run("test mocked API", func(t *testing.T) {
		const mergedCommitID = "0123456789012345678901234567890123456789"
		server := newTestGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/exampleorg/example/pulls/1" {
				writeJSONResponse(t, w, &gogithub.PullRequest{
					ID:             gogithub.Int64(1),
					Number:         gogithub.Int(1),
					MergeCommitSHA: gogithub.String(mergedCommitID),
					Base: &gogithub.PullRequestBranch{
						Ref:  gogithub.String("main"),
						User: &gogithub.User{Login: gogithub.String("exampleorg"), ID: gogithub.Int64(42)},
						SHA:  gogithub.String("a"),
					},
					MergedAt: &gogithub.Timestamp{Time: time.Now()},
				})
				return
			}
			http.NotFound(w, r)
		})

		testDir := t.TempDir()
		r := gitinterface.CreateTestGitRepository(t, testDir, false)

		pwd, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(testDir))
		defer os.Chdir(pwd) //nolint:errcheck

		repo := &Repository{r: newStorer(r)}

		targetRef := "main"
		absTargetRef := "refs/heads/main"
		featureRef := "feature"
		absFeatureRef := "refs/heads/feature"

		treeBuilder := gitinterface.NewTreeBuilder(repo.GetGitRepository())
		emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
		require.NoError(t, err)
		initialCommitID, err := repo.r.Commit(emptyTreeID, absTargetRef, "Initial commit\n", false)
		require.NoError(t, err)
		require.NoError(t, repo.r.SetReference(absFeatureRef, initialCommitID))

		_ = common.AddNTestCommitsToSpecifiedRef(t, r, absTargetRef, 1, gpgKeyBytes)
		require.NoError(t, repo.RecordRSLEntryForReference(testCtx, targetRef, false, rslopts.WithRecordLocalOnly()))

		_ = common.AddNTestCommitsToSpecifiedRef(t, r, absFeatureRef, 2, gpgKeyBytes)
		require.NoError(t, repo.RecordRSLEntryForReference(testCtx, featureRef, false, rslopts.WithRecordLocalOnly()))

		signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

		err = repo.AddGitHubPullRequestAttestationForNumber(context.Background(), signer, "exampleorg", "example", 1, false, testGitHubOptions(server.URL)...)
		require.NoError(t, err)

		assertStoredGitHubPullRequest(t, repo, signer.Verifier, mergedCommitID)
	})
}

func TestAddGitHubPullRequestApprover(t *testing.T) {
	t.Run("miscellaneous error checking", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		nr := &Repository{r: newStorer(repo)}

		targetsSigner := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)

		// Test signCommit
		err := repo.SetGitConfig("user.signingkey", "")
		if err != nil {
			t.Fatal(err)
		}

		err = nr.AddGitHubPullRequestApprover(testCtx, nil, "", "", 1, 1, "", true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)

		// Test no GitHub token
		err = nr.AddGitHubPullRequestApprover(testCtx, targetsSigner, "", "", 1, 1, "", false, githubopts.WithGitHubTokenSource(staticTokenSource("")))
		assert.ErrorIs(t, err, ErrNoGitHubToken)
	})

	t.Run("test mocked API", func(t *testing.T) {
		testDir := t.TempDir()
		r := gitinterface.CreateTestGitRepository(t, testDir, false)

		pwd, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(testDir))
		defer os.Chdir(pwd) //nolint:errcheck

		repo := &Repository{r: newStorer(r)}

		targetRef := "main"
		absTargetRef := "refs/heads/main"
		featureRef := "feature"
		absFeatureRef := "refs/heads/feature"

		treeBuilder := gitinterface.NewTreeBuilder(repo.GetGitRepository())
		emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
		require.NoError(t, err)
		initialCommitID, err := repo.r.Commit(emptyTreeID, absTargetRef, "Initial commit\n", false)
		require.NoError(t, err)
		require.NoError(t, repo.r.SetReference(absFeatureRef, initialCommitID))

		_ = common.AddNTestCommitsToSpecifiedRef(t, r, absTargetRef, 1, gpgKeyBytes)
		require.NoError(t, repo.RecordRSLEntryForReference(testCtx, targetRef, false, rslopts.WithRecordLocalOnly()))

		featureCommitIDs := common.AddNTestCommitsToSpecifiedRef(t, r, absFeatureRef, 2, gpgKeyBytes)
		featureHeadCommitID := featureCommitIDs[1]
		require.NoError(t, repo.RecordRSLEntryForReference(testCtx, featureRef, false, rslopts.WithRecordLocalOnly()))

		pr := &gogithub.PullRequest{
			ID: gogithub.Int64(1),
			Base: &gogithub.PullRequestBranch{
				Ref: gogithub.String("main"),
				SHA: gogithub.String(initialCommitID.String()),
			},
			Head: &gogithub.PullRequestBranch{
				Ref: gogithub.String("feature"),
				SHA: gogithub.String(featureHeadCommitID.String()),
				Repo: &gogithub.Repository{
					CloneURL: gogithub.String(testDir),
				},
			},
			MergedAt: &gogithub.Timestamp{Time: time.Now()},
		}

		server := newTestGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/exampleorg/example/pulls/1":
				writeJSONResponse(t, w, pr)
			case r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/exampleorg/example/pulls/1/reviews/123":
				writeJSONResponse(t, w, &gogithub.PullRequestReview{ID: gogithub.Int64(123)})
			default:
				http.NotFound(w, r)
			}
		})

		signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

		err = repo.AddGitHubPullRequestAttestationForNumber(context.Background(), signer, "exampleorg", "example", 1, false, testGitHubOptions(server.URL)...)
		require.NoError(t, err)

		err = repo.AddGitHubPullRequestApprover(context.Background(), signer, "exampleorg", "example", 1, 123, "bob", false, testGitHubOptions(server.URL)...)
		require.NoError(t, err)

		err = repo.ApplyAttestations(testCtx, "", true, false)
		require.NoError(t, err)

		allAttestations, err := attestations.LoadCurrentAttestations(repo.r)
		require.NoError(t, err)

		env, err := allAttestations.GetGitHubPullRequestApprovalAttestationForReviewID(repo.r, server.URL, 123, tuf.GitHubAppRoleName)
		require.NoError(t, err)
		assert.NotNil(t, env)

		predicate, err := getGitHubPullRequestApprovalPredicateFromEnvelope(env)
		require.NoError(t, err)
		assert.Contains(t, predicate.GetApprovers(), "bob")
	})
}

func TestDismissGitHubPullRequestApprover(t *testing.T) {
	t.Run("miscellaneous error checking", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		nr := &Repository{r: newStorer(repo)}

		targetsSigner := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)

		// Test signCommit
		err := repo.SetGitConfig("user.signingkey", "")
		if err != nil {
			t.Fatal(err)
		}

		err = nr.DismissGitHubPullRequestApprover(testCtx, nil, 1, "", true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)

		// Test non-existent review
		err = nr.DismissGitHubPullRequestApprover(testCtx, targetsSigner, 1, "", false)
		assert.ErrorIs(t, err, github.ErrGitHubReviewIDNotFound)
	})

	t.Run("test mocked API", func(t *testing.T) {
		testDir := t.TempDir()
		r := gitinterface.CreateTestGitRepository(t, testDir, false)

		pwd, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(testDir))
		defer os.Chdir(pwd) //nolint:errcheck

		repo := &Repository{r: newStorer(r)}

		targetRef := "main"
		absTargetRef := "refs/heads/main"
		featureRef := "feature"
		absFeatureRef := "refs/heads/feature"

		treeBuilder := gitinterface.NewTreeBuilder(repo.GetGitRepository())
		emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
		require.NoError(t, err)
		initialCommitID, err := repo.r.Commit(emptyTreeID, absTargetRef, "Initial commit\n", false)
		require.NoError(t, err)
		require.NoError(t, repo.r.SetReference(absFeatureRef, initialCommitID))

		_ = common.AddNTestCommitsToSpecifiedRef(t, r, absTargetRef, 1, gpgKeyBytes)
		require.NoError(t, repo.RecordRSLEntryForReference(testCtx, targetRef, false, rslopts.WithRecordLocalOnly()))

		featureCommitIDs := common.AddNTestCommitsToSpecifiedRef(t, r, absFeatureRef, 2, gpgKeyBytes)
		featureHeadCommitID := featureCommitIDs[1]
		require.NoError(t, repo.RecordRSLEntryForReference(testCtx, featureRef, false, rslopts.WithRecordLocalOnly()))

		pr := &gogithub.PullRequest{
			ID: gogithub.Int64(1),
			Base: &gogithub.PullRequestBranch{
				Ref: gogithub.String("main"),
				SHA: gogithub.String(initialCommitID.String()),
			},
			Head: &gogithub.PullRequestBranch{
				Ref: gogithub.String("feature"),
				SHA: gogithub.String(featureHeadCommitID.String()),
				Repo: &gogithub.Repository{
					CloneURL: gogithub.String(testDir),
				},
			},
			MergedAt: &gogithub.Timestamp{Time: time.Now()},
		}

		server := newTestGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/exampleorg/example/pulls/1":
				writeJSONResponse(t, w, pr)
			case r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/exampleorg/example/pulls/1/reviews/123":
				writeJSONResponse(t, w, &gogithub.PullRequestReview{ID: gogithub.Int64(123)})
			default:
				http.NotFound(w, r)
			}
		})

		signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

		err = repo.AddGitHubPullRequestAttestationForNumber(context.Background(), signer, "exampleorg", "example", 1, false, testGitHubOptions(server.URL)...)
		require.NoError(t, err)

		err = repo.AddGitHubPullRequestApprover(context.Background(), signer, "exampleorg", "example", 1, 123, "bob", false, testGitHubOptions(server.URL)...)
		require.NoError(t, err)

		err = repo.ApplyAttestations(testCtx, "", true, false)
		require.NoError(t, err)

		err = repo.DismissGitHubPullRequestApprover(context.Background(), signer, 123, "bob", false, testGitHubOptions(server.URL)...)
		require.NoError(t, err)

		err = repo.ApplyAttestations(testCtx, "", true, false)
		require.NoError(t, err)

		allAttestations, err := attestations.LoadCurrentAttestations(repo.r)
		require.NoError(t, err)

		env, err := allAttestations.GetGitHubPullRequestApprovalAttestationForReviewID(repo.r, server.URL, 123, tuf.GitHubAppRoleName)
		require.NoError(t, err)
		assert.NotNil(t, env)

		predicate, err := getGitHubPullRequestApprovalPredicateFromEnvelope(env)
		require.NoError(t, err)
		assert.NotContains(t, predicate.GetApprovers(), "bob")
		assert.Contains(t, predicate.GetDismissedApprovers(), "bob")
	})
}

func TestAddReferenceAuthorizationForNewTagZeroHashFormat(t *testing.T) {
	for _, objectFormat := range testObjectFormats {
		t.Run(string(objectFormat), func(t *testing.T) {
			testDir := t.TempDir()
			r := gitinterface.CreateTestGitRepository(t, testDir, false, gitinterface.WithObjectFormat(objectFormat))

			// We need to change the directory because we `checkout` for older
			// Git versions, modifying the worktree.
			pwd, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Chdir(testDir); err != nil {
				t.Fatal(err)
			}
			defer os.Chdir(pwd) //nolint:errcheck

			repo := &Repository{r: newStorer(r)}

			fromRef := "refs/heads/main"
			targetTagRef := "refs/tags/v1"

			treeBuilder := gitinterface.NewTreeBuilder(repo.r.Repository)
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

			signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

			// AddReferenceAuthorization calls r.r.ZeroHash() for the tag's fromID.
			err = repo.AddReferenceAuthorization(testCtx, signer, targetTagRef, fromRef, false, attestopts.WithRSLEntry())
			assert.NoError(t, err)

			allAttestations, err := attestations.LoadCurrentAttestations(r)
			if err != nil {
				t.Fatal(err)
			}

			// The format-correct zero hash must retrieve the authorization.
			formatZero := repo.r.ZeroHash().String()
			env, err := allAttestations.GetReferenceAuthorizationFor(repo.r, targetTagRef, formatZero, initialCommitID.String())
			assert.NoError(t, err)
			assert.Len(t, env.Signatures, 1)

			// On a SHA-256 repo, looking up with the SHA-1 zero (40 zeros)
			// must NOT find the authorization, pinning width-consistency.
			if objectFormat == gitinterface.ObjectFormatSHA256 {
				sha1Zero := gitinterface.ZeroHash.String()
				_, err = allAttestations.GetReferenceAuthorizationFor(repo.r, targetTagRef, sha1Zero, initialCommitID.String())
				assert.ErrorIs(t, err, authorizations.ErrAuthorizationNotFound)
			}
		})
	}
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

func TestGetGitHubClient(t *testing.T) {
	t.Run("default baseURL keeps github.com endpoints", func(t *testing.T) {
		client, err := getGitHubClient(githubopts.DefaultGitHubBaseURL, "test-token")
		assert.NoError(t, err)
		assert.NotNil(t, client)

		// Default go-github BaseURL is api.github.com
		assert.Equal(t, "https://api.github.com/", client.BaseURL.String())
		assert.Equal(t, "https://uploads.github.com/", client.UploadURL.String())
	})

	t.Run("enterprise baseURL gets /api/v3 and /api/uploads paths", func(t *testing.T) {
		client, err := getGitHubClient("https://github.example.com", "test-token")
		assert.NoError(t, err)
		assert.NotNil(t, client)

		assert.Equal(t, "https://github.example.com/api/v3/", client.BaseURL.String())
		assert.Equal(t, "https://github.example.com/api/uploads/", client.UploadURL.String())
	})

	t.Run("trailing slash in baseURL is normalized", func(t *testing.T) {
		client, err := getGitHubClient("https://github.example.com/", "test-token")
		assert.NoError(t, err)
		assert.NotNil(t, client)

		// Should produce the same paths as the no-trailing-slash case
		assert.Equal(t, "https://github.example.com/api/v3/", client.BaseURL.String())
		assert.Equal(t, "https://github.example.com/api/uploads/", client.UploadURL.String())
	})

	t.Run("invalid baseURL returns error", func(t *testing.T) {
		_, err := getGitHubClient("://no-scheme", "test-token")
		var urlErr *url.Error
		assert.ErrorAs(t, err, &urlErr)
	})

	t.Run("empty token still produces a usable client", func(t *testing.T) {
		// getGitHubClient itself doesn't enforce non-empty token;
		// callers do (via ErrNoGitHubToken). This documents that behavior.
		client, err := getGitHubClient(githubopts.DefaultGitHubBaseURL, "")
		assert.NoError(t, err)
		assert.NotNil(t, client)
	})
}

func TestGetGitHubPullRequestReviewDetails(t *testing.T) {
	t.Run("review is already stored in the attestations state", func(t *testing.T) {
		var requests atomic.Int32
		testDir := t.TempDir()
		r := gitinterface.CreateTestGitRepository(t, testDir, false)
		repo := &Repository{r: newStorer(r)}

		targetRef := "main"
		absTargetRef := "refs/heads/main"
		featureRef := "feature"
		absFeatureRef := "refs/heads/feature"

		treeBuilder := gitinterface.NewTreeBuilder(repo.GetGitRepository())
		emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
		require.NoError(t, err)
		initialCommitID, err := repo.r.Commit(emptyTreeID, absTargetRef, "Initial commit\n", false)
		require.NoError(t, err)
		require.NoError(t, repo.r.SetReference(absFeatureRef, initialCommitID))

		targetCommitIDs := common.AddNTestCommitsToSpecifiedRef(t, r, absTargetRef, 1, gpgKeyBytes)
		targetHeadCommitID := targetCommitIDs[0]
		require.NoError(t, repo.RecordRSLEntryForReference(testCtx, targetRef, false, rslopts.WithRecordLocalOnly()))

		featureCommitIDs := common.AddNTestCommitsToSpecifiedRef(t, r, absFeatureRef, 2, gpgKeyBytes)
		featureHeadCommitID := featureCommitIDs[1]
		require.NoError(t, repo.RecordRSLEntryForReference(testCtx, featureRef, false, rslopts.WithRecordLocalOnly()))

		mergeTreeID, err := r.GetMergeTree(targetHeadCommitID, featureHeadCommitID)
		require.NoError(t, err)
		mergeCommitID := createTestMergeCommit(t, testDir, mergeTreeID, targetHeadCommitID, featureHeadCommitID, "Merge feature into main")
		require.NoError(t, r.SetReference(absTargetRef, mergeCommitID))

		server := newTestGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
			requests.Add(1)
			requests.Add(1)
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/owner/repo/pulls/1":
				writeJSONResponse(t, w, &gogithub.PullRequest{
					ID: gogithub.Int64(1),
					Base: &gogithub.PullRequestBranch{
						Ref: gogithub.String("main"),
						SHA: gogithub.String(targetHeadCommitID.String()),
					},
					Head: &gogithub.PullRequestBranch{
						Ref: gogithub.String("feature"),
						SHA: gogithub.String(featureHeadCommitID.String()),
					},
					MergedAt:       &gogithub.Timestamp{Time: time.Now()},
					MergeCommitSHA: gogithub.String(mergeCommitID.String()),
				})
			case r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/owner/repo/pulls/1/reviews/123":
				writeJSONResponse(t, w, &gogithub.PullRequestReview{ID: gogithub.Int64(123)})
			case r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/owner/repo/git/ref/heads/main":
				writeJSONResponse(t, w, &gogithub.Reference{
					Ref:    gogithub.String("refs/heads/main"),
					Object: &gogithub.GitObject{SHA: gogithub.String(targetHeadCommitID.String())},
				})
			case r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/owner/repo/git/commits/"+mergeCommitID.String():
				writeJSONResponse(t, w, &gogithub.Commit{
					SHA:  gogithub.String(mergeCommitID.String()),
					Tree: &gogithub.Tree{SHA: gogithub.String(mergeTreeID.String())},
				})
			default:
				http.NotFound(w, r)
			}
		})

		signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

		opts := append(testGitHubOptions(server.URL), githubopts.WithRSLEntry())
		err = repo.AddGitHubPullRequestAttestationForNumber(testCtx, signer, "owner", "repo", 1, false, opts...)
		require.NoError(t, err)
		err = repo.AddGitHubPullRequestApprover(testCtx, signer, "owner", "repo", 1, 123, "bob", false, opts...)
		require.NoError(t, err)

		currentAttestations, err := attestations.LoadCurrentAttestations(repo.r)
		require.NoError(t, err)

		// Cached review details must make zero HTTP requests.
		requests.Store(0)
		baseRef, fromID, toID, err := repo.getGitHubPullRequestReviewDetails(testCtx, currentAttestations, server.URL, "test-token", "owner", "repo", 1, 123, true)
		require.NoError(t, err)

		assert.Zero(t, requests.Load())
		assert.Equal(t, "refs/heads/main", baseRef)
		assert.Equal(t, toID, mergeTreeID.String())
		assert.Equal(t, fromID, targetHeadCommitID.String())
	})

	t.Run("review is not in attestations state and useGitHubAPI is true", func(t *testing.T) {
		testDir := t.TempDir()
		r := gitinterface.CreateTestGitRepository(t, testDir, false)
		repo := &Repository{r: newStorer(r)}

		targetRef := "main"
		absTargetRef := "refs/heads/main"
		featureRef := "feature"
		absFeatureRef := "refs/heads/feature"

		treeBuilder := gitinterface.NewTreeBuilder(repo.GetGitRepository())
		emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
		require.NoError(t, err)
		initialCommitID, err := repo.r.Commit(emptyTreeID, absTargetRef, "Initial commit\n", false)
		require.NoError(t, err)
		require.NoError(t, repo.r.SetReference(absFeatureRef, initialCommitID))

		targetCommitIDs := common.AddNTestCommitsToSpecifiedRef(t, r, absTargetRef, 1, gpgKeyBytes)
		targetHeadCommitID := targetCommitIDs[0]
		require.NoError(t, repo.RecordRSLEntryForReference(testCtx, targetRef, false, rslopts.WithRecordLocalOnly()))

		featureCommitIDs := common.AddNTestCommitsToSpecifiedRef(t, r, absFeatureRef, 2, gpgKeyBytes)
		featureHeadCommitID := featureCommitIDs[1]
		require.NoError(t, repo.RecordRSLEntryForReference(testCtx, featureRef, false, rslopts.WithRecordLocalOnly()))

		mergeTreeID, err := r.GetMergeTree(targetHeadCommitID, featureHeadCommitID)
		require.NoError(t, err)
		mergeCommitID := createTestMergeCommit(t, testDir, mergeTreeID, targetHeadCommitID, featureHeadCommitID, "Merge feature into main")
		require.NoError(t, r.SetReference(absTargetRef, mergeCommitID))

		server := newTestGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/owner/repo/pulls/1":
				writeJSONResponse(t, w, &gogithub.PullRequest{
					ID: gogithub.Int64(1),
					Base: &gogithub.PullRequestBranch{
						Ref: gogithub.String("main"),
						SHA: gogithub.String(initialCommitID.String()),
					},
					Head: &gogithub.PullRequestBranch{
						Ref: gogithub.String("feature"),
						SHA: gogithub.String(featureHeadCommitID.String()),
					},
					MergedAt:       &gogithub.Timestamp{Time: time.Now()},
					MergeCommitSHA: gogithub.String(mergeCommitID.String()),
				})
			case r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/owner/repo/pulls/1/reviews/123":
				writeJSONResponse(t, w, &gogithub.PullRequestReview{ID: gogithub.Int64(123)})
			case r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/owner/repo/git/ref/heads/main":
				writeJSONResponse(t, w, &gogithub.Reference{
					Ref:    gogithub.String("refs/heads/main"),
					Object: &gogithub.GitObject{SHA: gogithub.String(targetHeadCommitID.String())},
				})
			case r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/owner/repo/git/commits/"+mergeCommitID.String():
				writeJSONResponse(t, w, &gogithub.Commit{
					SHA:  gogithub.String(mergeCommitID.String()),
					Tree: &gogithub.Tree{SHA: gogithub.String(mergeTreeID.String())},
				})
			default:
				http.NotFound(w, r)
			}
		})

		currentAttestations, err := attestations.LoadCurrentAttestations(repo.r)
		require.NoError(t, err)

		baseRef, fromID, toID, err := repo.getGitHubPullRequestReviewDetails(testCtx, currentAttestations, server.URL, "test-token", "owner", "repo", 1, 123, true)
		require.NoError(t, err)

		assert.Equal(t, "refs/heads/main", baseRef)
		assert.Equal(t, toID, mergeTreeID.String())
		assert.Equal(t, fromID, targetHeadCommitID.String())
	})

	t.Run("review is not in attestations state and useGitHubAPI is false but head hash is not locally found", func(t *testing.T) {
		remoteDir := t.TempDir()
		remoteR := gitinterface.CreateTestGitRepository(t, remoteDir, false)
		remoteRepo := &Repository{r: newStorer(remoteR)}

		localDir := t.TempDir()
		localR := gitinterface.CreateTestGitRepository(t, localDir, false)
		localRepo := &Repository{r: newStorer(localR)}

		targetRef := "main"
		absTargetRef := "refs/heads/main"
		featureRef := "feature"
		absFeatureRef := "refs/heads/feature"

		remoteTreeBuilder := gitinterface.NewTreeBuilder(remoteRepo.GetGitRepository())
		emptyRemoteTreeID, err := remoteTreeBuilder.WriteTreeFromEntries(nil)
		require.NoError(t, err)
		initialCommitID, err := remoteRepo.r.Commit(emptyRemoteTreeID, absTargetRef, "Initial commit\n", false)
		require.NoError(t, err)
		require.NoError(t, remoteRepo.r.SetReference(absFeatureRef, initialCommitID))

		targetCommitIDs := common.AddNTestCommitsToSpecifiedRef(t, remoteR, absTargetRef, 1, gpgKeyBytes)
		targetHeadCommitID := targetCommitIDs[0]
		require.NoError(t, remoteRepo.RecordRSLEntryForReference(testCtx, targetRef, false, rslopts.WithRecordLocalOnly()))

		require.NoError(t, localRepo.r.FetchObject(remoteDir, targetHeadCommitID))
		require.NoError(t, localRepo.r.SetReference(absTargetRef, targetHeadCommitID))
		require.NoError(t, localRepo.r.FetchObject(remoteDir, initialCommitID))
		require.NoError(t, localRepo.r.SetReference(absFeatureRef, initialCommitID))
		require.NoError(t, localRepo.RecordRSLEntryForReference(testCtx, targetRef, false, rslopts.WithRecordLocalOnly()))
		require.NoError(t, localRepo.RecordRSLEntryForReference(testCtx, featureRef, false, rslopts.WithRecordLocalOnly()))

		featureCommitIDs := common.AddNTestCommitsToSpecifiedRef(t, remoteR, absFeatureRef, 1, gpgKeyBytes)
		featureHeadCommitID := featureCommitIDs[0]
		require.NoError(t, remoteRepo.RecordRSLEntryForReference(testCtx, featureRef, false, rslopts.WithRecordLocalOnly()))

		mergeTreeID, err := remoteR.GetMergeTree(targetHeadCommitID, featureHeadCommitID)
		require.NoError(t, err)

		server := newTestGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/owner/repo/pulls/1":
				writeJSONResponse(t, w, &gogithub.PullRequest{
					ID: gogithub.Int64(1),
					Base: &gogithub.PullRequestBranch{
						Ref: gogithub.String("main"),
						SHA: gogithub.String(initialCommitID.String()),
					},
					Head: &gogithub.PullRequestBranch{
						Ref: gogithub.String("feature"),
						SHA: gogithub.String(featureHeadCommitID.String()),
						Repo: &gogithub.Repository{
							CloneURL: gogithub.String(remoteDir),
						},
					},
					MergedAt: &gogithub.Timestamp{Time: time.Now()},
				})
			case r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/owner/repo/pulls/1/reviews/123":
				writeJSONResponse(t, w, &gogithub.PullRequestReview{ID: gogithub.Int64(123)})
			default:
				http.NotFound(w, r)
			}
		})

		currentAttestations, err := attestations.LoadCurrentAttestations(localRepo.r)
		require.NoError(t, err)

		baseRef, fromID, toID, err := localRepo.getGitHubPullRequestReviewDetails(testCtx, currentAttestations, server.URL, "test-token", "owner", "repo", 1, 123, false)
		require.NoError(t, err)

		assert.Equal(t, "refs/heads/main", baseRef)
		assert.Equal(t, toID, mergeTreeID.String())
		assert.Equal(t, fromID, targetHeadCommitID.String())
	})
}

type staticTokenSource string

func (s staticTokenSource) Token(_ context.Context) (string, error) {
	return string(s), nil
}

func testGitHubOptions(baseURL string) []githubopts.Option {
	return []githubopts.Option{
		githubopts.WithGitHubBaseURL(baseURL),
		githubopts.WithGitHubTokenSource(staticTokenSource("test-token")),
	}
}

func newTestGitHubServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}

func writeJSONResponse(t *testing.T, w http.ResponseWriter, payload any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(payload))
}

func assertStoredGitHubPullRequest(t *testing.T, repo *Repository, verifier dsse.Verifier, mergedCommitID string) {
	t.Helper()
	commitID, err := repo.r.GetReference(attestations.Ref)
	require.NoError(t, err)
	commitInfo, err := repo.r.GetCommitInfo(commitID)
	require.NoError(t, err)
	files, err := repo.r.GetAllFilesInTree(commitInfo.TreeID)
	require.NoError(t, err)
	attestationPath := path.Join("github-pull-requests", attestations.GitHubPullRequestAttestationPath("exampleorg-42/refs/heads/main", mergedCommitID))
	require.Contains(t, files, attestationPath)
	blob, err := repo.r.ReadBlob(files[attestationPath])
	require.NoError(t, err)
	envelope := &dsse.Envelope{}
	require.NoError(t, json.Unmarshal(blob, envelope))
	accepted, err := signingdsse.VerifyEnvelope(testCtx, envelope, []dsse.Verifier{verifier}, 1)
	require.NoError(t, err)
	require.Len(t, accepted, 1)
	payload, err := envelope.DecodeB64Payload()
	require.NoError(t, err)
	statement := &ita.Statement{}
	require.NoError(t, json.Unmarshal(payload, statement))
	assert.Equal(t, ita.StatementTypeUri, statement.GetType())
	assert.Equal(t, githubv01.PullRequestPredicateType, statement.GetPredicateType())
	require.Len(t, statement.GetSubject(), 1)
	assert.Equal(t, "https://github.com/exampleorg/example/pull/1", statement.GetSubject()[0].GetUri())
	assert.Equal(t, map[string]string{"gitCommit": mergedCommitID}, statement.GetSubject()[0].GetDigest())
	predicate := statement.GetPredicate().AsMap()
	assert.Equal(t, float64(1), predicate["number"])
	assert.Equal(t, mergedCommitID, predicate["merge_commit_sha"])
	base, ok := predicate["base"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "main", base["ref"])
	assert.Equal(t, "a", base["sha"])
}

func createTestMergeCommit(t *testing.T, repoDir string, treeID githash.Hash, parent1, parent2 githash.Hash, message string) githash.Hash {
	t.Helper()
	cmd := exec.Command("git", "-C", repoDir, "commit-tree", "-m", message, "-p", parent1.String(), "-p", parent2.String(), treeID.String())
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test Author",
		"GIT_AUTHOR_EMAIL=author@example.com",
		"GIT_COMMITTER_NAME=Test Committer",
		"GIT_COMMITTER_EMAIL=committer@example.com",
	)
	out, err := cmd.Output()
	require.NoError(t, err)
	hash, err := githash.NewHash(strings.TrimSpace(string(out)))
	require.NoError(t, err)
	return hash
}
