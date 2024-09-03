// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"errors"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
)

type Searcher interface {
	FindPolicyEntryFor(*rsl.ReferenceEntry) (*rsl.ReferenceEntry, error)
}

type RegularSearcher struct {
	repo *gitinterface.Repository
}

func (r *RegularSearcher) FindPolicyEntryFor(entry *rsl.ReferenceEntry) (*rsl.ReferenceEntry, error) {
	// If the requested entry itself is for the policy ref, return as is
	if entry.RefName == PolicyRef {
		return entry, nil
	}

	policyEntry, _, err := rsl.GetLatestReferenceEntryForRefBefore(r.repo, PolicyRef, entry.ID)
	if err != nil {
		if errors.Is(err, rsl.ErrRSLEntryNotFound) {
			// No policy found is only okay if entry is the very
			// first entry in the RSL
			entryParentIDs, err := r.repo.GetCommitParentIDs(entry.ID)
			if err != nil {
				return nil, err
			}
			if len(entryParentIDs) != 0 {
				return nil, ErrPolicyNotFound
			}
			// The entry is the very first entry
			// However, we still return nil: we don't want a
			// policy-staging ref to be loaded and trusted
			return nil, nil
		}

		// Any other err must also be returned
		return nil, err
	}

	return policyEntry, nil
}

func NewRegularSearcher(repo *gitinterface.Repository) *RegularSearcher {
	return &RegularSearcher{repo: repo}
}
