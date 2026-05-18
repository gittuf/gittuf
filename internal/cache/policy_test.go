// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"testing"

	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHasPolicyEntryNumber(t *testing.T) {
	t.Run("no entries", func(t *testing.T) {
		p := &Persistent{}
		hash, has := p.HasPolicyEntryNumber(1)
		assert.False(t, has)
		assert.Equal(t, gitinterface.ZeroHash, hash)
	})

	t.Run("entries exist", func(t *testing.T) {
		p := &Persistent{
			PolicyEntries: []RSLEntryIndex{
				{EntryNumber: 1, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
				{EntryNumber: 2, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c1234"},
				{EntryNumber: 3, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c0980"},
				{EntryNumber: 4, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c9080"},
			},
		} // these hashes are random values that are used all accorss tests

		hash, has := p.HasPolicyEntryNumber(uint64(1))
		validHash, _ := gitinterface.NewHash("e69de29bb2d1d6434b8b29ae775ad8c2e48c5391")
		assert.True(t, has)
		assert.Equal(t, validHash, hash) // function retures the valid hash when a valid EntryNumber is given to it
	})

	t.Run("entry doesn't exist", func(t *testing.T) {
		p := &Persistent{
			PolicyEntries: []RSLEntryIndex{
				{EntryNumber: 2, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c1234"},
				{EntryNumber: 3, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c0980"},
				{EntryNumber: 4, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c9080"},
			},
		} // these hashes are random values that are used all accorss tests

		hash, has := p.HasPolicyEntryNumber(uint64(1))
		assert.False(t, has)
		assert.Equal(t, gitinterface.ZeroHash, hash) // function returns ZeroHash when entry doesn't exist
	})
}

func TestFindPolicyEntryNumberForEntry(t *testing.T) {
	t.Run("no entries", func(t *testing.T) {
		p := &Persistent{
			PolicyEntries: []RSLEntryIndex{},
		}
		index := p.FindPolicyEntryNumberForEntry(uint64(1))
		assert.Equal(t, RSLEntryIndex{EntryNumber: 0}, index)
	})

	t.Run("target entry exists", func(t *testing.T) {
		p := &Persistent{
			PolicyEntries: []RSLEntryIndex{
				{EntryNumber: 1, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
				{EntryNumber: 2, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c1234"},
				{EntryNumber: 3, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c0980"},
				{EntryNumber: 4, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c9080"},
			},
		}

		index := p.FindPolicyEntryNumberForEntry(uint64(2))
		assert.Equal(t, RSLEntryIndex{EntryNumber: 2, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c1234"}, index)
	})

	t.Run("target entry doesn't exist and there are no entries before it", func(t *testing.T) {
		p := &Persistent{
			PolicyEntries: []RSLEntryIndex{
				{EntryNumber: 2, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c1234"},
				{EntryNumber: 3, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c0980"},
				{EntryNumber: 4, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c9080"},
			},
		}
		index := p.FindPolicyEntryNumberForEntry(uint64(1))
		assert.Equal(t, RSLEntryIndex{EntryNumber: 0}, index) // when the target entry doesn't exist and there is no entry in the cache before the target, the function returns entryNumber 0
	})

	t.Run("target entry doesn't exist and there are entries before it", func(t *testing.T) {
		p := &Persistent{
			PolicyEntries: []RSLEntryIndex{
				{EntryNumber: 1, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
				{EntryNumber: 3, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c0980"},
				{EntryNumber: 4, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c9080"},
			},
		}

		index := p.FindPolicyEntryNumberForEntry(uint64(2))
		assert.Equal(t, RSLEntryIndex{EntryNumber: 1, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"}, index) // when the target entry doesn't exist but there are entries in the cache before the target entry number, the function returs the entry just before the target number
	})
}

func TestFindPolicyEntriesInRange(t *testing.T) {
	t.Run("no entries", func(t *testing.T) {
		p := &Persistent{
			PolicyEntries: []RSLEntryIndex{},
		}

		indices, err := p.FindPolicyEntriesInRange(uint64(1), uint64(3))
		assert.Nil(t, indices)
		assert.ErrorIs(t, ErrNoPersistentCache, err)
	})

	t.Run("entries exist in range", func(t *testing.T) {
		p := &Persistent{
			PolicyEntries: []RSLEntryIndex{
				{EntryNumber: 1, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
				{EntryNumber: 2, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c1234"},
				{EntryNumber: 3, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c0980"},
				{EntryNumber: 4, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c9080"},
			},
		}

		indices, err := p.FindPolicyEntriesInRange(uint64(2), uint64(4)) // checking range with both limits in bound
		require.Nil(t, err)
		require.Equal(t, []RSLEntryIndex{
			{EntryNumber: 2, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c1234"},
			{EntryNumber: 3, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c0980"},
			{EntryNumber: 4, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c9080"},
		}, indices)

		indices, err = p.FindPolicyEntriesInRange(uint64(2), uint64(6)) // checking range with last number outside bound
		require.Nil(t, err)
		require.Equal(t, []RSLEntryIndex{
			{EntryNumber: 2, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c1234"},
			{EntryNumber: 3, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c0980"},
			{EntryNumber: 4, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c9080"},
		}, indices)
	})
}

func TestInsertPolicyEntryNumber(t *testing.T) {
	t.Run("zero entry", func(t *testing.T) {
		p := &Persistent{
			PolicyEntries: []RSLEntryIndex{},
		}

		p.InsertPolicyEntryNumber(uint64(0), gitinterface.ZeroHash)
		assert.Equal(t, []RSLEntryIndex{}, p.PolicyEntries) // inserting entry number 0 should result in noop
	})

	t.Run("inserting in empty cache", func(t *testing.T) {
		p := &Persistent{
			PolicyEntries: []RSLEntryIndex{},
		}

		hash, _ := gitinterface.NewHash("e69de29bb2d1d6434b8b29ae775ad8c2e48c5391")
		p.InsertPolicyEntryNumber(uint64(1), hash)

		assert.Equal(t, []RSLEntryIndex{
			{EntryNumber: 1, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
		}, p.PolicyEntries)
	})

	t.Run("inserting at end", func(t *testing.T) {
		p := &Persistent{
			PolicyEntries: []RSLEntryIndex{
				{EntryNumber: 1, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
				{EntryNumber: 2, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c1234"},
				{EntryNumber: 3, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c0980"},
				{EntryNumber: 4, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c9080"},
			},
		}

		hash, _ := gitinterface.NewHash("e69de29bb2d1d6434b8b29ae775ad8c2e48d7914")
		p.InsertPolicyEntryNumber(uint64(5), hash)

		assert.Equal(t, []RSLEntryIndex{
			{EntryNumber: 1, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
			{EntryNumber: 2, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c1234"},
			{EntryNumber: 3, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c0980"},
			{EntryNumber: 4, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c9080"},
			{EntryNumber: 5, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48d7914"},
		}, p.PolicyEntries)
	})

	t.Run("inserting at insertion point", func(t *testing.T) {
		p := &Persistent{
			PolicyEntries: []RSLEntryIndex{
				{EntryNumber: 1, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
				{EntryNumber: 3, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c0980"},
				{EntryNumber: 4, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c9080"},
			},
		}

		hash, _ := gitinterface.NewHash("e69de29bb2d1d6434b8b29ae775ad8c2e48c1234")
		p.InsertPolicyEntryNumber(uint64(2), hash)

		assert.Equal(t, []RSLEntryIndex{
			{EntryNumber: 1, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
			{EntryNumber: 2, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c1234"},
			{EntryNumber: 3, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c0980"},
			{EntryNumber: 4, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c9080"},
		}, p.PolicyEntries)
	})
}

func TestGetPolicyEntries(t *testing.T) {
	p := &Persistent{
		PolicyEntries: []RSLEntryIndex{
			{EntryNumber: 1, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
			{EntryNumber: 2, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c1234"},
			{EntryNumber: 3, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c0980"},
			{EntryNumber: 4, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c9080"},
		},
	}

	entries := p.GetPolicyEntries()
	assert.Equal(t, p.PolicyEntries, entries)
}
