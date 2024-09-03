// SPDX-License-Identifier: Apache-2.0

package attestations

import (
	"errors"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
)

type Searcher interface {
	FindAttestationsEntryFor(*rsl.ReferenceEntry) (*rsl.ReferenceEntry, error)
}

type RegularSearcher struct {
	repo *gitinterface.Repository
}

func (r *RegularSearcher) FindAttestationsEntryFor(entry *rsl.ReferenceEntry) (*rsl.ReferenceEntry, error) {
	// If the requested entry itself is for the attestations ref, return as is
	if entry.RefName == Ref {
		return entry, nil
	}

	attestationsEntry, _, err := rsl.GetLatestReferenceEntryForRefBefore(r.repo, Ref, entry.ID)
	if err != nil {
		if !errors.Is(err, rsl.ErrRSLEntryNotFound) {
			// Attestations may not be used yet, they're not
			// compulsory
			return nil, err
		}
	}

	return attestationsEntry, nil
}

func NewRegularSearcher(repo *gitinterface.Repository) *RegularSearcher {
	return &RegularSearcher{repo: repo}
}
