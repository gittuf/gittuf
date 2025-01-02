// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"fmt"
	"log/slog"
	"slices"

	"github.com/gittuf/gittuf/internal/gitinterface"
)

func (p *Persistent) HasPolicyEntryNumber(entryNumber uint64) (gitinterface.Hash, bool) {
	if len(p.PolicyEntries) == 0 || entryNumber == 0 {
		return gitinterface.ZeroHash, false
	}

	index, has := slices.BinarySearchFunc(p.PolicyEntries, RSLEntryIndex{entryNumber: entryNumber}, binarySearch)
	if !has {
		return gitinterface.ZeroHash, false
	}

	// Unlike Find... we're actively checking if a policy number has been
	// inserted into the cache before, so we return the ID from that index
	// exactly
	return p.PolicyEntries[index].GetEntryID(), true
}

func (p *Persistent) FindPolicyEntryNumberForEntry(entryNumber uint64) RSLEntryIndex {
	if len(p.PolicyEntries) == 0 {
		return RSLEntryIndex{entryNumber: 0} // this is a special case
	}

	index, has := slices.BinarySearchFunc(p.PolicyEntries, RSLEntryIndex{entryNumber: entryNumber}, binarySearch)
	if has {
		// The entry number given to us is the first entry which happens
		// to be the start of verification as well
		// This can happen for full verification
		return p.PolicyEntries[index]
	}

	// When !has, index is point of insertion, but we want the applicable
	// entry which is index-1
	return p.PolicyEntries[index-1]
}

func (p *Persistent) InsertPolicyEntryNumber(entryNumber uint64, entryID gitinterface.Hash) {
	if entryNumber == 0 {
		// For now, we don't have a way to track non-numbered entries
		// We likely never want to track non-numbered entries in this
		// cache as this is very dependent on numbering
		return
	}

	// TODO: check this is for the right ref?

	slog.Debug(fmt.Sprintf("Inserting policy entry with ID '%s' and number %d into persistent cache...", entryID.String(), entryNumber))

	if len(p.PolicyEntries) == 0 {
		// No entries yet, just add the current entry
		p.PolicyEntries = []RSLEntryIndex{{entryNumber: entryNumber, entryID: entryID}}
		return
	}

	if p.PolicyEntries[len(p.PolicyEntries)-1].GetEntryNumber() < entryNumber {
		// Current entry clearly belongs at the very end
		p.PolicyEntries = append(p.PolicyEntries, RSLEntryIndex{entryNumber: entryNumber, entryID: entryID})
		return
	}

	// We don't check the converse where the current entry is less than the
	// first entry because we're inserting as entries are encountered
	// chronologically. Worst case, binary search fallthrough below will still
	// handle it

	index, has := slices.BinarySearchFunc(p.PolicyEntries, RSLEntryIndex{entryNumber: entryNumber}, binarySearch)
	if has {
		// We could assume that if we've seen an entry with a number greater
		// than this, we should have seen this one too, but for now...
		return
	}

	newSlice := make([]RSLEntryIndex, 0, len(p.PolicyEntries)+1)
	for i := 0; i < index; i++ {
		newSlice[i] = p.PolicyEntries[i]
	}
	for i := index + 1; i < len(p.PolicyEntries)+1; i++ {
		newSlice[i] = p.PolicyEntries[i-1]
	}
	newSlice[index] = RSLEntryIndex{entryNumber: entryNumber, entryID: entryID}

	p.PolicyEntries = newSlice
}
