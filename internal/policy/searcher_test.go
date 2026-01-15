// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"testing"

	"github.com/gittuf/gittuf/internal/attestations"
	"github.com/gittuf/gittuf/internal/cache"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestRegularSearcher(t *testing.T) {
	t.Run("policy exists", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithOnlyRoot)

		expectedPolicyEntry, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}
		// Add an entry after
		if err := rsl.NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		entry, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		searcher := newRegularSearcher(repo)
		policyEntry, err := searcher.FindPolicyEntryFor(entry)
		assert.Nil(t, err)
		assert.Equal(t, expectedPolicyEntry.GetID(), policyEntry.GetID())

		// Try with annotation
		if err := rsl.NewAnnotationEntry([]gitinterface.Hash{entry.GetID()}, false, "Annotation\n").Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		policyEntry, err = searcher.FindPolicyEntryFor(entry)
		assert.Nil(t, err)
		assert.Equal(t, expectedPolicyEntry.GetID(), policyEntry.GetID())

		// Requested entry is policy entry
		if err := rsl.NewReferenceEntry(PolicyRef, gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		entry, err = rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		policyEntry, err = searcher.FindPolicyEntryFor(entry)
		assert.Nil(t, err)
		assert.Equal(t, entry.GetID(), policyEntry.GetID())
	})

	t.Run("policy does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		// Add an entry after
		if err := rsl.NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		entry, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		searcher := newRegularSearcher(repo)
		policyEntry, err := searcher.FindPolicyEntryFor(entry)
		assert.ErrorIs(t, err, ErrPolicyNotFound)
		assert.Nil(t, policyEntry)
	})

	t.Run("first policy", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithOnlyRoot)

		expectedPolicyEntry, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		searcher := newRegularSearcher(repo)
		policyEntry, err := searcher.FindFirstPolicyEntry()
		assert.Nil(t, err)
		assert.Equal(t, expectedPolicyEntry.GetID(), policyEntry.GetID())
	})

	t.Run("policies in range", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithOnlyRoot)

		latestEntry, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}
		firstEntry := latestEntry

		expectedPolicyEntries := []rsl.ReferenceUpdaterEntry{latestEntry.(*rsl.ReferenceEntry)}

		searcher := newRegularSearcher(repo)

		policyEntries, err := searcher.FindPolicyEntriesInRange(firstEntry, latestEntry)
		assert.Nil(t, err)
		assert.Equal(t, expectedPolicyEntries, policyEntries)

		if err := rsl.NewReferenceEntry(PolicyRef, gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		latestEntry, err = rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}
		expectedPolicyEntries = append(expectedPolicyEntries, latestEntry.(*rsl.ReferenceEntry))

		policyEntries, err = searcher.FindPolicyEntriesInRange(firstEntry, latestEntry)
		assert.Nil(t, err)
		assert.Equal(t, expectedPolicyEntries, policyEntries)

		if err := rsl.NewReferenceEntry(PolicyStagingRef, gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		latestEntry, err = rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		// expectedPolicyEntries does not change in this instance
		policyEntries, err = searcher.FindPolicyEntriesInRange(firstEntry, latestEntry)
		assert.Nil(t, err)
		assert.Equal(t, expectedPolicyEntries, policyEntries)
	})

	t.Run("attestations exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		currentAttestations := &attestations.Attestations{}
		if err := currentAttestations.Commit(repo, "Initial attestations\n", true, false); err != nil {
			t.Fatal(err)
		}

		expectedAttestationsEntry, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		// Add an entry after
		if err := rsl.NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		entry, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		searcher := newRegularSearcher(repo)
		attestationsEntry, err := searcher.FindAttestationsEntryFor(entry)
		assert.Nil(t, err)
		assert.Equal(t, expectedAttestationsEntry.GetID(), attestationsEntry.GetID())

		// Try with annotation
		if err := rsl.NewAnnotationEntry([]gitinterface.Hash{entry.GetID()}, false, "Annotation\n").Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		attestationsEntry, err = searcher.FindAttestationsEntryFor(entry)
		assert.Nil(t, err)
		assert.Equal(t, expectedAttestationsEntry.GetID(), attestationsEntry.GetID())

		// Requested entry is attestations entry
		if err := rsl.NewReferenceEntry(attestations.Ref, gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		entry, err = rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		attestationsEntry, err = searcher.FindAttestationsEntryFor(entry)
		assert.Nil(t, err)
		assert.Equal(t, entry.GetID(), attestationsEntry.GetID())
	})

	t.Run("attestations do not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		// Add an entry after
		if err := rsl.NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		entry, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		searcher := newRegularSearcher(repo)
		attestationsEntry, err := searcher.FindAttestationsEntryFor(entry)
		assert.ErrorIs(t, err, attestations.ErrAttestationsNotFound)
		assert.Nil(t, attestationsEntry)
	})
}

