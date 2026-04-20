// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"testing"

	"github.com/gittuf/gittuf/internal/attestations"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommit(t *testing.T) {
	t.Run("empty cache", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		p := &Persistent{}
		err := p.Commit(repo)
		assert.Nil(t, err) // an empty cache should do nothing

		_, err = repo.GetReference(Ref)
		assert.Error(t, err) // no reference should be created for an empty cache
	})

	t.Run("non-empty cache", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		p := &Persistent{
			PolicyEntries: []RSLEntryIndex{
				{EntryNumber: 1, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
			},
		}

		err := p.Commit(repo)
		require.Nil(t, err) // there should be no error for non-empty cache

		refID, err := repo.GetReference(Ref)
		require.Nil(t, err)             // a reference should be created for a non-empty cache
		assert.False(t, refID.IsZero()) // the reference should not be zero
	})

	t.Run("no changes causes noop", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		p := &Persistent{
			AttestationEntries: []RSLEntryIndex{
				{EntryNumber: 1, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
			},
		}

		err := p.Commit(repo)
		require.Nil(t, err)

		firstRef, err := repo.GetReference(Ref)
		require.Nil(t, err)

		err = p.Commit(repo)
		require.Nil(t, err)

		secondRef, err := repo.GetReference(Ref)
		require.Nil(t, err)

		assert.Equal(t, firstRef, secondRef) // the references should be equal to each other because noop
	})

	t.Run("cache changes causes new commit", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		p := &Persistent{
			AttestationEntries: []RSLEntryIndex{
				{EntryNumber: 1, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
			},
		}

		err := p.Commit(repo)
		require.Nil(t, err)

		firstRef, err := repo.GetReference(Ref)
		require.Nil(t, err)

		p.PolicyEntries = append(p.PolicyEntries, RSLEntryIndex{
			EntryNumber: 2,
			EntryID:     "e69de29bb2d1d6434b8b29ae775ad8c2e48c1234",
		})

		err = p.Commit(repo)
		require.Nil(t, err)

		secondRef, err := repo.GetReference(Ref)
		require.Nil(t, err)

		assert.NotEqual(t, firstRef, secondRef) // the reference should not be equal when data changes
	})
}

