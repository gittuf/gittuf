// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/gittuf/gittuf/internal/attestations"
	"github.com/gittuf/gittuf/internal/cache"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/pkg/gitinterface"
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

func newSearcher(repo *gitinterface.Repository) searcher {
	persistentCache, err := cache.LoadPersistentCache(repo)
	if err == nil {
		slog.Debug("Persistent cache found, loading cache RSL searcher...")
		return newCacheSearcher(repo, persistentCache)
	}

	slog.Debug("Persistent cache not found, using regular RSL searcher...")

	return newRegularSearcher(repo)
}

// regularSearcher implements the searcher interface. It walks back the RSL from
// to identify the requested policy or attestation entries.
type regularSearcher struct {
	repo *gitinterface.Repository
}

// FindFirstPolicyEntry identifies the very first policy entry in the RSL.
func (r *regularSearcher) FindFirstPolicyEntry() (rsl.ReferenceUpdaterEntry, error) {
	entry, _, err := rsl.GetFirstReferenceUpdaterEntryForRef(r.repo, PolicyRef)
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
	entry, _, err := rsl.GetLatestReferenceUpdaterEntry(r.repo, rsl.ForReference(PolicyRef))
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

	policyEntry, _, err := rsl.GetLatestReferenceUpdaterEntry(r.repo, rsl.ForReference(PolicyRef), rsl.BeforeEntryID(entry.GetID()))
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
	allPolicyEntries, _, err := rsl.GetReferenceUpdaterEntriesInRangeForRef(r.repo, firstEntry.GetID(), lastEntry.GetID(), PolicyRef)
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

	attestationsEntry, _, err := rsl.GetLatestReferenceUpdaterEntry(r.repo, rsl.ForReference(attestations.Ref), rsl.BeforeEntryID(entry.GetID()))
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
	entry, _, err := rsl.GetLatestReferenceUpdaterEntry(r.repo, rsl.ForReference(attestations.Ref))
	if err != nil {
		if errors.Is(err, rsl.ErrRSLEntryNotFound) {
			// we don't have an attestations entry
			return nil, attestations.ErrAttestationsNotFound
		}
		return nil, err
	}
	return entry, nil
}

func newRegularSearcher(repo *gitinterface.Repository) *regularSearcher {
	return &regularSearcher{repo: repo}
}

// cacheSearcher implements the searcher interface. It checks the persistent
// cache for results before falling back to the regular searcher if the
// persistent cache yields no results.
type cacheSearcher struct {
	repo            *gitinterface.Repository
	persistentCache *cache.Persistent
	searcher        *regularSearcher
}

// FindFirstPolicyEntry identifies the very first policy entry in the RSL.
func (c *cacheSearcher) FindFirstPolicyEntry() (rsl.ReferenceUpdaterEntry, error) {
	if c.persistentCache == nil {
		return c.searcher.FindFirstPolicyEntry()
	}

	policyEntries := c.persistentCache.GetPolicyEntries()
	if len(policyEntries) == 0 {
		return nil, ErrPolicyNotFound
	}

	entry, err := loadRSLReferenceUpdaterEntry(c.repo, policyEntries[0].GetEntryID())
	if err != nil {
		return c.searcher.FindFirstPolicyEntry()
	}
	return entry, nil
}

func (c *cacheSearcher) FindLatestPolicyEntry() (rsl.ReferenceUpdaterEntry, error) {
	if c.persistentCache == nil {
		return c.searcher.FindLatestPolicyEntry()
	}

	policyEntries := c.persistentCache.GetPolicyEntries()
	if len(policyEntries) == 0 {
		return nil, ErrPolicyNotFound
	}

	entry, err := loadRSLReferenceUpdaterEntry(c.repo, policyEntries[len(policyEntries)-1].GetEntryID())
	if err != nil {
		return c.searcher.FindLatestPolicyEntry()
	}
	return entry, nil
}

// FindPolicyEntryFor identifies the latest policy entry for the specified
// entry.
func (c *cacheSearcher) FindPolicyEntryFor(entry rsl.Entry) (rsl.ReferenceUpdaterEntry, error) {
	if c.persistentCache == nil {
		slog.Debug("No persistent cache found, falling back to regular searcher...")
		return c.searcher.FindPolicyEntryFor(entry)
	}

	if entry.GetNumber() == 0 {
		// no number is set
		slog.Debug("Entry is not numbered, falling back to regular searcher...")
		return c.searcher.FindPolicyEntryFor(entry)
	}

	if entry, isReferenceUpdaterEntry := entry.(rsl.ReferenceUpdaterEntry); isReferenceUpdaterEntry && entry.GetRefName() == PolicyRef {
		slog.Debug("Requested entry is a policy entry, inserting into cache...")
		c.persistentCache.InsertPolicyEntryNumber(entry.GetNumber(), entry.GetID())

		return entry, nil
	}

	policyEntryIndex := c.persistentCache.FindPolicyEntryNumberForEntry(entry.GetNumber())
	if policyEntryIndex.GetEntryNumber() == 0 {
		return nil, ErrPolicyNotFound
	}

	policyEntry, err := loadRSLReferenceUpdaterEntry(c.repo, policyEntryIndex.GetEntryID())
	if err != nil {
		return c.searcher.FindPolicyEntryFor(entry)
	}
	return policyEntry, nil
}

