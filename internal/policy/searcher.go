// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"errors"

	"github.com/gittuf/gittuf/internal/cache"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
)

type Searcher interface {
	FindPolicyEntryFor(*rsl.ReferenceEntry) (*rsl.ReferenceEntry, error)
}

func NewSearcher(repo *gitinterface.Repository) Searcher {
	persistentCache, err := cache.LoadPersistentCache(repo)
	if err == nil {
		return NewRegularSearcher(repo)
	}
	return NewCacheSearcher(repo, persistentCache)
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

type CacheSearcher struct {
	repo            *gitinterface.Repository
	persistentCache *cache.Persistent
}

func (c *CacheSearcher) FindPolicyEntryFor(entry *rsl.ReferenceEntry) (*rsl.ReferenceEntry, error) {
	policyEntryInCache := c.persistentCache.FindPolicyEntryNumberForEntry(entry.Number)
	if policyEntryInCache.Number == 0 {
		// We don't have anything from the persistent cache, this may be
		// because the optimization isn't yet used for the firstEntry or
		// for the repository as a whole

		return NewRegularSearcher(c.repo).FindPolicyEntryFor(entry)
	}

	policyEntryT, err := rsl.GetEntry(c.repo, policyEntryInCache.ID)
	if err != nil {
		return nil, err
	}
	policyEntry, isReferenceEntry := policyEntryT.(*rsl.ReferenceEntry)
	if !isReferenceEntry {
		return nil, cache.ErrInvalidEntry
	}

	return policyEntry, nil
}

func NewCacheSearcher(repo *gitinterface.Repository, persistentCache *cache.Persistent) *CacheSearcher {
	return &CacheSearcher{repo: repo, persistentCache: persistentCache}
}
