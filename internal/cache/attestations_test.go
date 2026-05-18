// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"testing"

	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestFindAttestationsEntryNumberForEntry(t *testing.T) {
	t.Run("no entries", func(t *testing.T) {
		p := &Persistent{
			AttestationEntries: []RSLEntryIndex{},
		}

		index, _ := p.FindAttestationsEntryNumberForEntry(uint64(1))
		assert.Equal(t, RSLEntryIndex{EntryNumber: 0}, index)
	})

	t.Run("target entry exists", func(t *testing.T) {
		p := &Persistent{
			AttestationEntries: []RSLEntryIndex{
				{EntryNumber: 1, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
				{EntryNumber: 2, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c1234"},
				{EntryNumber: 3, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c0980"},
				{EntryNumber: 4, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c9080"},
			},
		}

		index, _ := p.FindAttestationsEntryNumberForEntry(uint64(2))
		assert.Equal(t, RSLEntryIndex{EntryNumber: 2, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c1234"}, index)
	})

	t.Run("target entry doesn't exist and there are no entries before it", func(t *testing.T) {
		p := &Persistent{
			AttestationEntries: []RSLEntryIndex{
				{EntryNumber: 2, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c1234"},
				{EntryNumber: 3, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c0980"},
				{EntryNumber: 4, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c9080"},
			},
		}
		index, _ := p.FindAttestationsEntryNumberForEntry(uint64(1))
		assert.Equal(t, RSLEntryIndex{EntryNumber: 0}, index) // when the target entry doesn't exist and there is no entry in the cache before the target, the function returns entryNumber 0
	})

	t.Run("target entry doesn't exist and there are entries before it", func(t *testing.T) {
		p := &Persistent{
			AttestationEntries: []RSLEntryIndex{
				{EntryNumber: 1, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
				{EntryNumber: 3, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c0980"},
				{EntryNumber: 4, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c9080"},
			},
		}

		index, _ := p.FindAttestationsEntryNumberForEntry(uint64(2))
		assert.Equal(t, RSLEntryIndex{EntryNumber: 1, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"}, index) // when the target entry doesn't exist but there are entries in the cache before the target entry number, the function returs the entry just before the target number
	})
}

func TestInsertAttestationEntryNumber(t *testing.T) {
	t.Run("zero entry", func(t *testing.T) {
		p := &Persistent{
			AttestationEntries: []RSLEntryIndex{},
		}

		p.InsertAttestationEntryNumber(uint64(0), gitinterface.ZeroHash)
		assert.Equal(t, []RSLEntryIndex{}, p.AttestationEntries) // inserting entry number 0 should result in noop
	})

	t.Run("inserting in empty cache", func(t *testing.T) {
		p := &Persistent{
			AttestationEntries: []RSLEntryIndex{},
		}

		hash, _ := gitinterface.NewHash("e69de29bb2d1d6434b8b29ae775ad8c2e48c5391")
		p.InsertAttestationEntryNumber(uint64(1), hash)

		assert.Equal(t, []RSLEntryIndex{
			{EntryNumber: 1, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
		}, p.AttestationEntries)
	})

	t.Run("inserting at end", func(t *testing.T) {
		p := &Persistent{
			AttestationEntries: []RSLEntryIndex{
				{EntryNumber: 1, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
				{EntryNumber: 2, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c1234"},
				{EntryNumber: 3, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c0980"},
				{EntryNumber: 4, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c9080"},
			},
		}

		hash, _ := gitinterface.NewHash("e69de29bb2d1d6434b8b29ae775ad8c2e48d7914")
		p.InsertAttestationEntryNumber(uint64(5), hash)

		assert.Equal(t, []RSLEntryIndex{
			{EntryNumber: 1, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
			{EntryNumber: 2, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c1234"},
			{EntryNumber: 3, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c0980"},
			{EntryNumber: 4, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c9080"},
			{EntryNumber: 5, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48d7914"},
		}, p.AttestationEntries)
	})

	t.Run("inserting at insertion point", func(t *testing.T) {
		p := &Persistent{
			AttestationEntries: []RSLEntryIndex{
				{EntryNumber: 1, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
				{EntryNumber: 3, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c0980"},
				{EntryNumber: 4, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c9080"},
			},
		}

		hash, _ := gitinterface.NewHash("e69de29bb2d1d6434b8b29ae775ad8c2e48c1234")
		p.InsertAttestationEntryNumber(uint64(2), hash)

		assert.Equal(t, []RSLEntryIndex{
			{EntryNumber: 1, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
			{EntryNumber: 2, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c1234"},
			{EntryNumber: 3, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c0980"},
			{EntryNumber: 4, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c9080"},
		}, p.AttestationEntries)
	})
}

func TestSetAddedAttestationsBeforeNumber(t *testing.T) {
	p := Persistent{
		AddedAttestationsBeforeNumber: 1,
	}

	p.SetAddedAttestationsBeforeNumber(uint64(2))
	assert.Equal(t, uint64(2), p.AddedAttestationsBeforeNumber) // number bigger than the current value will be assigned

	p.SetAddedAttestationsBeforeNumber(uint64(1))
	assert.Equal(t, uint64(2), p.AddedAttestationsBeforeNumber) // number smaller than the current value will be ignored
}

func TestGetAttestationsEntries(t *testing.T) {
	p := &Persistent{
		AttestationEntries: []RSLEntryIndex{
			{EntryNumber: 1, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
			{EntryNumber: 3, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c0980"},
			{EntryNumber: 4, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c9080"},
		},
	}

	values := p.GetAttestationsEntries()
	assert.Equal(t, p.AttestationEntries, values)
}
