// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package cache

import "github.com/gittuf/gittuf/pkg/gitinterface"

func (p *Persistent) SetLastVerifiedEntryForRef(ref string, entryNumber uint64, entryID gitinterface.Hash) {
	if p.LastVerifiedEntryForRef == nil {
		p.LastVerifiedEntryForRef = map[string]RSLEntryIndex{}
	}

	currentIndex, hasEntryForRef := p.LastVerifiedEntryForRef[ref]
	if hasEntryForRef {
		// If set verified number is higher than entryNumber, noop
		if currentIndex.GetEntryNumber() > entryNumber {
			return
		}
	}

	p.LastVerifiedEntryForRef[ref] = RSLEntryIndex{EntryNumber: entryNumber, EntryID: entryID.String()}
}

func (p *Persistent) GetLastVerifiedEntryForRef(ref string) (uint64, gitinterface.Hash) {
	if p.LastVerifiedEntryForRef == nil {
		return 0, gitinterface.ZeroHash
	}

	currentIndex, hasEntryForRef := p.LastVerifiedEntryForRef[ref]
	if !hasEntryForRef {
		return 0, gitinterface.ZeroHash
	}

	return currentIndex.GetEntryNumber(), currentIndex.GetEntryID()
}
