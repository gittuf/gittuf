// SPDX-License-Identifier: Apache-2.0

package rsl

import (
	"testing"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestRSLCache(t *testing.T) {
	// Create repo to add test entries to
	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	// Add test entries
	// Note: We want to be careful in identifying their IDs not to use a method
	// that populates the cache
	if err := NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
		t.Fatal(err)
	}
	entry1ID, err := repo.GetReference(Ref)
	if err != nil {
		t.Fatal(err)
	}

	if err := NewReferenceEntry("refs/heads/feature", gitinterface.ZeroHash).Commit(repo, false); err != nil {
		t.Fatal(err)
	}
	entry2ID, err := repo.GetReference(Ref)
	if err != nil {
		t.Fatal(err)
	}

	// Nothing yet in the parent cache
	assert.Empty(t, cache.parentCache)

	// Test set and get for parent-child cache
	cache.setParent(entry2ID, entry1ID)
	assert.Equal(t, entry1ID.String(), cache.parentCache[entry2ID.String()])
	assert.Equal(t, 1, len(cache.parentCache))

	parentID, has, err := cache.getParent(entry2ID)
	assert.Nil(t, err)
	assert.Equal(t, entry1ID.String(), parentID.String())
	assert.True(t, has)

	_, has, err = cache.getParent(entry1ID) // not in cache
	assert.Nil(t, err)
	assert.False(t, has)

	// Nothing yet in the entry cache
	assert.Empty(t, cache.entryCache)

	// Test set and get for entry cache
	// Note: As before, we want to be careful not to populate the cache when we
	// load the entries
	message, err := repo.GetCommitMessage(entry1ID)
	if err != nil {
		t.Fatal(err)
	}
	entry1, err := parseRSLEntryText(entry1ID, message)
	if err != nil {
		t.Fatal(err)
	}

	cache.setEntry(entry1ID, entry1)
	assert.Equal(t, entry1, cache.entryCache[entry1ID.String()])
	assert.Equal(t, 1, len(cache.entryCache))

	entry, has := cache.getEntry(entry1ID)
	assert.Equal(t, entry1, entry)
	assert.True(t, has)
}
