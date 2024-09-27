// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"fmt"
	"sort"
	"strings"
)

// GetCommitsBetweenRange returns the IDs of the commits that exist between the
// specified new and old commit identifiers.
func (r *Repository) GetCommitsBetweenRange(commitNewID, commitOldID Hash) ([]Hash, error) {
	var args []string
	if commitOldID.IsZero() {
		args = []string{"rev-list", commitNewID.String()}
	} else {
		args = []string{"rev-list", fmt.Sprintf("%s..%s", commitOldID.String(), commitNewID.String())}
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

	// FIXME: we should ideally be sorting this in the order of occurrence
	// rather than by commit ID. The only reason this is happening is because
	// the ordering of commitRange by default is not deterministic. Rather than
	// walking through them and identifying the right order, we're sorting by
	// commit ID. The intended use case of this function is to get a list of
	// commits that are then checked for the changes they introduce. At that
	// point, they must be diffed with their parent directly.
	sort.Slice(commitRange, func(i, j int) bool {
		return commitRange[i].String() < commitRange[j].String()
	})

	return commitRange, nil
}
