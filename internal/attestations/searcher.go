// SPDX-License-Identifier: Apache-2.0

package attestations

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
)

var ErrAttestationsNotFound = errors.New("attestations not found")

// Searcher defines the interface for identifying the applicable attestations
// entry in the RSL for some entry.
type Searcher interface {
	FindAttestationsEntryFor(rsl.Entry) (*rsl.ReferenceEntry, error)
}

// RegularSearcher implements the Searcher interface. It walks back the RSL from
// the specified entry to find the latest attestations entry.
type RegularSearcher struct {
	repo *gitinterface.Repository
}

func (r *RegularSearcher) FindAttestationsEntryFor(entry rsl.Entry) (*rsl.ReferenceEntry, error) {
	// If the requested entry itself is for the attestations ref, return as is
	if entry, isReferenceEntry := entry.(*rsl.ReferenceEntry); isReferenceEntry && entry.RefName == Ref {
		slog.Debug(fmt.Sprintf("Initial entry '%s' is for attestations, setting that as current set of attestations...", entry.GetID().String()))
		return entry, nil
	}

	attestationsEntry, _, err := rsl.GetLatestReferenceEntry(r.repo, rsl.ForReference(Ref), rsl.BeforeEntryID(entry.GetID()))
	if err != nil {
		if errors.Is(err, rsl.ErrRSLEntryNotFound) {
			// Attestations may not be used yet, they're not
			// compulsory
			slog.Debug(fmt.Sprintf("No attestations found before initial entry '%s'", entry.GetID().String()))
			return nil, ErrAttestationsNotFound
		}

		return nil, err
	}

	return attestationsEntry, nil
}

func NewRegularSearcher(repo *gitinterface.Repository) *RegularSearcher {
	return &RegularSearcher{repo: repo}
}
