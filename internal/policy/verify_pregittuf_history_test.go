// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"errors"
	"fmt"
	"sort"
	"testing"

	"github.com/gittuf/gittuf/internal/common"
	"github.com/gittuf/gittuf/pkg/githash"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/gittuf/gittuf/pkg/rsl"
	"github.com/stretchr/testify/assert"
)

// TestVerifyRefFullWithPreGittufHistory records how file rules are applied to
// Git history that predates gittuf (see #1609). When an RSL entry is the first
// for its ref, getCommits enumerates the ref's entire ancestry, so commits made
// before gittuf was set up are verified against the current policy.
//
// These tests capture current behavior as a baseline for a fix. Cases that are
// known to be incorrect today are marked with TODO(#1609). Cases that must
// continue to fail after any fix are called out explicitly: a ref's first RSL
// entry is not necessarily pre-gittuf, so new commits on it must still be
// verified.
//
// All cases use createTestStateWithPolicy, where files "1" and "2" are protected
// by a rule trusting only gpgKeyBytes.
func TestVerifyRefFullWithPreGittufHistory(t *testing.T) {
	mainRefName := "refs/heads/main"
	featureRefName := "refs/heads/feature"

	// TODO(#1609): this incorrectly fails verification. The unauthorized
	// commits predate gittuf, which makes no claims about pre-gittuf history.
	// This test documents current behavior as a regression baseline; update
	// the assertion once a fix lands.
	t.Run("pre-gittuf commits violating current rules, unsigned", func(t *testing.T) {
		repo := createTestRepositoryWithPreGittufHistory(t, createTestStateWithPolicy, func(repo *gitinterface.Repository) {
			addTestCommitModifyingPath(t, repo, mainRefName, "1", nil)
			addTestCommitModifyingPath(t, repo, mainRefName, "2", nil)
		})
		recordTestRSLEntryForRef(t, repo, mainRefName)

		_, err := NewPolicyVerifier(repo).VerifyRefFull(testCtx, mainRefName)
		assert.ErrorIs(t, err, ErrVerificationFailed)
	})

	// TODO(#1609): this incorrectly fails verification. The unauthorized
	// commits predate gittuf, which makes no claims about pre-gittuf history.
	// This test documents current behavior as a regression baseline; update
	// the assertion once a fix lands.
	t.Run("pre-gittuf commits violating current rules, signed by untrusted key", func(t *testing.T) {
		repo := createTestRepositoryWithPreGittufHistory(t, createTestStateWithPolicy, func(repo *gitinterface.Repository) {
			addTestCommitModifyingPath(t, repo, mainRefName, "1", gpgUnauthorizedKeyBytes)
			addTestCommitModifyingPath(t, repo, mainRefName, "2", gpgUnauthorizedKeyBytes)
		})
		recordTestRSLEntryForRef(t, repo, mainRefName)

		_, err := NewPolicyVerifier(repo).VerifyRefFull(testCtx, mainRefName)
		assert.ErrorIs(t, err, ErrVerificationFailed)
	})

	// TODO(#1609): this passes only because the pre-gittuf commits happen to
	// be signed by a key trusted in the current policy. getCommits still
	// returns (and verification checks) every pre-gittuf commit. This test
	// documents current behavior as a regression baseline; update the
	// expected commits once a fix lands.
	t.Run("pre-gittuf commits signed by key trusted in current policy", func(t *testing.T) {
		preGittufCommitIDs := []githash.Hash{}
		repo := createTestRepositoryWithPreGittufHistory(t, createTestStateWithPolicy, func(repo *gitinterface.Repository) {
			preGittufCommitIDs = append(preGittufCommitIDs, addTestCommitModifyingPath(t, repo, mainRefName, "1", gpgKeyBytes))
			preGittufCommitIDs = append(preGittufCommitIDs, addTestCommitModifyingPath(t, repo, mainRefName, "2", gpgKeyBytes))
		})
		entry := recordTestRSLEntryForRef(t, repo, mainRefName)

		_, err := NewPolicyVerifier(repo).VerifyRefFull(testCtx, mainRefName)
		assert.Nil(t, err)

		sort.Slice(preGittufCommitIDs, func(i, j int) bool {
			return preGittufCommitIDs[i].String() < preGittufCommitIDs[j].String()
		})
		commitIDs, err := getCommits(repo, entry)
		assert.Nil(t, err)
		assert.Equal(t, preGittufCommitIDs, commitIDs)
	})

	// TODO(#1609): this incorrectly fails verification. The unauthorized
	// commit predates gittuf, which makes no claims about pre-gittuf history.
	// This test documents current behavior as a regression baseline; update
	// the assertion once a fix lands.
	t.Run("pre-gittuf commits on another ref modifying protected path", func(t *testing.T) {
		devRefName := "refs/heads/dev"
		repo := createTestRepositoryWithPreGittufHistory(t, createTestStateWithPolicy, func(repo *gitinterface.Repository) {
			addTestCommitModifyingPath(t, repo, devRefName, "1", gpgUnauthorizedKeyBytes)
		})
		recordTestRSLEntryForRef(t, repo, devRefName)

		_, err := NewPolicyVerifier(repo).VerifyRefFull(testCtx, devRefName)
		assert.ErrorIs(t, err, ErrVerificationFailed)
	})

	t.Run("pre-gittuf commits violating current rules, verified from a later entry", func(t *testing.T) {
		repo := createTestRepositoryWithPreGittufHistory(t, createTestStateWithPolicy, func(repo *gitinterface.Repository) {
			addTestCommitModifyingPath(t, repo, mainRefName, "1", nil)
		})
		recordTestRSLEntryForRef(t, repo, mainRefName)

		addTestCommitModifyingPath(t, repo, mainRefName, "2", gpgKeyBytes)
		recordTestRSLEntryForRef(t, repo, mainRefName)

		// Only the latest entry is verified, so the pre-gittuf commit is not
		// checked
		_, err := NewPolicyVerifier(repo).VerifyRef(testCtx, mainRefName)
		assert.Nil(t, err)
	})

	t.Run("new branch after gittuf setup, authorized commits", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicy)

		addTestCommitModifyingPath(t, repo, mainRefName, "3", gpgKeyBytes)
		recordTestRSLEntryForRef(t, repo, mainRefName)

		createTestBranchFromRef(t, repo, mainRefName, featureRefName)
		addTestCommitModifyingPath(t, repo, featureRefName, "1", gpgKeyBytes)
		addTestCommitModifyingPath(t, repo, featureRefName, "2", gpgKeyBytes)
		recordTestRSLEntryForRef(t, repo, featureRefName)

		_, err := NewPolicyVerifier(repo).VerifyRefFull(testCtx, featureRefName)
		assert.Nil(t, err)
	})

	// This correctly fails today, and any fix for #1609 must keep it failing:
	// the branch's first RSL entry is not pre-gittuf, so its new commits must
	// still be verified.
	t.Run("new branch after gittuf setup, unauthorized commit", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicy)

		addTestCommitModifyingPath(t, repo, mainRefName, "3", gpgKeyBytes)
		recordTestRSLEntryForRef(t, repo, mainRefName)

		createTestBranchFromRef(t, repo, mainRefName, featureRefName)
		addTestCommitModifyingPath(t, repo, featureRefName, "2", gpgUnauthorizedKeyBytes)
		recordTestRSLEntryForRef(t, repo, featureRefName)

		_, err := NewPolicyVerifier(repo).VerifyRefFull(testCtx, featureRefName)
		assert.ErrorIs(t, err, ErrVerificationFailed)
	})

	// TODO(#1609): this incorrectly fails verification. The branch's own
	// commits are authorized; the failure comes from the unsigned pre-gittuf
	// commit it inherits from main. This test documents current behavior as a
	// regression baseline; update the assertion once a fix lands.
	t.Run("new branch after gittuf setup forked from pre-gittuf history, authorized commits", func(t *testing.T) {
		repo := createTestRepositoryWithPreGittufHistory(t, createTestStateWithPolicy, func(repo *gitinterface.Repository) {
			addTestCommitModifyingPath(t, repo, mainRefName, "1", nil)
		})
		recordTestRSLEntryForRef(t, repo, mainRefName)

		createTestBranchFromRef(t, repo, mainRefName, featureRefName)
		addTestCommitModifyingPath(t, repo, featureRefName, "2", gpgKeyBytes)
		recordTestRSLEntryForRef(t, repo, featureRefName)

		_, err := NewPolicyVerifier(repo).VerifyRefFull(testCtx, featureRefName)
		assert.ErrorIs(t, err, ErrVerificationFailed)
	})

	// TODO(#1609): this incorrectly fails verification. The unauthorized
	// commit predates gittuf and exists only on the side branch, which is
	// first recorded after gittuf setup. This test documents current behavior
	// as a regression baseline; update the assertion once a fix lands.
	t.Run("pre-gittuf side branch recorded after gittuf setup", func(t *testing.T) {
		releaseRefName := "refs/heads/release"
		repo := createTestRepositoryWithPreGittufHistory(t, createTestStateWithPolicy, func(repo *gitinterface.Repository) {
			addTestCommitModifyingPath(t, repo, mainRefName, "3", gpgKeyBytes)
			createTestBranchFromRef(t, repo, mainRefName, releaseRefName)
			addTestCommitModifyingPath(t, repo, releaseRefName, "1", gpgUnauthorizedKeyBytes)
		})
		recordTestRSLEntryForRef(t, repo, mainRefName)
		recordTestRSLEntryForRef(t, repo, releaseRefName)

		_, err := NewPolicyVerifier(repo).VerifyRefFull(testCtx, releaseRefName)
		assert.ErrorIs(t, err, ErrVerificationFailed)
	})

	// This correctly fails today, and any fix for #1609 must keep it failing:
	// the first entry contains pre-gittuf history, but also a commit made
	// after gittuf setup that must still be verified.
	t.Run("first entry with pre-gittuf history and new unauthorized commit", func(t *testing.T) {
		repo := createTestRepositoryWithPreGittufHistory(t, createTestStateWithPolicy, func(repo *gitinterface.Repository) {
			addTestCommitModifyingPath(t, repo, mainRefName, "3", nil)
		})

		addTestCommitModifyingPath(t, repo, mainRefName, "1", gpgUnauthorizedKeyBytes)
		recordTestRSLEntryForRef(t, repo, mainRefName)

		_, err := NewPolicyVerifier(repo).VerifyRefFull(testCtx, mainRefName)
		assert.ErrorIs(t, err, ErrVerificationFailed)
	})

	// This correctly fails today, and any fix for #1609 must keep it failing:
	// the RSL records entries regardless of whether they pass verification,
	// so an unauthorized commit already recorded for one ref must not be
	// trusted when it is recorded for another.
	t.Run("unauthorized commit recorded for one new ref, then for another", func(t *testing.T) {
		otherRefName := "refs/heads/other"
		repo, _ := createTestRepository(t, createTestStateWithPolicy)

		addTestCommitModifyingPath(t, repo, mainRefName, "3", gpgKeyBytes)
		recordTestRSLEntryForRef(t, repo, mainRefName)

		createTestBranchFromRef(t, repo, mainRefName, featureRefName)
		addTestCommitModifyingPath(t, repo, featureRefName, "2", gpgUnauthorizedKeyBytes)
		recordTestRSLEntryForRef(t, repo, featureRefName)

		createTestBranchFromRef(t, repo, featureRefName, otherRefName)
		recordTestRSLEntryForRef(t, repo, otherRefName)

		_, err := NewPolicyVerifier(repo).VerifyRefFull(testCtx, otherRefName)
		assert.ErrorIs(t, err, ErrVerificationFailed)
	})
}

