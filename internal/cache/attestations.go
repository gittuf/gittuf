// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"fmt"
	"log/slog"
	"slices"

	"github.com/gittuf/gittuf/internal/gitinterface"
)

func (p *Persistent) FindAttestationsEntryNumberForEntry(entryNumber uint64) RSLEntryIndex {
	if len(p.AttestationEntries) == 0 {
		return RSLEntryIndex{entryNumber: 0} // special case
	}

	// Set entryNumber as max scanned if it's higher than what's already there
	if p.AddedAttestationsBeforeNumber < entryNumber {
		p.AddedAttestationsBeforeNumber = entryNumber
	}

	index, has := slices.BinarySearchFunc(p.AttestationEntries, RSLEntryIndex{entryNumber: entryNumber}, binarySearch)
	if has {
		return p.AttestationEntries[index]
	}

	return p.AttestationEntries[index-1]
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
		p.AttestationEntries = []RSLEntryIndex{{entryNumber: entryNumber, entryID: entryID}}
		return
	}

	if p.AttestationEntries[len(p.AttestationEntries)-1].GetEntryNumber() < entryNumber {
		// Current entry clearly belongs at the very end
		p.AttestationEntries = append(p.AttestationEntries, RSLEntryIndex{entryNumber: entryNumber, entryID: entryID})
		return
	}

	// We don't check the converse where the current entry is less than the
	// first entry because we're inserting as entries are encountered
	// chronologically. Worst case, binary search fallthrough below will still
	// handle it

	index, has := slices.BinarySearchFunc(p.AttestationEntries, RSLEntryIndex{entryNumber: entryNumber}, binarySearch)
	if has {
		// We could assume that if we've seen an entry with a number greater
		// than this, we should have seen this one too, but for now...
		return
	}

	newSlice := make([]RSLEntryIndex, 0, len(p.AttestationEntries)+1)
	for i := 0; i < index; i++ {
		newSlice[i] = p.AttestationEntries[i]
	}
	for i := index + 1; i < len(p.AttestationEntries)+1; i++ {
		newSlice[i] = p.AttestationEntries[i-1]
	}
	newSlice[index] = RSLEntryIndex{entryNumber: entryNumber, entryID: entryID}

	p.AttestationEntries = newSlice
}
