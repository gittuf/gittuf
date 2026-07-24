// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"testing"

	"github.com/go-git/go-git/v6/plumbing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplaceRefsDoNotAffectVerificationReads(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	repo := CreateTestGitRepository(t, tmpDir, false)
	treeBuilder := NewTreeBuilder(repo)

	allowedV1, err := repo.WriteBlob([]byte("allowed v1"))
	require.NoError(t, err)
	allowedV2, err := repo.WriteBlob([]byte("allowed v2"))
	require.NoError(t, err)
	secretBlob, err := repo.WriteBlob([]byte("secret payload"))
	require.NoError(t, err)

	// base:  { allowed: v1 }
	// true:  { allowed: v1, secret }   -> the true commit changes protected "secret"
	// decoy: { allowed: v2 }           -> the decoy changes only the allowed path
	baseTree, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("allowed", allowedV1)})
	require.NoError(t, err)
	trueTree, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("allowed", allowedV1), NewEntryBlob("secret", secretBlob)})
	require.NoError(t, err)
	decoyTree, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("allowed", allowedV2)})
	require.NoError(t, err)

	mainRef := testNameToRefName(t.Name())
	baseCommit, err := repo.Commit(baseTree, mainRef, "base\n", false)
	require.NoError(t, err)

	trueCommit, err := repo.Commit(trueTree, mainRef, "TRUE: adds secret\n", false)
	require.NoError(t, err)

	// Give the decoy the same parent as the true commit so the diffs are
	// directly comparable.
	decoyRef := mainRef + "-decoy"
	require.NoError(t, repo.SetReference(decoyRef, baseCommit))
	decoyCommit, err := repo.Commit(decoyTree, decoyRef, "DECOY: benign change\n", false)
	require.NoError(t, err)

	// Sanity: before any replacement, gittuf reads the true commit correctly.
	paths, err := repo.GetFilePathsChangedByCommit(trueCommit)
	require.NoError(t, err)
	require.Equal(t, []string{"secret"}, paths, "test setup: true commit should change only 'secret'")

	// Plant the malicious replacement: refs/replace/<trueCommit> -> <decoyCommit>.
	_, err = repo.executor("replace", trueCommit.String(), decoyCommit.String()).executeString()
	require.NoError(t, err, "unable to create replace ref")

	// The signature path (go-git) is replace-blind and sees the true
	// object.
	goGitRepo, err := repo.GetGoGitRepository()
	require.NoError(t, err)
	goGitCommit, err := goGitRepo.CommitObject(plumbing.NewHash(trueCommit.String()))
	require.NoError(t, err)
	require.Equal(t, trueTree.String(), goGitCommit.TreeHash.String(),
		"go-git (signature path) must read the true object")

	gotTree, err := repo.GetCommitTreeID(trueCommit)
	require.NoError(t, err)
	assert.Equal(t, trueTree.String(), gotTree.String(),
		"refs/replace/ must not alter the tree gittuf verifies")

	gotPaths, err := repo.GetFilePathsChangedByCommit(trueCommit)
	require.NoError(t, err)
	assert.Equal(t, []string{"secret"}, gotPaths,
		"refs/replace/ must not hide the protected-path change from file-policy verification")

	gotMessage, err := repo.GetCommitMessage(trueCommit)
	require.NoError(t, err)
	assert.Equal(t, "TRUE: adds secret", gotMessage,
		"refs/replace/ must not alter the commit message gittuf reads")
}