// FindPolicyEntriesInRange returns all policy RSL entries in the specified
// range. firstEntry and lastEntry are included if they are for the policy ref.
func (c *cacheSearcher) FindPolicyEntriesInRange(firstEntry, lastEntry rsl.Entry) ([]rsl.ReferenceUpdaterEntry, error) {
	if c.persistentCache == nil {
		return c.searcher.FindPolicyEntriesInRange(firstEntry, lastEntry)
	}

	if lastEntry.GetNumber() == 0 || firstEntry.GetNumber() == 0 {
		// first or last entry doesn't have a number
		return c.searcher.FindPolicyEntriesInRange(firstEntry, lastEntry)
	}

	if firstEntry, isReferenceUpdaterEntry := firstEntry.(rsl.ReferenceUpdaterEntry); isReferenceUpdaterEntry && firstEntry.GetRefName() == PolicyRef {
		slog.Debug("Requested first entry is a policy entry, inserting into cache...")
		c.persistentCache.InsertPolicyEntryNumber(firstEntry.GetNumber(), firstEntry.GetID())
	}
	if lastEntry, isReferenceUpdaterEntry := lastEntry.(rsl.ReferenceUpdaterEntry); isReferenceUpdaterEntry && lastEntry.GetRefName() == PolicyRef {
		slog.Debug("Requested last entry is a policy entry, inserting into cache...")
		c.persistentCache.InsertPolicyEntryNumber(lastEntry.GetNumber(), lastEntry.GetID())
	}

	policyIndices, err := c.persistentCache.FindPolicyEntriesInRange(firstEntry.GetNumber(), lastEntry.GetNumber())
	if err != nil {
		return c.searcher.FindPolicyEntriesInRange(firstEntry, lastEntry)
	}

	entries := []rsl.ReferenceUpdaterEntry{}
	for _, index := range policyIndices {
		entry, err := loadRSLReferenceUpdaterEntry(c.repo, index.GetEntryID())
		if err != nil {
			return c.searcher.FindPolicyEntriesInRange(firstEntry, lastEntry)
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// FindAttestationsEntryFor identifies the latest attestations entry for the
// specified entry.
func (c *cacheSearcher) FindAttestationsEntryFor(entry rsl.Entry) (rsl.ReferenceUpdaterEntry, error) {
	if c.persistentCache == nil {
		slog.Debug("No persistent cache found, falling back to regular searcher...")
		return c.searcher.FindAttestationsEntryFor(entry)
	}

	if entry.GetNumber() == 0 {
		// no number is set
		slog.Debug("Entry is not numbered, falling back to regular searcher...")
		return c.searcher.FindAttestationsEntryFor(entry)
	}

	if entry, isReferenceUpdaterEntry := entry.(rsl.ReferenceUpdaterEntry); isReferenceUpdaterEntry && entry.GetRefName() == attestations.Ref {
		slog.Debug("Requested entry is an attestations entry, inserting into cache...")
		c.persistentCache.InsertAttestationEntryNumber(entry.GetNumber(), entry.GetID())

		return entry, nil
	}

	attestationsEntryIndex, _ := c.persistentCache.FindAttestationsEntryNumberForEntry(entry.GetNumber())
	if attestationsEntryIndex.GetEntryNumber() == 0 {
		return nil, attestations.ErrAttestationsNotFound
	}

	attestationsEntry, err := loadRSLReferenceUpdaterEntry(c.repo, attestationsEntryIndex.GetEntryID())
	if err != nil {
		return c.searcher.FindAttestationsEntryFor(entry)
	}
	return attestationsEntry, nil
}

func (c *cacheSearcher) FindLatestAttestationsEntry() (rsl.ReferenceUpdaterEntry, error) {
	if c.persistentCache == nil {
		return c.searcher.FindLatestAttestationsEntry()
	}

	attestationsEntries := c.persistentCache.GetAttestationsEntries()
	if len(attestationsEntries) == 0 {
		return nil, attestations.ErrAttestationsNotFound
	}

	entry, err := loadRSLReferenceUpdaterEntry(c.repo, attestationsEntries[len(attestationsEntries)-1].GetEntryID())
	if err != nil {
		return c.searcher.FindLatestAttestationsEntry()
	}
	return entry, nil
}

func newCacheSearcher(repo *gitinterface.Repository, persistentCache *cache.Persistent) *cacheSearcher {
	return &cacheSearcher{
		repo:            repo,
		persistentCache: persistentCache,
		searcher:        newRegularSearcher(repo),
	}
}

func loadRSLReferenceUpdaterEntry(repo *gitinterface.Repository, entryID gitinterface.Hash) (rsl.ReferenceUpdaterEntry, error) {
	entryT, err := rsl.GetEntry(repo, entryID)
	if err != nil {
		return nil, err
	}

	entry, isReferenceEntry := entryT.(*rsl.ReferenceEntry)
	if !isReferenceEntry {
		return nil, fmt.Errorf("not reference entry")
	}

	return entry, nil
}
