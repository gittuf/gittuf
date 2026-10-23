// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"bytes"
	"fmt"
	"sort"
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
		stdOut, err := r.executor("ls-tree", "--name-only", "-r", "-z", commitID.String()).executeBytes()
		if err != nil {
			return nil, fmt.Errorf("unable to identify all commit file paths: %w", err)
		}

		return parseNULDelimitedPaths(stdOut), nil
	}

	if len(parentCommitIDs) > 1 {
		// Check if tree matches last commit
		stdOut, err := r.executor("diff-tree", "--no-commit-id", "--name-only", "-r", "-z", parentCommitIDs[len(parentCommitIDs)-1].String(), commitID.String()).executeBytes()
		if err != nil {
			return nil, fmt.Errorf("unable to diff commit against last parent commit: %w", err)
		}
		if len(stdOut) == 0 {
			return nil, nil
		}

		pathSet := map[string]bool{}
		for _, parentCommitID := range parentCommitIDs {
			stdOut, err := r.executor("diff-tree", "--no-commit-id", "--name-only", "-r", "-z", parentCommitID.String(), commitID.String()).executeBytes()
			if err != nil {
				return nil, fmt.Errorf("unable to diff commit against parent: %w", err)
			}

			for _, path := range parseNULDelimitedPaths(stdOut) {
				pathSet[path] = true
			}
		}

		paths := make([]string, 0, len(pathSet))
		for path := range pathSet {
			paths = append(paths, path)
		}

		sort.Strings(paths)

		return paths, nil
	}

	stdOut, err := r.executor("diff-tree", "--no-commit-id", "--name-only", "-r", "-z", fmt.Sprintf("%s~1", commitID.String()), commitID.String()).executeBytes()
	if err != nil {
		return nil, fmt.Errorf("unable to diff commit against parent: %w", err)
	}

	return parseNULDelimitedPaths(stdOut), nil
}

func parseNULDelimitedPaths(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}

	raw = bytes.TrimSuffix(raw, []byte{0})
	if len(raw) == 0 {
		return nil
	}

	parts := bytes.Split(raw, []byte{0})
	paths := make([]string, len(parts))
	for i, part := range parts {
		paths[i] = string(part)
	}

	return paths
}