// recordTestRSLEntryForRef creates an RSL reference entry for refName's
// current tip.
func recordTestRSLEntryForRef(t *testing.T, repo *gitinterface.Repository, refName string) *rsl.ReferenceEntry {
	t.Helper()

	tipID, err := repo.GetReference(refName)
	if err != nil {
		t.Fatal(err)
	}

	entry := rsl.NewReferenceEntry(refName, tipID)
	entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
	entry.ID = entryID

	return entry
}

// createTestBranchFromRef creates newRefName pointing to fromRefName's tip.
func createTestBranchFromRef(t *testing.T, repo *gitinterface.Repository, fromRefName, newRefName string) {
	t.Helper()

	tipID, err := repo.GetReference(fromRefName)
	if err != nil {
		t.Fatal(err)
	}

	if err := repo.SetReference(newRefName, tipID); err != nil {
		t.Fatal(err)
	}
}

// createTestRepositoryWithPreGittufHistory is like createTestRepository, but
// createHistory is invoked on the new Git repository before the policy is
// committed and applied, so the commits it creates predate gittuf.
func createTestRepositoryWithPreGittufHistory(t *testing.T, stateCreator func(*testing.T) *State, createHistory func(*gitinterface.Repository)) *gitinterface.Repository {
	t.Helper()

	state := stateCreator(t)

	tempDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
	state.repository = repo

	createHistory(repo)

	if err := state.Commit(repo, "Create test state", true, false); err != nil {
		t.Fatal(err)
	}
	if err := Apply(testCtx, repo, false); err != nil {
		t.Fatal(err)
	}

	return repo
}

