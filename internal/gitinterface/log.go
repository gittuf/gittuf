// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/featureflags"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
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
func GetCommitsBetweenRange(repo *git.Repository, commitNewID, commitOldID plumbing.Hash) ([]plumbing.Hash, error) {
	if featureflags.UseGitBinary {
		return GetCommitsBetweenRangeUsingBinary(repo, commitNewID, commitOldID)
	}

	all := false

	if commitOldID.IsZero() {
		all = true
	}

	var (
		objectsRange []plumbing.Hash
		err          error
	)

	if all {
		objectsRange, err = revlist.Objects(repo.Storer, []plumbing.Hash{commitNewID}, nil)
		if err != nil {
			return nil, err
		}
	} else {
		reachableFromCommitOld, err := revlist.Objects(repo.Storer, []plumbing.Hash{commitOldID}, nil)
		if err != nil {
			return nil, err
		}

		objectsRange, err = revlist.Objects(repo.Storer, []plumbing.Hash{commitNewID}, reachableFromCommitOld)
		if err != nil {
			return nil, err
		}
	}

	commitRange := make([]plumbing.Hash, 0, len(objectsRange))
	for _, objectID := range objectsRange {
		_, err := GetCommit(repo, objectID)
		if err != nil {
			if errors.Is(err, plumbing.ErrObjectNotFound) {
				// Returned for non-commit objects
				continue
			}
			return nil, err
		}
		commitRange = append(commitRange, objectID)
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

// GetCommitsBetweenRangeUsingBinary is an implementation of
// GetCommitsBetweenRange that uses the Git binary instead of go-git.
func GetCommitsBetweenRangeUsingBinary(_ *git.Repository, commitNewID, commitOldID plumbing.Hash) ([]plumbing.Hash, error) {
	if !dev.InDevMode() {
		return nil, dev.ErrNotInDevMode
	}

	var command *exec.Cmd

	if commitOldID.IsZero() {
		command = exec.Command("git", "rev-list", commitNewID.String()) //nolint:gosec
	} else {
		command = exec.Command("git", "rev-list", fmt.Sprintf("%s..%s", commitOldID.String(), commitNewID.String())) //nolint:gosec
	}

	stdOut, err := command.Output()
	if err != nil {
		return nil, err
	}
	commitRangeString := strings.Split(strings.TrimSpace(string(stdOut)), "\n")

	commitRange := make([]plumbing.Hash, 0, len(commitRangeString))
	for _, cID := range commitRangeString {
		if cID == "" {
			continue
		}
		commitRange = append(commitRange, plumbing.NewHash(cID))
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
