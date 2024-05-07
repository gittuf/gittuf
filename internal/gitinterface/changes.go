// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"container/heap"
	"fmt"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// GetCommitFilePaths returns all the file paths of the provided commit object.
// This strictly enumerates all the files recursively in the commit object's
// tree.
func GetCommitFilePaths(commit *object.Commit) ([]string, error) {
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
// one parent, check if the commit is the same as at least one of its parents.
// If there is a matching parent, we return no changes. If there is no matching
// parent commit, we return the changes between the commit and each of its parents.
func GetFilePathsChangedByCommit(repo *git.Repository, commit *object.Commit) ([]string, error) {
	if len(commit.ParentHashes) > 1 {
		// Merge commit: compare with each parent and aggregate changes

		// Create a map to store all changes

		contains := make(map[string]bool)

		// keeping a heap of diffs so that we can pop them in sorted order

		diffs := &diffHeap{}
		heap.Init(diffs)

		// We are iterating backwards since if there is a matching parent commit,
		// it is likely to be the last one.
		for parentHashIndex := len(commit.ParentHashes) - 1; parentHashIndex >= 0; parentHashIndex-- {
			parentCommit, err := GetCommit(repo, commit.ParentHashes[parentHashIndex])
			if err != nil {
				return nil, err
			}
			// If the commit tree hash is the same as the parent tree hash, there are no changes
			if commit.TreeHash == parentCommit.TreeHash {
				return nil, nil
			}

			// Get the diff file paths between the commit and its parent
			diff, err := GetDiffFilePaths(commit, parentCommit)
			if err != nil {
				return nil, err
			}

			for _, change := range diff {
				// if we have not already added this change
				if !contains[change] {
					heap.Push(diffs, change)
				}
				// Add changes to the map
				contains[change] = true
			}
		}

		// Convert the heap to a slice
		changes := make([]string, len(contains))

		for pos := 0; diffs.Len() > 0; pos++ {
			changes[pos] = heap.Pop(diffs).(string)
		}

		return changes, nil
	}

	if len(commit.ParentHashes) == 0 {
		// No parent, return all file paths for commit
		return GetCommitFilePaths(commit)
	}

	parentCommit, err := GetCommit(repo, commit.ParentHashes[0])
	if err != nil {
		return nil, err
	}

	return GetDiffFilePaths(commit, parentCommit)
}

func (r *Repository) GetFilePathsChangedByCommit(commitID Hash) ([]string, error) {
	if err := r.ensureIsCommit(commitID); err != nil {
		return nil, err
	}

	parentCommitIDs, err := r.GetCommitParentIDs(commitID)
	if err != nil {
		return nil, err
	}

	if len(parentCommitIDs) == 0 {
		stdOut, stdErr, err := r.executeGitCommand("ls-tree", "--name-only", "-r", commitID.String())
		if err != nil {
			return nil, fmt.Errorf("unable to identify all commit file paths: %s", stdErr)
		}

		paths := strings.Split(strings.TrimSpace(stdOut), "\n")
		return paths, nil
	}

	if len(parentCommitIDs) > 1 {
		// Check if tree matches last commit
		stdOut, stdErr, err := r.executeGitCommand("diff-tree", "--no-commit-id", "--name-only", "-r", parentCommitIDs[len(parentCommitIDs)-1].String(), commitID.String())
		if err != nil {
			return nil, fmt.Errorf("unable to diff commit against last parent commit: %s", stdErr)
		}

		stdOut = strings.TrimSpace(stdOut)
		if stdOut == "" {
			return nil, nil
		}

		pathSet := map[string]bool{}
		for _, parentCommitID := range parentCommitIDs {
			stdOut, stdErr, err := r.executeGitCommand("diff-tree", "--no-commit-id", "--name-only", "-r", parentCommitID.String(), commitID.String())
			if err != nil {
				return nil, fmt.Errorf("unable to diff commit against parent: %s", stdErr)
			}

			stdOut = strings.TrimSpace(stdOut)
			if stdOut == "" {
				continue
			}

			paths := strings.Split(stdOut, "\n")
			for _, path := range paths {
				if path == "" {
					continue
				}
				pathSet[path] = true
			}
		}

		paths := make([]string, 0, len(pathSet))
		for path := range pathSet {
			paths = append(paths, path)
		}

		sort.Slice(paths, func(i, j int) bool {
			return paths[i] < paths[j]
		})

		return paths, nil
	}

	stdOut, stdErr, err := r.executeGitCommand("diff-tree", "--no-commit-id", "--name-only", "-r", fmt.Sprintf("%s~1", commitID.String()), commitID.String())
	if err != nil {
		return nil, fmt.Errorf("unable to diff commit against parent: %s", stdErr)
	}

	stdOut = strings.TrimSpace(stdOut)
	if stdOut == "" {
		return nil, nil
	}

	paths := strings.Split(stdOut, "\n")
	return paths, nil
}

// GetDiffFilePaths enumerates all the changed file paths between the two
// commits. If one of the commits is nil, the other commit's tree is enumerated.
func GetDiffFilePaths(commitA, commitB *object.Commit) ([]string, error) {
	if commitA == nil && commitB == nil {
		return nil, fmt.Errorf("both commits cannot be empty")
	}

	if commitA == nil {
		return GetCommitFilePaths(commitB)
	}
	if commitB == nil {
		return GetCommitFilePaths(commitA)
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

type diffHeap []string

func (h diffHeap) Len() int           { return len(h) }
func (h diffHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h diffHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *diffHeap) Push(x any) {
	// Push and Pop use pointer receivers because they modify the slice's length,
	// not just its contents.
	*h = append(*h, x.(string))
}

func (h *diffHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}