// addTestCommitModifyingPath adds a commit to refName that modifies only the
// top-level file at path, keeping all other files from the parent commit. The
// commit is signed using signingKeyBytes, or left unsigned if signingKeyBytes
// is nil.
func addTestCommitModifyingPath(t *testing.T, repo *gitinterface.Repository, refName, path string, signingKeyBytes []byte) githash.Hash {
	t.Helper()

	files := map[string]githash.Hash{}
	parentID, err := repo.GetReference(refName)
	if err != nil {
		if !errors.Is(err, gitinterface.ErrReferenceNotFound) {
			t.Fatal(err)
		}
	} else {
		parentTreeID, err := repo.GetCommitTreeID(parentID)
		if err != nil {
			t.Fatal(err)
		}
		files, err = repo.GetAllFilesInTree(parentTreeID)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Derive the contents from the parent so that the file changes in every
	// commit
	blobID, err := repo.WriteBlob([]byte(fmt.Sprintf("%s:%s", parentID.String(), path)))
	if err != nil {
		t.Fatal(err)
	}
	files[path] = blobID

	entries := make([]gitinterface.TreeEntry, 0, len(files))
	for name, id := range files {
		entries = append(entries, gitinterface.NewEntryBlob(name, id))
	}
	treeID, err := gitinterface.NewTreeBuilder(repo).WriteTreeFromEntries(entries)
	if err != nil {
		t.Fatal(err)
	}

	var commitID githash.Hash
	if signingKeyBytes == nil {
		commitID, err = repo.Commit(treeID, refName, "Test commit\n", false)
	} else {
		commitID, err = repo.CommitUsingSpecificKey(treeID, refName, "Test commit\n", signingKeyBytes)
	}
	if err != nil {
		t.Fatal(err)
	}

	return commitID
}
