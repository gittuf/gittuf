// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package rsl

import (
	"testing"

	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestRSLCache(t *testing.T) {
	// Add test entries
	// Using fake hashes (these are commits in the gittuf repo itself)
	entry1 := NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash)
	entry1.Number = 1
	hash, err := gitinterface.NewHash("4dcd174e182cedf597b8a84f24ea5a53dae7e1e7")
	if err != nil {
		t.Fatal(err)
	}
	entry1.ID = hash

	entry2 := NewReferenceEntry("refs/heads/feature", gitinterface.ZeroHash)
	entry2.Number = 2
	hash, err = gitinterface.NewHash("5bf80ffecacfde7e6b8281e65223b139a76160e1")
	if err != nil {
		t.Fatal(err)
	}
	entry2.ID = hash

	// Nothing yet in the parent cache
	assert.Empty(t, cache.parentCache)

	// Test set and get for parent-child cache
	cache.setParent(entry2.ID, entry1.ID)
	assert.Equal(t, entry1.ID.String(), cache.parentCache[entry2.ID.String()])
	assert.Equal(t, 1, len(cache.parentCache))

	parentID, has, err := cache.getParent(entry2.ID)
	assert.Nil(t, err)
	assert.Equal(t, entry1.ID.String(), parentID.String())
	assert.True(t, has)

	_, has, err = cache.getParent(entry1.ID) // not in cache
	assert.Nil(t, err)
	assert.False(t, has)

	// Nothing yet in the entry cache
	assert.Empty(t, cache.entryCache)

	// Test set and get for entry cache
	cache.setEntry(entry1.ID, entry1)
	assert.Equal(t, entry1, cache.entryCache[entry1.ID.String()])
	assert.Equal(t, 1, len(cache.entryCache))

	entry, has := cache.getEntry(entry1.ID)
	assert.Equal(t, entry1, entry)
	assert.True(t, has)
}
