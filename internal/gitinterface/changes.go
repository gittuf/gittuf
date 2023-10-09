// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"fmt"
	"sort"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// GetCommitFilePaths returns all the file paths of the provided commit object.
// This strictly enumerates all the files recursively in the commit object's
// tree.
func GetCommitFilePaths(repo *git.Repository, commit *object.Commit) ([]string, error) {
	filesIter, err := commit.Files()
	if err != nil {
		return nil, err
	}

	paths := []string{}
	if err := filesIter.ForEach(func(f *object.File) error {
		paths = append(paths, f.Name)
		return nil
	}); err != nil {
		return nil, err
	}

	sort.Slice(paths, func(i, j int) bool {
		return paths[i] < paths[j]
	})

	return paths, nil
}

// GetFilePathsChangedByCommit returns the paths changed by the commit relative
// to its parent commit. If the commit is a merge commit, i.e., it has more than
// one parent, no changes are returned.
//
// Currently, this function does not verify that the tree for a merge commit
// matches one of its parents. In a future version, this behavior may change and
// return an error if a multi-parent commit seems invalid.
func GetFilePathsChangedByCommit(repo *git.Repository, commit *object.Commit) ([]string, error) {
	if len(commit.ParentHashes) > 1 {
		// merge commits are expected not to introduce changes themselves
		// TODO: should we check that the merge commit's tree matches one of its
		// parents (usually the last)?
		return nil, nil
	}

	if len(commit.ParentHashes) == 0 {
		// No parent, return all file paths for commit
		return GetCommitFilePaths(repo, commit)
	}

	parentCommit, err := repo.CommitObject(commit.ParentHashes[0])
	if err != nil {
		return nil, err
	}

	return GetDiffFilePaths(repo, commit, parentCommit)
}

// GetDiffFilePaths enumerates all the changed file paths between the two
// commits. If one of the commits is nil, the other commit's tree is enumerated.
func GetDiffFilePaths(repo *git.Repository, commitA, commitB *object.Commit) ([]string, error) {
	if commitA == nil && commitB == nil {
		return nil, fmt.Errorf("both commits cannot be empty")
	}

	if commitA == nil {
		return GetCommitFilePaths(repo, commitB)
	}
	if commitB == nil {
		return GetCommitFilePaths(repo, commitA)
	}

	treeA, err := commitA.Tree()
	if err != nil {
		return nil, err
	}

	treeB, err := commitB.Tree()
	if err != nil {
		return nil, err
	}

	return diff(treeA, treeB)
}

// diff is a helper that enumerates and sorts the paths of all files that differ
// between the two trees. If a file is renamed, both its source name and
// destination name are recorded.
func diff(treeA, treeB *object.Tree) ([]string, error) {
	changesSet := map[string]bool{}
	changes, err := treeA.Diff(treeB)
	if err != nil {
		return nil, err
	}

	for _, c := range changes {
		if len(c.From.Name) > 0 {
			changesSet[c.From.Name] = true
		}
		if len(c.To.Name) > 0 {
			changesSet[c.To.Name] = true
		}
	}

	paths := []string{}
	for p := range changesSet {
		paths = append(paths, p)
	}

	sort.Slice(paths, func(i, j int) bool {
		return paths[i] < paths[j]
	})

	return paths, nil
}