func TestCacheSearcher(t *testing.T) {
	t.Run("policy exists", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithOnlyRoot)

		if err := cache.PopulatePersistentCache(repo); err != nil {
			t.Fatal(err)
		}
		persistentCache, err := cache.LoadPersistentCache(repo)
		if err != nil {
			t.Fatal(err)
		}

		expectedPolicyEntry, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}
		// Add an entry after
		if err := rsl.NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		entry, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		searcher := newCacheSearcher(repo, persistentCache)
		policyEntry, err := searcher.FindPolicyEntryFor(entry)
		assert.Nil(t, err)
		assert.Equal(t, expectedPolicyEntry.GetID(), policyEntry.GetID())

		// Try with annotation
		if err := rsl.NewAnnotationEntry([]gitinterface.Hash{entry.GetID()}, false, "Annotation\n").Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		policyEntry, err = searcher.FindPolicyEntryFor(entry)
		assert.Nil(t, err)
		assert.Equal(t, expectedPolicyEntry.GetID(), policyEntry.GetID())

		// Requested entry is policy entry
		if err := rsl.NewReferenceEntry(PolicyRef, gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		entry, err = rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		policyEntry, err = searcher.FindPolicyEntryFor(entry)
		assert.Nil(t, err)
		assert.Equal(t, entry.GetID(), policyEntry.GetID())
	})

	t.Run("policy does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()

		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		// Add an entry after
		if err := rsl.NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		if err := cache.PopulatePersistentCache(repo); err != nil {
			t.Fatal(err)
		}
		persistentCache, err := cache.LoadPersistentCache(repo)
		if err != nil {
			t.Fatal(err)
		}

		entry, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		searcher := newCacheSearcher(repo, persistentCache)
		policyEntry, err := searcher.FindPolicyEntryFor(entry)
		assert.ErrorIs(t, err, ErrPolicyNotFound)
		assert.Nil(t, policyEntry)
	})

	t.Run("first policy", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithOnlyRoot)

		if err := cache.PopulatePersistentCache(repo); err != nil {
			t.Fatal(err)
		}
		persistentCache, err := cache.LoadPersistentCache(repo)
		if err != nil {
			t.Fatal(err)
		}

		expectedPolicyEntry, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		searcher := newCacheSearcher(repo, persistentCache)
		policyEntry, err := searcher.FindFirstPolicyEntry()
		assert.Nil(t, err)
		assert.Equal(t, expectedPolicyEntry.GetID(), policyEntry.GetID())
	})

	t.Run("policies in range", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithOnlyRoot)

		if err := cache.PopulatePersistentCache(repo); err != nil {
			t.Fatal(err)
		}
		persistentCache, err := cache.LoadPersistentCache(repo)
		if err != nil {
			t.Fatal(err)
		}

		latestEntry, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}
		firstEntry := latestEntry

		expectedPolicyEntries := []rsl.ReferenceUpdaterEntry{latestEntry.(*rsl.ReferenceEntry)}

		searcher := newCacheSearcher(repo, persistentCache)

		policyEntries, err := searcher.FindPolicyEntriesInRange(firstEntry, latestEntry)
		assert.Nil(t, err)
		assert.Equal(t, expectedPolicyEntries, policyEntries)

		if err := rsl.NewReferenceEntry(PolicyRef, gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		latestEntry, err = rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}
		expectedPolicyEntries = append(expectedPolicyEntries, latestEntry.(*rsl.ReferenceEntry))

		policyEntries, err = searcher.FindPolicyEntriesInRange(firstEntry, latestEntry)
		assert.Nil(t, err)
		assert.Equal(t, expectedPolicyEntries, policyEntries)

		if err := rsl.NewReferenceEntry(PolicyStagingRef, gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		latestEntry, err = rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		// expectedPolicyEntries does not change in this instance
		policyEntries, err = searcher.FindPolicyEntriesInRange(firstEntry, latestEntry)
		assert.Nil(t, err)
		assert.Equal(t, expectedPolicyEntries, policyEntries)
	})

	t.Run("attestations exist", func(t *testing.T) {
		tmpDir := t.TempDir()

		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		currentAttestations := &attestations.Attestations{}
		if err := currentAttestations.Commit(repo, "Initial attestations\n", true, false); err != nil {
			t.Fatal(err)
		}

		expectedAttestationsEntry, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		// Add an entry after
		if err := rsl.NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		entry, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		if err := cache.PopulatePersistentCache(repo); err != nil {
			t.Fatal(err)
		}
		persistentCache, err := cache.LoadPersistentCache(repo)
		if err != nil {
			t.Fatal(err)
		}

		searcher := newCacheSearcher(repo, persistentCache)
		attestationsEntry, err := searcher.FindAttestationsEntryFor(entry)
		assert.Nil(t, err)
		assert.Equal(t, expectedAttestationsEntry.GetID(), attestationsEntry.GetID())

		// Try with annotation
		if err := rsl.NewAnnotationEntry([]gitinterface.Hash{entry.GetID()}, false, "Annotation\n").Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		attestationsEntry, err = searcher.FindAttestationsEntryFor(entry)
		assert.Nil(t, err)
		assert.Equal(t, expectedAttestationsEntry.GetID(), attestationsEntry.GetID())

		// Requested entry is annotations entry
		if err := rsl.NewReferenceEntry(attestations.Ref, gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		entry, err = rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		attestationsEntry, err = searcher.FindAttestationsEntryFor(entry)
		assert.Nil(t, err)
		assert.Equal(t, entry.GetID(), attestationsEntry.GetID())
	})

	t.Run("attestations do not exist", func(t *testing.T) {
		tmpDir := t.TempDir()

		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		// Add an entry after
		if err := rsl.NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		entry, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		if err := cache.PopulatePersistentCache(repo); err != nil {
			t.Fatal(err)
		}
		persistentCache, err := cache.LoadPersistentCache(repo)
		if err != nil {
			t.Fatal(err)
		}

		searcher := newCacheSearcher(repo, persistentCache)
		attestationsEntry, err := searcher.FindAttestationsEntryFor(entry)
		assert.ErrorIs(t, err, attestations.ErrAttestationsNotFound)
		assert.Nil(t, attestationsEntry)
	})
}
