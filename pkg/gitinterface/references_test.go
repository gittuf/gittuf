// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetReference(t *testing.T) {
	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir, false)

	refName := "refs/heads/main"
	treeBuilder := NewTreeBuilder(repo)

	// Write empty tree
	emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
	if err != nil {
		t.Fatal(err)
	}

	commitID, err := repo.Commit(emptyTreeID, refName, "Initial commit\n", false)
	require.Nil(t, err)

	refTip, err := repo.GetReference(refName)
	assert.Nil(t, err)
	assert.Equal(t, commitID, refTip)

	t.Run("reference not found", func(t *testing.T) {
		_, err := repo.GetReference("refs/heads/nonexistent")
		assert.ErrorIs(t, err, ErrReferenceNotFound)
	})
}

func TestSetReference(t *testing.T) {
	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir, false)

	refName := "refs/heads/main"
	treeBuilder := NewTreeBuilder(repo)

	// Write empty tree
	emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
	if err != nil {
		t.Fatal(err)
	}

	firstCommitID, err := repo.Commit(emptyTreeID, refName, "Initial commit\n", false)
	require.Nil(t, err)

	// Create second commit with tree
	secondCommitID, err := repo.Commit(emptyTreeID, refName, "Add README\n", false)
	require.Nil(t, err)

	refTip, err := repo.GetReference(refName)
	require.Nil(t, err)
	require.Equal(t, secondCommitID, refTip)

	err = repo.SetReference(refName, firstCommitID)
	assert.Nil(t, err)

	refTip, err = repo.GetReference(refName)
	require.Nil(t, err)
	assert.Equal(t, firstCommitID, refTip)

	t.Run("set new reference", func(t *testing.T) {
		newRefName := "refs/heads/feature"
		err = repo.SetReference(newRefName, firstCommitID)
		assert.Nil(t, err)

		refTip, err := repo.GetReference(newRefName)
		assert.Nil(t, err)
		assert.Equal(t, firstCommitID, refTip)
	})
}

func TestCheckAndSetReference(t *testing.T) {
	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir, false)

	refName := "refs/heads/main"
	treeBuilder := NewTreeBuilder(repo)

	// Write empty tree
	emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
	if err != nil {
		t.Fatal(err)
	}

	firstCommitID, err := repo.Commit(emptyTreeID, refName, "Initial commit\n", false)
	require.Nil(t, err)

	// Create second commit with tree
	secondCommitID, err := repo.Commit(emptyTreeID, refName, "Add README\n", false)
	require.Nil(t, err)

	refTip, err := repo.GetReference(refName)
	require.Nil(t, err)
	require.Equal(t, secondCommitID, refTip)

	err = repo.CheckAndSetReference(refName, firstCommitID, secondCommitID)
	assert.Nil(t, err)

	refTip, err = repo.GetReference(refName)
	require.Nil(t, err)
	assert.Equal(t, firstCommitID, refTip)

	t.Run("error when expected value doesn't match", func(t *testing.T) {
		err = repo.CheckAndSetReference(refName, secondCommitID, secondCommitID)
		assert.NotNil(t, err)

		refTip, err := repo.GetReference(refName)
		require.Nil(t, err)
		assert.Equal(t, firstCommitID, refTip)
	})
}

func TestGetSymbolicReferenceTarget(t *testing.T) {
	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir, false)

	refName := "refs/heads/main"
	treeBuilder := NewTreeBuilder(repo)

	// Write empty tree
	emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = repo.Commit(emptyTreeID, refName, "Initial commit\n", false)
	require.Nil(t, err)

	// HEAD must be set to the main branch -> this is handled by git init
	head, err := repo.GetSymbolicReferenceTarget("HEAD")
	assert.Nil(t, err)
	assert.Equal(t, refName, head)

	t.Run("error with non-symbolic ref", func(t *testing.T) {
		_, err = repo.GetSymbolicReferenceTarget("refs/heads/main")
		assert.NotNil(t, err)
	})
}

func TestSetSymbolicReference(t *testing.T) {
	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir, false)

	refName := "refs/heads/not-main" // we want to ensure it's set to something other than the default main
	treeBuilder := NewTreeBuilder(repo)

	// Write empty tree
	emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = repo.Commit(emptyTreeID, refName, "Initial commit\n", false)
	require.Nil(t, err)

	head, err := repo.GetSymbolicReferenceTarget("HEAD")
	require.Nil(t, err)
	assert.Equal(t, "refs/heads/main", head)

	err = repo.SetSymbolicReference("HEAD", refName)
	assert.Nil(t, err)

	head, err = repo.GetSymbolicReferenceTarget("HEAD")
	require.Nil(t, err)
	assert.Equal(t, refName, head) // not main anymore

	t.Run("create new symbolic reference", func(t *testing.T) {
		symRefName := "refs/heads/current"
		err = repo.SetSymbolicReference(symRefName, refName)
		assert.Nil(t, err)

		target, err := repo.GetSymbolicReferenceTarget(symRefName)
		assert.Nil(t, err)
		assert.Equal(t, refName, target)
	})

	t.Run("error with invalid target", func(t *testing.T) {
		err := repo.SetSymbolicReference("HEAD", "")
		assert.NotNil(t, err)
	})
}

