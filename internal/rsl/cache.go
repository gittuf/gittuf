// SPDX-License-Identifier: Apache-2.0

package rsl

import "github.com/gittuf/gittuf/internal/gitinterface"

type rslCache struct {
	entryCache  map[string]Entry
	parentCache map[string]string
}

func (r *rslCache) getEntry(id gitinterface.Hash) (Entry, bool) {
	entry, has := r.entryCache[id.String()]
	return entry, has
}

func (r *rslCache) setEntry(id gitinterface.Hash, entry Entry) {
	r.entryCache[id.String()] = entry
}

func (r *rslCache) getParent(id gitinterface.Hash) (gitinterface.Hash, bool) {
	parentID, has := r.parentCache[id.String()]
	if !has {
		return nil, false
	}

	parentIDHash, _ := gitinterface.NewHash(parentID)
	return parentIDHash, true
}

func (r *rslCache) setParent(id, parentID gitinterface.Hash) {
	r.parentCache[id.String()] = parentID.String()
}

var cache *rslCache

func newRSLCache() {
	cache = &rslCache{
		entryCache:  map[string]Entry{},
		parentCache: map[string]string{},
	}
}

func init() {
	newRSLCache()
}
