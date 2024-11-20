// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/gittuf/gittuf/internal/attestations"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
)

// searcher defines the interface for finding policy and attestation entries in
// the RSL.
type searcher interface {
	FindFirstPolicyEntry() (*rsl.ReferenceEntry, error)
	FindPolicyEntryFor(rsl.Entry) (*rsl.ReferenceEntry, error)
	FindPolicyEntriesInRange(rsl.Entry, rsl.Entry) ([]*rsl.ReferenceEntry, error)
	FindAttestationsEntryFor(rsl.Entry) (*rsl.ReferenceEntry, error)
}

func newSearcher(repo *gitinterface.Repository) searcher {
	return newRegularSearcher(repo)
}

// regularSearcher implements the searcher interface. It walks back the RSL from
// the specified entry to find the latest policy or attestations entry.
type regularSearcher struct {
	repo *gitinterface.Repository
}

// FindFirstPolicyEntry identifies the very first policy entry in the RSL.
func (r *regularSearcher) FindFirstPolicyEntry() (*rsl.ReferenceEntry, error) {
	entry, _, err := rsl.GetFirstReferenceEntryForRef(r.repo, PolicyRef)
	if err != nil {
		if errors.Is(err, rsl.ErrRSLEntryNotFound) {
			// we don't have a policy entry yet
			return nil, ErrPolicyNotFound
		}
		return nil, err
	}

	return entry, nil
}

// FindPolicyEntryFor identifies the latest policy entry for the specified
// entry.
func (r *regularSearcher) FindPolicyEntryFor(entry rsl.Entry) (*rsl.ReferenceEntry, error) {
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

// FindPolicyEntriesInRange returns all policy RSL entries in the specified
// range. firstEntry and lastEntry are included if they are for the policy ref.
func (r *regularSearcher) FindPolicyEntriesInRange(firstEntry, lastEntry rsl.Entry) ([]*rsl.ReferenceEntry, error) {
	allPolicyEntries, _, err := rsl.GetReferenceEntriesInRangeForRef(r.repo, firstEntry.GetID(), lastEntry.GetID(), PolicyRef, true)
	if err != nil {
		return nil, err
	}

	return allPolicyEntries, nil
}

// FindAttestationsEntryFor identifies the latest attestations entry for the
// specified entry.
func (r *regularSearcher) FindAttestationsEntryFor(entry rsl.Entry) (*rsl.ReferenceEntry, error) {
	// If the requested entry itself is for the attestations ref, return as is
	if entry, isReferenceEntry := entry.(*rsl.ReferenceEntry); isReferenceEntry && entry.RefName == attestations.Ref {
		slog.Debug(fmt.Sprintf("Initial entry '%s' is for attestations, setting that as current set of attestations...", entry.GetID().String()))
		return entry, nil
	}

	attestationsEntry, _, err := rsl.GetLatestReferenceEntry(r.repo, rsl.ForReference(attestations.Ref), rsl.BeforeEntryID(entry.GetID()))
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

func newRegularSearcher(repo *gitinterface.Repository) *regularSearcher {
	return &regularSearcher{repo: repo}
}