func TestRepositoryRefSpec(t *testing.T) {
	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir, false)

	shortRefName := "master"
	qualifiedRefName := "refs/heads/master"
	qualifiedRemoteRefName := "refs/remotes/origin/master"

	treeBuilder := NewTreeBuilder(repo)
	emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
	if err != nil {
		t.Fatal(err)
	}

	commitID, err := repo.Commit(emptyTreeHash, qualifiedRefName, "Test Commit", false)
	if err != nil {
		t.Fatal(err)
	}
	refHash, err := repo.GetReference(qualifiedRefName)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, commitID, refHash, "unexpected value configuring test repo")

	tests := map[string]struct {
		repo            *Repository
		refName         string
		remoteName      string
		fastForwardOnly bool
		expectedRefSpec string
		expectedError   error
	}{
		"standard branch, not fast forward only, no remote": {
			refName:         "refs/heads/main",
			expectedRefSpec: "+refs/heads/main:refs/heads/main",
		},
		"standard branch, fast forward only, no remote": {
			refName:         "refs/heads/main",
			fastForwardOnly: true,
			expectedRefSpec: "refs/heads/main:refs/heads/main",
		},
		"standard branch, not fast forward only, remote": {
			refName:         "refs/heads/main",
			remoteName:      "origin",
			expectedRefSpec: "+refs/heads/main:refs/remotes/origin/main",
		},
		"standard branch, fast forward only, remote": {
			refName:         "refs/heads/main",
			remoteName:      "origin",
			fastForwardOnly: true,
			expectedRefSpec: "refs/heads/main:refs/remotes/origin/main",
		},
		"non-standard branch, not fast forward only, no remote": {
			refName:         "refs/heads/foo/bar",
			expectedRefSpec: "+refs/heads/foo/bar:refs/heads/foo/bar",
		},
		"non-standard branch, fast forward only, no remote": {
			refName:         "refs/heads/foo/bar",
			fastForwardOnly: true,
			expectedRefSpec: "refs/heads/foo/bar:refs/heads/foo/bar",
		},
		"non-standard branch, not fast forward only, remote": {
			refName:         "refs/heads/foo/bar",
			remoteName:      "origin",
			expectedRefSpec: "+refs/heads/foo/bar:refs/remotes/origin/foo/bar",
		},
		"non-standard branch, fast forward only, remote": {
			refName:         "refs/heads/foo/bar",
			remoteName:      "origin",
			fastForwardOnly: true,
			expectedRefSpec: "refs/heads/foo/bar:refs/remotes/origin/foo/bar",
		},
		"short branch, not fast forward only, no remote": {
			refName:         shortRefName,
			repo:            repo,
			expectedRefSpec: fmt.Sprintf("+%s:%s", qualifiedRefName, qualifiedRefName),
		},
		"short branch, fast forward only, no remote": {
			refName:         shortRefName,
			repo:            repo,
			fastForwardOnly: true,
			expectedRefSpec: fmt.Sprintf("%s:%s", qualifiedRefName, qualifiedRefName),
		},
		"short branch, not fast forward only, remote": {
			refName:         shortRefName,
			repo:            repo,
			remoteName:      "origin",
			expectedRefSpec: fmt.Sprintf("+%s:%s", qualifiedRefName, qualifiedRemoteRefName),
		},
		"short branch, fast forward only, remote": {
			refName:         shortRefName,
			repo:            repo,
			fastForwardOnly: true,
			remoteName:      "origin",
			expectedRefSpec: fmt.Sprintf("%s:%s", qualifiedRefName, qualifiedRemoteRefName),
		},
		"custom namespace, not fast forward only, no remote": {
			refName:         "refs/foo/bar",
			expectedRefSpec: "+refs/foo/bar:refs/foo/bar",
		},
		"custom namespace, fast forward only, no remote": {
			refName:         "refs/foo/bar",
			fastForwardOnly: true,
			expectedRefSpec: "refs/foo/bar:refs/foo/bar",
		},
		"custom namespace, not fast forward only, remote": {
			refName:         "refs/foo/bar",
			remoteName:      "origin",
			expectedRefSpec: "+refs/foo/bar:refs/remotes/origin/foo/bar",
		},
		"custom namespace, fast forward only, remote": {
			refName:         "refs/foo/bar",
			remoteName:      "origin",
			fastForwardOnly: true,
			expectedRefSpec: "refs/foo/bar:refs/remotes/origin/foo/bar",
		},
		"tag, not fast forward only, no remote": {
			refName:         "refs/tags/v1.0.0",
			fastForwardOnly: false,
			expectedRefSpec: "refs/tags/v1.0.0:refs/tags/v1.0.0",
		},
		"tag, fast forward only, no remote": {
			refName:         "refs/tags/v1.0.0",
			fastForwardOnly: true,
			expectedRefSpec: "refs/tags/v1.0.0:refs/tags/v1.0.0",
		},
		"tag, not fast forward only, remote": {
			refName:         "refs/tags/v1.0.0",
			remoteName:      "origin",
			fastForwardOnly: false,
			expectedRefSpec: "refs/tags/v1.0.0:refs/tags/v1.0.0",
		},
		"tag, fast forward only, remote": {
			refName:         "refs/tags/v1.0.0",
			remoteName:      "origin",
			fastForwardOnly: true,
			expectedRefSpec: "refs/tags/v1.0.0:refs/tags/v1.0.0",
		},
	}

	for name, test := range tests {
		refSpec, err := test.repo.RefSpec(test.refName, test.remoteName, test.fastForwardOnly)
		assert.ErrorIs(t, err, test.expectedError, fmt.Sprintf("unexpected error in test '%s'", name))
		assert.Equal(t, test.expectedRefSpec, refSpec, fmt.Sprintf("unexpected refspec returned in test '%s'", name))
	}
}

