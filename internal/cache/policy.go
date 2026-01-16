// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"fmt"
	"log/slog"
	"slices"

	"github.com/gittuf/gittuf/pkg/gitinterface"
)

func (p *Persistent) GetPolicyEntries() []RSLEntryIndex {
	return p.PolicyEntries
}

func (p *Persistent) HasPolicyEntryNumber(entryNumber uint64) (gitinterface.Hash, bool) {
	if len(p.PolicyEntries) == 0 || entryNumber == 0 {
		return gitinterface.ZeroHash, false
	}

	index, has := slices.BinarySearchFunc(p.PolicyEntries, RSLEntryIndex{EntryNumber: entryNumber}, binarySearch)
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
		return RSLEntryIndex{EntryNumber: 0} // this is a special case
	}

	slog.Debug(fmt.Sprintf("Finding policy entry in cache before entry %d...", entryNumber))

	index, has := slices.BinarySearchFunc(p.PolicyEntries, RSLEntryIndex{EntryNumber: entryNumber}, binarySearch)
	if has {
		// The entry number given to us is the first entry which happens
		// to be the start of verification as well
		// This can happen for full verification
		return p.PolicyEntries[index]
	}

	if !has && index == 0 {
		// this happens when a policy entry doesn't exist before the specified
		// entryNumber
		return RSLEntryIndex{EntryNumber: 0}
	}

	// When !has, index is point of insertion, but we want the applicable
	// entry which is index-1
	return p.PolicyEntries[index-1]
}

func (p *Persistent) FindPolicyEntriesInRange(firstNumber, lastNumber uint64) ([]RSLEntryIndex, error) {
	if len(p.PolicyEntries) == 0 {
		return nil, ErrNoPersistentCache // TODO: check if custom error makes sense
	}

	firstIndex, has := slices.BinarySearchFunc(p.PolicyEntries, RSLEntryIndex{EntryNumber: firstNumber}, binarySearch)
	if !has {
		// When !has, index is point of insertion, but we want the applicable
		// entry which is index-1
		firstIndex--
	}

	lastIndex, has := slices.BinarySearchFunc(p.PolicyEntries, RSLEntryIndex{EntryNumber: lastNumber}, binarySearch)
	if has {
		// When has, lastIndex is an entry we want to return, so we increment
		// lastIndex to ensure the corresponding entry is included in the return
		lastIndex++
	}

	return p.PolicyEntries[firstIndex:lastIndex], nil
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
		slog.Debug("No policy entries in cache, adding current entry as sole item...")
		p.PolicyEntries = []RSLEntryIndex{{EntryNumber: entryNumber, EntryID: entryID.String()}}
		return
	}

	if p.PolicyEntries[len(p.PolicyEntries)-1].GetEntryNumber() < entryNumber {
		// Current entry clearly belongs at the very end
		slog.Debug("Current entry belongs at the end of ordered list of attestations entry, appending...")
		p.PolicyEntries = append(p.PolicyEntries, RSLEntryIndex{EntryNumber: entryNumber, EntryID: entryID.String()})
		return
	}

	// We don't check the converse where the current entry is less than the
	// first entry because we're inserting as entries are encountered
	// chronologically. Worst case, binary search fallthrough below will still
	// handle it

	slog.Debug("Searching for insertion point...")
	index, has := slices.BinarySearchFunc(p.PolicyEntries, RSLEntryIndex{EntryNumber: entryNumber}, binarySearch)
	if has {
		// We could assume that if we've seen an entry with a number greater
		// than this, we should have seen this one too, but for now...
		slog.Debug("Entry with same number found, skipping addition of entry...")
		return
	}
	slog.Debug(fmt.Sprintf("Found insertion point %d", index))

	newSlice := make([]RSLEntryIndex, 0, len(p.PolicyEntries)+1)
	newSlice = append(newSlice, p.PolicyEntries[:index]...)
	newSlice = append(newSlice, RSLEntryIndex{EntryNumber: entryNumber, EntryID: entryID.String()})
	newSlice = append(newSlice, p.PolicyEntries[index:]...)

	p.PolicyEntries = newSlice
}
