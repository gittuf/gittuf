// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/gittuf/gittuf/internal/attestations"
	"github.com/gittuf/gittuf/pkg/gitstore"
	"github.com/gittuf/gittuf/pkg/rsl"
)

// searcher defines the interface for finding policy and attestation entries in
// the RSL.
type searcher interface {
	FindFirstPolicyEntry() (rsl.ReferenceUpdaterEntry, error)
	FindLatestPolicyEntry() (rsl.ReferenceUpdaterEntry, error)
	FindPolicyEntryFor(rsl.Entry) (rsl.ReferenceUpdaterEntry, error)
	FindPolicyEntriesInRange(rsl.Entry, rsl.Entry) ([]rsl.ReferenceUpdaterEntry, error)
	FindAttestationsEntryFor(rsl.Entry) (rsl.ReferenceUpdaterEntry, error)
	FindLatestAttestationsEntry() (rsl.ReferenceUpdaterEntry, error)
}

func newSearcher(repo gitstore.Storer) searcher {
	return newRegularSearcher(repo)
}

// regularSearcher implements the searcher interface. It walks back the RSL from
// to identify the requested policy or attestation entries.
type regularSearcher struct {
	storer gitstore.Storer
}

// FindFirstPolicyEntry identifies the very first policy entry in the RSL.
func (r *regularSearcher) FindFirstPolicyEntry() (rsl.ReferenceUpdaterEntry, error) {
	entry, _, err := rsl.GetFirstReferenceUpdaterEntryForRef(r.storer, PolicyRef)
	if err != nil {
		if errors.Is(err, rsl.ErrRSLEntryNotFound) {
			// we don't have a policy entry yet
			return nil, ErrPolicyNotFound
		}
		return nil, err
	}

	return entry, nil
}

// FindLatestPolicyEntry returns the latest policy entry in the RSL.
func (r *regularSearcher) FindLatestPolicyEntry() (rsl.ReferenceUpdaterEntry, error) {
	entry, _, err := rsl.GetLatestReferenceUpdaterEntry(r.storer, rsl.ForReference(PolicyRef))
	if err != nil {
		if errors.Is(err, rsl.ErrRSLEntryNotFound) {
			// we don't have a policy entry
			return nil, ErrPolicyNotFound
		}
		return nil, err
	}
	return entry, nil
}

// FindPolicyEntryFor identifies the latest policy entry for the specified
// entry.
func (r *regularSearcher) FindPolicyEntryFor(entry rsl.Entry) (rsl.ReferenceUpdaterEntry, error) {
	// If the requested entry itself is for the policy ref, return as is
	if entry, isReferenceUpdaterEntry := entry.(rsl.ReferenceUpdaterEntry); isReferenceUpdaterEntry && entry.GetRefName() == PolicyRef {
		slog.Debug(fmt.Sprintf("Initial entry '%s' is for gittuf policy, setting that as current policy...", entry.GetID().String()))
		return entry, nil
	}

	policyEntry, _, err := rsl.GetLatestReferenceUpdaterEntry(r.storer, rsl.ForReference(PolicyRef), rsl.BeforeEntryID(entry.GetID()))
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

// FindPolicyEntriesInRange returns all policy RSL entries in the specified
// range. firstEntry and lastEntry are included if they are for the policy ref.
func (r *regularSearcher) FindPolicyEntriesInRange(firstEntry, lastEntry rsl.Entry) ([]rsl.ReferenceUpdaterEntry, error) {
	allPolicyEntries, _, err := rsl.GetReferenceUpdaterEntriesInRangeForRef(r.storer, firstEntry.GetID(), lastEntry.GetID(), PolicyRef)
	if err != nil {
		return nil, err
	}

	return allPolicyEntries, nil
}

// FindAttestationsEntryFor identifies the latest attestations entry for the
// specified entry.
func (r *regularSearcher) FindAttestationsEntryFor(entry rsl.Entry) (rsl.ReferenceUpdaterEntry, error) {
	// If the requested entry itself is for the attestations ref, return as is
	if entry, isReferenceUpdaterEntry := entry.(rsl.ReferenceUpdaterEntry); isReferenceUpdaterEntry && entry.GetRefName() == attestations.Ref {
		slog.Debug(fmt.Sprintf("Initial entry '%s' is for attestations, setting that as current set of attestations...", entry.GetID().String()))
		return entry, nil
	}

	attestationsEntry, _, err := rsl.GetLatestReferenceUpdaterEntry(r.storer, rsl.ForReference(attestations.Ref), rsl.BeforeEntryID(entry.GetID()))
	if err != nil {
		if errors.Is(err, rsl.ErrRSLEntryNotFound) {
			// Attestations may not be used yet, they're not
			// compulsory
			slog.Debug(fmt.Sprintf("No attestations found before initial entry '%s'", entry.GetID().String()))
			return nil, attestations.ErrAttestationsNotFound
		}

		return nil, err
	}

	return attestationsEntry, nil
}

// FindLatestAttestationsEntry returns the latest RSL entry for the attestations
// reference.
func (r *regularSearcher) FindLatestAttestationsEntry() (rsl.ReferenceUpdaterEntry, error) {
	entry, _, err := rsl.GetLatestReferenceUpdaterEntry(r.storer, rsl.ForReference(attestations.Ref))
	if err != nil {
		if errors.Is(err, rsl.ErrRSLEntryNotFound) {
			// we don't have an attestations entry
			return nil, attestations.ErrAttestationsNotFound
		}
		return nil, err
	}
	return entry, nil
}

func newRegularSearcher(repo gitstore.Storer) *regularSearcher {
	return &regularSearcher{storer: repo}
}
