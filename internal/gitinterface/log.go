// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/revlist"
)

// GetCommitsBetweenRange returns the commits (including the new commit,
// excluding the old) between the specified ranges. If the old commit ID is set
// to zero, all commits reachable from the new commit are returned.
//
// The returned commits are sorted by commit IDs. Ideally, they should be
// ordered by occurrence but go-git introduces some randomness here. It might be
// an effect of walking the graph anyway, so the sort by ID ensures the returned
// commit slice is deterministic.
func GetCommitsBetweenRange(repo *git.Repository, commitNewID, commitOldID plumbing.Hash) ([]*object.Commit, error) {
	all := false

	if commitOldID.IsZero() {
		all = true
	}

	var (
		commitRange []plumbing.Hash
		err         error
	)

	if all {
		commitRange, err = revlist.Objects(repo.Storer, []plumbing.Hash{commitNewID}, nil)
		if err != nil {
			return nil, err
		}
	} else {
		reachableFromCommitOld, err := revlist.Objects(repo.Storer, []plumbing.Hash{commitOldID}, nil)
		if err != nil {
			return nil, err
		}

		commitRange, err = revlist.Objects(repo.Storer, []plumbing.Hash{commitNewID}, reachableFromCommitOld)
		if err != nil {
			return nil, err
		}
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

	commits := make([]*object.Commit, 0, len(commitRange))
	for _, commitID := range commitRange {
		commit, err := GetCommit(repo, commitID)
		if err != nil {
			if errors.Is(err, plumbing.ErrObjectNotFound) {
				// Returned for non-commit objects
				continue
			}
			return nil, err
		}

		commits = append(commits, commit)
	}

	return commits, nil
}

// GetCommitsBetweenRange returns the IDs of the commits that exist between the
// specified new and old commit identifiers.
func (r *Repository) GetCommitsBetweenRange(commitNewID, commitOldID Hash) ([]Hash, error) {
	var args []string
	if commitOldID.IsZero() {
		args = []string{"rev-list", commitNewID.String()}
	} else {
		args = []string{"rev-list", fmt.Sprintf("%s..%s", commitOldID.String(), commitNewID.String())}
	}

	commitRangeString, err := r.executor(args...).execute()
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