func TestBranchReferenceName(t *testing.T) {
	tests := map[string]struct {
		branchName            string
		expectedReferenceName string
	}{
		"short name": {
			branchName:            "main",
			expectedReferenceName: "refs/heads/main",
		},
		"reference name": {
			branchName:            "refs/heads/main",
			expectedReferenceName: "refs/heads/main",
		},
	}

	for name, test := range tests {
		referenceName := BranchReferenceName(test.branchName)
		assert.Equal(t, test.expectedReferenceName, referenceName, fmt.Sprintf("unexpected branch reference received in test '%s'", name))
	}
}

func TestTagReferenceName(t *testing.T) {
	tests := map[string]struct {
		tagName               string
		expectedReferenceName string
	}{
		"short name": {
			tagName:               "v1",
			expectedReferenceName: "refs/tags/v1",
		},
		"reference name": {
			tagName:               "refs/tags/v1",
			expectedReferenceName: "refs/tags/v1",
		},
	}

	for name, test := range tests {
		referenceName := TagReferenceName(test.tagName)
		assert.Equal(t, test.expectedReferenceName, referenceName, fmt.Sprintf("unexpected tag reference received in test '%s'", name))
	}
}

func TestDeleteReference(t *testing.T) {
	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir, false)

	refName := "refs/heads/main"
	treeBuilder := NewTreeBuilder(repo)

	emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
	if err != nil {
		t.Fatal(err)
	}

	commitID, err := repo.Commit(emptyTreeID, refName, "Initial commit\n", false)
	require.Nil(t, err)

	refTip, err := repo.GetReference(refName)
	require.Nil(t, err)
	require.Equal(t, commitID, refTip)

	err = repo.DeleteReference(refName)
	assert.Nil(t, err)

	_, err = repo.GetReference(refName)
	assert.ErrorIs(t, err, ErrReferenceNotFound)

	t.Run("delete non-existent reference", func(t *testing.T) {
		err := repo.DeleteReference("refs/heads/nonexistent")
		assert.Nil(t, err)
	})
}

func TestRemoteReferenceName(t *testing.T) {
	tests := map[string]struct {
		input    string
		expected string
	}{
		"adds prefix if missing": {
			input:    "origin/main",
			expected: "refs/remotes/origin/main",
		},
		"keeps prefix if already present": {
			input:    "refs/remotes/origin/main",
			expected: "refs/remotes/origin/main",
		},
		"empty input returns prefix only": {
			input:    "",
			expected: "refs/remotes/",
		},
		"exact prefix is preserved": {
			input:    "refs/remotes/",
			expected: "refs/remotes/",
		},
	}

	for name, test := range tests {
		referenceName := RemoteReferenceName(test.input)
		assert.Equal(t, test.expected, referenceName, fmt.Sprintf("unexpected remote reference for input %s", name))
	}
}

func TestCustomReferenceName(t *testing.T) {
	tests := map[string]struct {
		input    string
		expected string
	}{
		"adds prefix if missing": {
			input:    "custom/ref",
			expected: "refs/custom/ref",
		},
		"keeps prefix if already present": {
			input:    "refs/custom/ref",
			expected: "refs/custom/ref",
		},
		"simple name": {
			input:    "myref",
			expected: "refs/myref",
		},
	}

	for name, test := range tests {
		referenceName := CustomReferenceName(test.input)
		assert.Equal(t, test.expected, referenceName, fmt.Sprintf("unexpected custom reference for input %s", name))
	}
}

func TestAbsoluteReference(t *testing.T) {
	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir, false)

	refName := "refs/heads/main"
	treeBuilder := NewTreeBuilder(repo)

	emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
	require.Nil(t, err)

	_, err = repo.Commit(emptyTreeID, refName, "Initial commit\n", false)
	require.Nil(t, err)

	tests := map[string]struct {
		input    string
		expected string
		hasError bool
	}{
		"short branch name": {
			input:    "main",
			expected: "refs/heads/main",
		},
		"full branch reference": {
			input:    "refs/heads/main",
			expected: "refs/heads/main",
		},
		"HEAD symbolic ref": {
			input:    "HEAD",
			expected: "refs/heads/main",
		},
		"non-existent ref": {
			input:    "nonexistent",
			hasError: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := repo.AbsoluteReference(test.input)
			if test.hasError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, test.expected, result)
			}
		})
	}
}
