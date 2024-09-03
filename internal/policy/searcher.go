// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
)

// Searcher defines the interface for identifying the applicable policy entry in
// the RSL for some entry.
type Searcher interface {
	FindPolicyEntryFor(rsl.Entry) (*rsl.ReferenceEntry, error)
}

// RegularSearcher implements the Searcher interface. It walks back the RSL from
// the specified entry to find the latest policy entry.
type RegularSearcher struct {
	repo *gitinterface.Repository
}

func (r *RegularSearcher) FindPolicyEntryFor(entry rsl.Entry) (*rsl.ReferenceEntry, error) {
	// If the requested entry itself is for the policy ref, return as is
	if entry, isReferenceEntry := entry.(*rsl.ReferenceEntry); isReferenceEntry && entry.RefName == PolicyRef {
		slog.Debug(fmt.Sprintf("Initial entry '%s' is for gittuf policy, setting that as current policy...", entry.GetID().String()))
		return entry, nil
	}

	policyEntry, _, err := rsl.GetLatestReferenceEntry(r.repo, rsl.ForReference(PolicyRef), rsl.BeforeEntryID(entry.GetID()))
	if err != nil {
		if errors.Is(err, rsl.ErrRSLEntryNotFound) {
			slog.Debug(fmt.Sprintf("No policy found before initial entry '%s'", entry.GetID().String()))
			return nil, ErrPolicyNotFound
		}

		// Any other err must be returned
		return nil, err
	}

	return policyEntry, nil
}

func NewRegularSearcher(repo *gitinterface.Repository) *RegularSearcher {
	return &RegularSearcher{repo: repo}
}
