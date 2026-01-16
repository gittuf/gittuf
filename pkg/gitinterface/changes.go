// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"fmt"
	"sort"
	"strings"
)

// GetFilePathsChangedByCommit returns the paths changed by the commit relative
// to its parent commit. If the commit is a merge commit, i.e., it has more than
// one parent, check if the commit is the same as at least one of its parents.
// If there is a matching parent, we return no changes. If there is no matching
// parent commit, we return the changes between the commit and each of its parents.
func (r *Repository) GetFilePathsChangedByCommit(commitID Hash) ([]string, error) {
	if err := r.ensureIsCommit(commitID); err != nil {
		return nil, err
	}

	parentCommitIDs, err := r.GetCommitParentIDs(commitID)
	if err != nil {
		return nil, err
	}

	if len(parentCommitIDs) == 0 {
		filePaths, err := r.executor("ls-tree", "--name-only", "-r", commitID.String()).executeString()
		if err != nil {
			return nil, fmt.Errorf("unable to identify all commit file paths: %w", err)
		}

		paths := strings.Split(filePaths, "\n")
		return paths, nil
	}

	if len(parentCommitIDs) > 1 {
		// Check if tree matches last commit
		stdOut, err := r.executor("diff-tree", "--no-commit-id", "--name-only", "-r", parentCommitIDs[len(parentCommitIDs)-1].String(), commitID.String()).executeString()
		if err != nil {
			return nil, fmt.Errorf("unable to diff commit against last parent commit: %w", err)
		}
		if stdOut == "" {
			return nil, nil
		}

		pathSet := map[string]bool{}
		for _, parentCommitID := range parentCommitIDs {
			stdOut, err := r.executor("diff-tree", "--no-commit-id", "--name-only", "-r", parentCommitID.String(), commitID.String()).executeString()
			if err != nil {
				return nil, fmt.Errorf("unable to diff commit against parent: %w", err)
			}
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

	stdOut, err := r.executor("diff-tree", "--no-commit-id", "--name-only", "-r", fmt.Sprintf("%s~1", commitID.String()), commitID.String()).executeString()
	if err != nil {
		return nil, fmt.Errorf("unable to diff commit against parent: %w", err)
	}
	if stdOut == "" {
		return nil, nil
	}

	paths := strings.Split(stdOut, "\n")
	return paths, nil
}