func TestBinarySearch(t *testing.T) {
	t.Run("equals", func(t *testing.T) {
		a := RSLEntryIndex{EntryNumber: 1, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"}
		b := RSLEntryIndex{EntryNumber: 1, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c1234"}

		c := RSLEntryIndex{EntryNumber: 2, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c9080"}
		d := RSLEntryIndex{EntryNumber: 2, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c9080"}

		res := binarySearch(a, b)
		assert.Equal(t, 0, res) // the result should be 0 for equal entry numbers and unequal IDs

		res = binarySearch(c, d)
		assert.Equal(t, 0, res) // the result should be 0 for equal entry numbers and equal IDs
	})

	t.Run("precedes", func(t *testing.T) {
		a := RSLEntryIndex{EntryNumber: 0, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"}
		b := RSLEntryIndex{EntryNumber: 1, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c1234"}

		c := RSLEntryIndex{EntryNumber: 2, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c9080"}
		d := RSLEntryIndex{EntryNumber: 3, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c9080"}

		res := binarySearch(a, b)
		assert.Equal(t, -1, res) // the result should be -1 for cases with unequal IDs but first entry number is smaller than the second

		res = binarySearch(c, d)
		assert.Equal(t, -1, res) // the result should be -1 for cases with equal IDs but first entry number is smaller than the second
	})

	t.Run("succeeds", func(t *testing.T) {
		a := RSLEntryIndex{EntryNumber: 0, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"}
		b := RSLEntryIndex{EntryNumber: 1, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c1234"}

		c := RSLEntryIndex{EntryNumber: 2, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c9080"}
		d := RSLEntryIndex{EntryNumber: 3, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c9080"}

		res := binarySearch(b, a)
		assert.Equal(t, 1, res) // the result should be 1 for cases with unequal IDs but second entry number is smaller than the first

		res = binarySearch(d, c)
		assert.Equal(t, 1, res) // the result should be 1 for cases with equal IDs but second entry number is smaller than the first
	})
}

func TestPopulatePersistentCache(t *testing.T) {
	t.Run("repo with reference and policy", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		err := rsl.NewReferenceEntry(policyRef, gitinterface.ZeroHash).Commit(repo, false)
		require.Nil(t, err)

		err = PopulatePersistentCache(repo)
		assert.Nil(t, err)

		sampleCache := Persistent{
			PolicyEntries: []RSLEntryIndex{
				{EntryNumber: 1, EntryID: "a2f603abf945588e8ad0d6b1f71a37bdcaf87e13"},
			},
			AttestationEntries:            []RSLEntryIndex{},
			AddedAttestationsBeforeNumber: 1,
		}
		cache, err := LoadPersistentCache(repo)
		require.Nil(t, err)
		assert.Equal(t, &sampleCache, cache)
	})

	t.Run("empty repo", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		err := PopulatePersistentCache(repo) // since this empty repo has no references or commits yet so it should throw an error
		require.Error(t, err)
	})
}

func TestLoadPersistantCache(t *testing.T) {
	t.Run("empty repo", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		_, err := LoadPersistentCache(repo)
		require.ErrorIs(t, ErrNoPersistentCache, err) // empty repo with no reference throws ErrNoPersistentCache error
	})

	t.Run("repo with policy reference", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		err := rsl.NewReferenceEntry(policyRef, gitinterface.ZeroHash).Commit(repo, false)
		require.Nil(t, err)

		err = PopulatePersistentCache(repo)
		require.Nil(t, err)

		_, err = repo.GetReference(Ref)
		require.Nil(t, err)

		persistentCache, err := LoadPersistentCache(repo)
		require.Nil(t, err)
		assert.NotNil(t, persistentCache) // the cache should be loaded successfully if the persistentCache exists

		sampleCache := Persistent{
			PolicyEntries: []RSLEntryIndex{
				{EntryNumber: 1, EntryID: "a2f603abf945588e8ad0d6b1f71a37bdcaf87e13"},
			},
			AttestationEntries:            []RSLEntryIndex{},
			AddedAttestationsBeforeNumber: 1,
		}
		assert.Equal(t, &sampleCache, persistentCache)
	})

	t.Run("repo with attestation reference", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		err := rsl.NewReferenceEntry(attestations.Ref, gitinterface.ZeroHash).Commit(repo, false)
		require.Nil(t, err)

		err = PopulatePersistentCache(repo)
		require.Nil(t, err)

		_, err = repo.GetReference(Ref)
		require.Nil(t, err)

		persistentCache, err := LoadPersistentCache(repo)
		require.Nil(t, err)
		assert.NotNil(t, persistentCache)

		sampleCache := Persistent{
			PolicyEntries: []RSLEntryIndex{},
			AttestationEntries: []RSLEntryIndex{
				{EntryNumber: 1, EntryID: "4f6fd1c67daa2acf4f2dd8626ddd8d6a51fcd026"},
			},
			AddedAttestationsBeforeNumber: 1,
		}
		assert.Equal(t, &sampleCache, persistentCache)
	})
}

func TestDeletePersistentCache(t *testing.T) {
	t.Run("delete cache for empty repo", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		err := DeletePersistentCache(repo)
		require.ErrorIs(t, ErrNoPersistentCache, err)
	})

	t.Run("delete existing cache", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		err := rsl.NewReferenceEntry(policyRef, gitinterface.ZeroHash).Commit(repo, false)
		require.Nil(t, err)

		err = PopulatePersistentCache(repo)
		require.Nil(t, err)

		err = DeletePersistentCache(repo)
		assert.Nil(t, err)

		_, err = repo.GetReference(Ref)
		assert.ErrorIs(t, gitinterface.ErrReferenceNotFound, err)
	})
}

func TestRSLEntryMethods(t *testing.T) {
	r := RSLEntryIndex{EntryNumber: 1, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"}

	number := r.GetEntryNumber()
	assert.Equal(t, uint64(1), number)

	id := r.GetEntryID()
	expectedID, _ := gitinterface.NewHash("e69de29bb2d1d6434b8b29ae775ad8c2e48c5391")
	assert.Equal(t, expectedID, id)
}
