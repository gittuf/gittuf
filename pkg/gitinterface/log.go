// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"fmt"
	"strings"
)

// GetCommitsBetweenRange returns the IDs of the commits that exist between the
// specified new and old commit identifiers. Commits are returned in
// topological order, where each commit appears before its parents.
func (r *Repository) GetCommitsBetweenRange(commitNewID, commitOldID Hash) ([]Hash, error) {
	var args []string
	if commitOldID.IsZero() {
		args = []string{"rev-list", "--topo-order", commitNewID.String()}
	} else {
		args = []string{"rev-list", "--topo-order", fmt.Sprintf("%s..%s", commitOldID.String(), commitNewID.String())}
	}

	commitRangeString, err := r.executor(args...).executeString()
	if err != nil {
		return nil, fmt.Errorf("unable to enumerate commits in range: %w", err)
	}

	commitRangeSplit := strings.Split(commitRangeString, "\n")

	commitRange := make([]Hash, 0, len(commitRangeSplit))
	for _, cID := range commitRangeSplit {
		if cID == "" {
			continue
		}
		hash, err := NewHash(cID)
		if err != nil {
			return nil, err
		}
		commitRange = append(commitRange, hash)
	}

	return commitRange, nil
}
