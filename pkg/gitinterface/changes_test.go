// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetFilePathsChangedByCommitRepository(t *testing.T) {
	tmpDir := t.TempDir()
	repo := CreateTestGitRepository(t, tmpDir, false)

	treeBuilder := NewTreeBuilder(repo)

	blobIDs := []Hash{}
	for i := 0; i < 3; i++ {
		blobID, err := repo.WriteBlob([]byte(fmt.Sprintf("%d", i)))
		require.NoError(t, err)
		blobIDs = append(blobIDs, blobID)
	}

	emptyTree, err := treeBuilder.WriteTreeFromEntries(nil)
	require.NoError(t, err)

	tests := map[string]struct {
		input  []string
		output []string
	}{
		"ASCII": {
			input:  []string{"a", "b", "c"},
			output: []string{"a", "b", "c"},
		},
		"non-ASCII": {
			input:  []string{"acafé.txt", "bcafé.txt", "ccafé.txt"},
			output: []string{"acafé.txt", "bcafé.txt", "ccafé.txt"},
		},
		"tab": {
			input:  []string{"a\tb.txt", "b\tb.txt", "c\tb.txt"},
			output: []string{"a\tb.txt", "b\tb.txt", "c\tb.txt"},
		},
		"double quote": {
			input:  []string{`a"b.txt`, `b"b.txt`, `c"b.txt`},
			output: []string{`a"b.txt`, `b"b.txt`, `c"b.txt`},
		},
		"backslash": {
			input:  []string{`a\ba`, `b\bb`, `c\bc`},
			output: []string{`a\ba`, `b\bb`, `c\bc`},
		},
		"space": {
			input:  []string{"a a", "b b", "c c"},
			output: []string{"a a", "b b", "c c"},
		},
		"leading space": {
			input:  []string{" a", " b", " c"},
			output: []string{" a", " b", " c"},
		},
	}

	// In each of the tests below, repo.Commit uses the test name as a ref
	// This allows us to use a single repo in all the tests without interference
	// For example, if we use a single repo and a single ref (say main), the test that
	// expects a commit with no parents will have a parent because of a commit created
	// in a previous test

	for name, test := range tests {
		t.Run("modify single file "+name, func(t *testing.T) {
			treeA, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob(test.input[0], blobIDs[0])})
			require.NoError(t, err)

			treeB, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob(test.input[0], blobIDs[1])})
			require.NoError(t, err)

			_, err = repo.Commit(treeA, testNameToRefName(t.Name()), "Test commit\n", false)
			require.NoError(t, err)

			cB, err := repo.Commit(treeB, testNameToRefName(t.Name()), "Test commit\n", false)
			require.NoError(t, err)

			diffs, err := repo.GetFilePathsChangedByCommit(cB)
			assert.NoError(t, err)
			assert.Equal(t, []string{test.output[0]}, diffs)
		})

		t.Run("rename single file "+name, func(t *testing.T) {
			treeA, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob(test.input[0], blobIDs[0])})
			require.NoError(t, err)

			treeB, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob(test.input[1], blobIDs[0])})
			require.NoError(t, err)

			_, err = repo.Commit(treeA, testNameToRefName(t.Name()), "Test commit\n", false)
			require.NoError(t, err)

			cB, err := repo.Commit(treeB, testNameToRefName(t.Name()), "Test commit\n", false)
			require.NoError(t, err)

			diffs, err := repo.GetFilePathsChangedByCommit(cB)
			assert.NoError(t, err)
			assert.Equal(t, []string{test.output[0], test.output[1]}, diffs)
		})

		t.Run("swap two files around "+name, func(t *testing.T) {
			treeA, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob(test.input[0], blobIDs[0]), NewEntryBlob(test.input[1], blobIDs[1])})
			require.NoError(t, err)

			treeB, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob(test.input[0], blobIDs[1]), NewEntryBlob(test.input[1], blobIDs[0])})
			require.NoError(t, err)

			_, err = repo.Commit(treeA, testNameToRefName(t.Name()), "Test commit\n", false)
			require.NoError(t, err)

			cB, err := repo.Commit(treeB, testNameToRefName(t.Name()), "Test commit\n", false)
			require.NoError(t, err)

			diffs, err := repo.GetFilePathsChangedByCommit(cB)
			assert.NoError(t, err)
			assert.Equal(t, []string{test.output[0], test.output[1]}, diffs)
		})

		t.Run("create new file "+name, func(t *testing.T) {
			treeA, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob(test.input[0], blobIDs[0])})
			require.NoError(t, err)

			treeB, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob(test.input[0], blobIDs[0]), NewEntryBlob(test.input[1], blobIDs[1])})
			require.NoError(t, err)

			_, err = repo.Commit(treeA, testNameToRefName(t.Name()), "Test commit\n", false)
			require.NoError(t, err)

			cB, err := repo.Commit(treeB, testNameToRefName(t.Name()), "Test commit\n", false)
			require.NoError(t, err)

			diffs, err := repo.GetFilePathsChangedByCommit(cB)
			assert.NoError(t, err)
			assert.Equal(t, []string{test.output[1]}, diffs)
		})

		t.Run("delete file "+name, func(t *testing.T) {
			treeA, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob(test.input[0], blobIDs[0]), NewEntryBlob(test.input[1], blobIDs[1])})
			require.NoError(t, err)

			treeB, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob(test.input[0], blobIDs[0])})
			require.NoError(t, err)

			_, err = repo.Commit(treeA, testNameToRefName(t.Name()), "Test commit\n", false)
			require.NoError(t, err)

			cB, err := repo.Commit(treeB, testNameToRefName(t.Name()), "Test commit\n", false)
			require.NoError(t, err)

			diffs, err := repo.GetFilePathsChangedByCommit(cB)
			assert.NoError(t, err)
			assert.Equal(t, []string{test.output[1]}, diffs)
		})

		t.Run("modify file and create new file "+name, func(t *testing.T) {
			treeA, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob(test.input[0], blobIDs[0])})
			require.NoError(t, err)

			treeB, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob(test.input[0], blobIDs[2]), NewEntryBlob(test.input[1], blobIDs[1])})
			require.NoError(t, err)

			_, err = repo.Commit(treeA, testNameToRefName(t.Name()), "Test commit\n", false)
			require.NoError(t, err)

			cB, err := repo.Commit(treeB, testNameToRefName(t.Name()), "Test commit\n", false)
			require.NoError(t, err)

			diffs, err := repo.GetFilePathsChangedByCommit(cB)
			assert.NoError(t, err)
			assert.Equal(t, []string{test.output[0], test.output[1]}, diffs)
		})

		t.Run("no parent "+name, func(t *testing.T) {
			treeA, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob(test.input[0], blobIDs[0])})
			if err != nil {
				t.Fatal(err)
			}

			cA, err := repo.Commit(treeA, testNameToRefName(t.Name()), "Test commit\n", false)
			if err != nil {
				t.Fatal(err)
			}

			diffs, err := repo.GetFilePathsChangedByCommit(cA)
			assert.NoError(t, err)
			assert.Equal(t, []string{test.output[0]}, diffs)
		})

		t.Run("merge commit with commit matching parent "+name, func(t *testing.T) {
			treeA, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob(test.input[0], blobIDs[0])})
			require.NoError(t, err)

			treeB, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob(test.input[0], blobIDs[1])})
			require.NoError(t, err)

			mainBranch := testNameToRefName(t.Name())
			featureBranch := testNameToRefName(t.Name() + " feature branch")

			// Write common commit for both branches
			cCommon, err := repo.Commit(emptyTree, mainBranch, "Initial commit\n", false)
			require.NoError(t, err)
			if err := repo.SetReference(featureBranch, cCommon); err != nil {
				t.Fatal(err)
			}

			cA, err := repo.Commit(treeA, mainBranch, "Test commit\n", false)
			require.NoError(t, err)

			cB, err := repo.Commit(treeB, featureBranch, "Test commit\n", false)
			require.NoError(t, err)

			// Create a merge commit with two parents
			cM := repo.commitWithParents(t, treeB, []Hash{cA, cB}, "Merge commit\n", false)

			diffs, err := repo.GetFilePathsChangedByCommit(cM)
			assert.NoError(t, err)
			assert.Nil(t, diffs)
		})

		t.Run("merge commit with no matching parent "+name, func(t *testing.T) {
			treeA, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob(test.input[0], blobIDs[0])})
			require.NoError(t, err)

			treeB, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob(test.input[1], blobIDs[1])})
			require.NoError(t, err)

			treeC, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob(test.input[2], blobIDs[2])})
			require.NoError(t, err)

			mainBranch := testNameToRefName(t.Name())
			featureBranch := testNameToRefName(t.Name() + " feature branch")

			// Write common commit for both branches
			cCommon, err := repo.Commit(emptyTree, mainBranch, "Initial commit\n", false)
			require.NoError(t, err)
			if err := repo.SetReference(featureBranch, cCommon); err != nil {
				t.Fatal(err)
			}

			cA, err := repo.Commit(treeA, mainBranch, "Test commit\n", false)
			require.NoError(t, err)

			cB, err := repo.Commit(treeB, featureBranch, "Test commit\n", false)
			require.NoError(t, err)

			// Create a merge commit with two parents and a different tree
			cM := repo.commitWithParents(t, treeC, []Hash{cA, cB}, "Merge commit\n", false)

			diffs, err := repo.GetFilePathsChangedByCommit(cM)
			assert.NoError(t, err)
			assert.Equal(t, []string{test.output[0], test.output[1], test.output[2]}, diffs)
		})

		t.Run("merge commit with overlapping parent trees "+name, func(t *testing.T) {
			treeA, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob(test.input[0], blobIDs[0])})
			require.NoError(t, err)

			treeB, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob(test.input[0], blobIDs[1])})
			require.NoError(t, err)

			treeC, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob(test.input[0], blobIDs[2])})
			require.NoError(t, err)

			mainBranch := testNameToRefName(t.Name())
			featureBranch := testNameToRefName(t.Name() + " feature branch")

			// Write common commit for both branches
			cCommon, err := repo.Commit(emptyTree, mainBranch, "Initial commit\n", false)
			require.NoError(t, err)
			if err := repo.SetReference(featureBranch, cCommon); err != nil {
				t.Fatal(err)
			}

			cA, err := repo.Commit(treeA, mainBranch, "Test commit\n", false)
			require.NoError(t, err)

			cB, err := repo.Commit(treeB, featureBranch, "Test commit\n", false)
			require.NoError(t, err)

			// Create a merge commit with two parents and an overlapping tree
			cM := repo.commitWithParents(t, treeC, []Hash{cA, cB}, "Merge commit\n", false)

			diffs, err := repo.GetFilePathsChangedByCommit(cM)
			assert.NoError(t, err)
			assert.Equal(t, []string{test.output[0]}, diffs)
		})
	}

	t.Run("no parent, empty tree", func(t *testing.T) {
		cA, err := repo.Commit(emptyTree, testNameToRefName(t.Name()), "Test commit\n", false)
		require.NoError(t, err)

		diffs, err := repo.GetFilePathsChangedByCommit(cA)
		assert.NoError(t, err)
		assert.Nil(t, diffs)
	})

	t.Run("non-commit object", func(t *testing.T) {
		blobID, err := repo.WriteBlob([]byte("test"))
		require.Nil(t, err)

		_, err = repo.GetFilePathsChangedByCommit(blobID)
		assert.ErrorContains(t, err, "is it a commit?")
	})

	t.Run("single parent commit uses two git invocations", func(t *testing.T) {
		tree, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("a", blobIDs[0])})
		require.NoError(t, err)

		refName := testNameToRefName(t.Name())
		_, err = repo.Commit(emptyTree, refName, "Initial commit\n", false)
		require.NoError(t, err)
		commitID, err := repo.Commit(tree, refName, "Add a\n", false)
		require.NoError(t, err)

		before := repo.GitInvocationCount()
		diffs, err := repo.GetFilePathsChangedByCommit(commitID)
		require.NoError(t, err)
		assert.Equal(t, []string{"a"}, diffs)
		assert.Equal(t, uint64(2), repo.GitInvocationCount()-before)
	})
}
