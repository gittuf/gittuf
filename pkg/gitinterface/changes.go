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
	commitInfo, err := r.GetCommitInfo(commitID)
	if err != nil {
		return nil, err
	}
	parentCommitIDs := commitInfo.ParentIDs

	if len(parentCommitIDs) == 0 {
		filePaths, err := r.executor("ls-tree", "--name-only", "-r", "-z", commitID.String()).withPreserveSpaces().executeString()
		if err != nil {
			return nil, fmt.Errorf("unable to identify all commit file paths: %w", err)
		}

		if filePaths == "" {
			return nil, nil
		}

		paths := splitPathsOnNUL(filePaths)
		return paths, nil
	}

	if len(parentCommitIDs) > 1 {
		// Check if tree matches last commit
		stdOut, err := r.executor("diff-tree", "--no-commit-id", "--name-only", "-r", "-z", parentCommitIDs[len(parentCommitIDs)-1].String(), commitID.String()).withPreserveSpaces().executeString()
		if err != nil {
			return nil, fmt.Errorf("unable to diff commit against last parent commit: %w", err)
		}
		if stdOut == "" {
			return nil, nil
		}

		pathSet := map[string]bool{}
		for _, parentCommitID := range parentCommitIDs {
			stdOut, err := r.executor("diff-tree", "--no-commit-id", "--name-only", "-r", "-z", parentCommitID.String(), commitID.String()).withPreserveSpaces().executeString()
			if err != nil {
				return nil, fmt.Errorf("unable to diff commit against parent: %w", err)
			}
			if stdOut == "" {
				continue
			}

			paths := splitPathsOnNUL(stdOut)
			for _, path := range paths {
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

	stdOut, err := r.executor("diff-tree", "--no-commit-id", "--name-only", "-r", "-z", fmt.Sprintf("%s~1", commitID.String()), commitID.String()).withPreserveSpaces().executeString()
	if err != nil {
		return nil, fmt.Errorf("unable to diff commit against parent: %w", err)
	}
	if stdOut == "" {
		return nil, nil
	}

	paths := splitPathsOnNUL(stdOut)
	return paths, nil
}

// splitPathsOnNUL splits -z NUL-terminated output into literal path strings.
func splitPathsOnNUL(output string) []string {
	rawPaths := strings.Split(output, "\x00")
	paths := make([]string, 0, len(rawPaths))
	for _, path := range rawPaths {
		if path == "" {
			continue
		}
		paths = append(paths, path)
	}
	return paths
}
