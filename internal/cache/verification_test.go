// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"testing"

	"github.com/gittuf/gittuf/internal/attestations"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestSetLastVerifiedEntryForRef(t *testing.T) {
	p := &Persistent{
		PolicyEntries: []RSLEntryIndex{
			{EntryNumber: 1, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
			{EntryNumber: 2, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c1234"},
		},
	}

	hash1, _ := gitinterface.NewHash("e69de29bb2d1d6434b8b29ae775ad8c2e48c1234")
	p.SetLastVerifiedEntryForRef(policyRef, uint64(2), hash1)

	value, has := p.LastVerifiedEntryForRef[policyRef]
	assert.True(t, has)
	assert.Equal(t, RSLEntryIndex{EntryNumber: 2, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c1234"}, value) // value set successfully

	hash2, _ := gitinterface.NewHash("e69de29bb2d1d6434b8b29ae775ad8c2e48c5391")
	p.SetLastVerifiedEntryForRef(policyRef, uint64(1), hash2)

	value = p.LastVerifiedEntryForRef[policyRef]
	assert.NotEqual(t, RSLEntryIndex{EntryNumber: 1, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"}, value) // since an entry withn a bigger entry number exist, entry number 1 doesn't get set
	assert.Equal(t, RSLEntryIndex{EntryNumber: 2, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c1234"}, value)
}

func TestGetLastVerifiedEntryForRef(t *testing.T) {
	t.Run("map doesn't exist", func(t *testing.T) {
		p := &Persistent{
			PolicyEntries: []RSLEntryIndex{
				{EntryNumber: 1, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
				{EntryNumber: 2, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c1234"},
			},
		}

		number, id := p.GetLastVerifiedEntryForRef(policyRef)
		assert.Equal(t, uint64(0), number)
		assert.Equal(t, gitinterface.ZeroHash, id) // if the map doesn't exist, should return 0 for entry number and zero hash for entry id
	})

	t.Run("entry for ref doesn't exist", func(t *testing.T) {
		p := &Persistent{
			PolicyEntries: []RSLEntryIndex{
				{EntryNumber: 1, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
				{EntryNumber: 2, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c1234"},
			},
		}

		hash, _ := gitinterface.NewHash("e69de29bb2d1d6434b8b29ae775ad8c2e48c1234")
		p.SetLastVerifiedEntryForRef(policyRef, uint64(2), hash)

		number, id := p.GetLastVerifiedEntryForRef(attestations.Ref)
		assert.Equal(t, uint64(0), number)
		assert.Equal(t, gitinterface.ZeroHash, id)
	})

	t.Run("entry for ref exists", func(t *testing.T) {
		p := &Persistent{
			PolicyEntries: []RSLEntryIndex{
				{EntryNumber: 1, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
				{EntryNumber: 2, EntryID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c1234"},
			},
		}

		hash, _ := gitinterface.NewHash("e69de29bb2d1d6434b8b29ae775ad8c2e48c5391")
		p.SetLastVerifiedEntryForRef(policyRef, uint64(2), hash)

		number, id := p.GetLastVerifiedEntryForRef(policyRef)
		assert.Equal(t, uint64(2), number)
		assert.Equal(t, hash, id)
	})
}
