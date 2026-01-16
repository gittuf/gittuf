// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"fmt"
	"log/slog"
	"slices"

	"github.com/gittuf/gittuf/pkg/gitinterface"
)

func (p *Persistent) GetAttestationsEntries() []RSLEntryIndex {
	return p.AttestationEntries
}

// FindAttestationsEntryNumberForEntry returns the index of the attestations
// entry to use. If the returned index has EntryNumber set to 0, it indicates
// that an applicable entry was not found in the cache.
func (p *Persistent) FindAttestationsEntryNumberForEntry(entryNumber uint64) (RSLEntryIndex, bool) {
	// Set entryNumber as max scanned if it's higher than what's already there
	p.SetAddedAttestationsBeforeNumber(entryNumber)

	index, has := slices.BinarySearchFunc(p.AttestationEntries, RSLEntryIndex{EntryNumber: entryNumber}, binarySearch)
	if has {
		slog.Debug(fmt.Sprintf("Requested entry number '%d' is for attestations", entryNumber))
		slog.Debug("Requested attestations entry found in cache!")
		return p.AttestationEntries[index], false
	}

	if !has && index == 0 {
		// this happens when an attestations entry doesn't exist before the
		// specified entryNumber. No need to use the fallthrough.
		slog.Debug("No applicable attestations entry found in cache")
		return RSLEntryIndex{EntryNumber: 0}, false
	}

	slog.Debug("Requested attestations entry found in cache!")
	return p.AttestationEntries[index-1], false
}

func (p *Persistent) InsertAttestationEntryNumber(entryNumber uint64, entryID gitinterface.Hash) {
	if entryNumber == 0 {
		// For now, we don't have a way to track non-numbered entries
		// We likely never want to track non-numbered entries in this
		// cache as this is very dependent on numbering
		return
	}

	// TODO: check this is for the right ref?

	slog.Debug(fmt.Sprintf("Inserting attestations entry with ID '%s' and number %d into persistent cache...", entryID.String(), entryNumber))

	if len(p.AttestationEntries) == 0 {
		// No entries yet, just add the current entry
		slog.Debug("No attestations entries in cache, adding current entry as sole item...")
		p.AttestationEntries = []RSLEntryIndex{{EntryNumber: entryNumber, EntryID: entryID.String()}}
		return
	}

	if p.AttestationEntries[len(p.AttestationEntries)-1].GetEntryNumber() < entryNumber {
		// Current entry clearly belongs at the very end
		slog.Debug("Current entry belongs at the end of ordered list of attestations entry, appending...")
		p.AttestationEntries = append(p.AttestationEntries, RSLEntryIndex{EntryNumber: entryNumber, EntryID: entryID.String()})
		return
	}

	// We don't check the converse where the current entry is less than the
	// first entry because we're inserting as entries are encountered
	// chronologically. Worst case, binary search fallthrough below will still
	// handle it

	slog.Debug("Searching for insertion point...")
	index, has := slices.BinarySearchFunc(p.AttestationEntries, RSLEntryIndex{EntryNumber: entryNumber}, binarySearch)
	if has {
		// We could assume that if we've seen an entry with a number greater
		// than this, we should have seen this one too, but for now...
		slog.Debug("Entry with same number found, skipping addition of entry...")
		return
	}
	slog.Debug(fmt.Sprintf("Found insertion point %d", index))

	newSlice := make([]RSLEntryIndex, 0, len(p.AttestationEntries)+1)
	newSlice = append(newSlice, p.AttestationEntries[:index]...)
	newSlice = append(newSlice, RSLEntryIndex{EntryNumber: entryNumber, EntryID: entryID.String()})
	newSlice = append(newSlice, p.AttestationEntries[index:]...)

	p.AttestationEntries = newSlice

	p.SetAddedAttestationsBeforeNumber(entryNumber)
}

func (p *Persistent) SetAddedAttestationsBeforeNumber(entryNumber uint64) {
	if p.AddedAttestationsBeforeNumber < entryNumber {
		p.AddedAttestationsBeforeNumber = entryNumber
	}
}
