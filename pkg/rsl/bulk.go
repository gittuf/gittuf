// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

// This file holds the bulk reference entry: its type, codec, validation,
// parser, and the folding helpers that expand it into per-ref views.

package rsl

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/gittuf/gittuf/pkg/githash"
	"github.com/gittuf/gittuf/pkg/gitstore"
)

const BulkReferenceEntryHeader = "RSL Bulk Reference Entry"

var (
	ErrDuplicateReferenceInBulkEntry = errors.New("bulk reference entry lists the same reference more than once")
	ErrGittufReferenceInBulkEntry    = errors.New("bulk reference entry cannot record references in the gittuf namespace")
	ErrCannotCommitView              = errors.New("cannot commit a per-ref view of a bulk reference entry, commit the bulk entry instead")
)

// ReferenceUpdate is one reference update inside a BulkReferenceEntry.
type ReferenceUpdate struct {
	RefName  string
	TargetID githash.Hash
}

// BulkReferenceEntry records one or more reference updates under a single
// signature. It implements Entry. It does not implement ReferenceUpdaterEntry
// because it has no single ref. Walkers expand it into per-ref views with
// ReferenceEntries. Refs are unique within an entry and never in the gittuf
// namespace.
//
// On the wire an entry is its header, a blank line, one "<ref>: <targetID>"
// line per update in listed order, a blank line, and a mandatory number line:
//
//	RSL Bulk Reference Entry
//
//	refs/heads/main: 9e37b2f8b9d5b0e1cc2f2c2a6a4b1f9c7b5e0d31
//	refs/heads/feature: 4a2f1b9c7e5d3a8b6c4f2e1d0b9a8c7f6e5d4c3b
//
//	number: 5
type BulkReferenceEntry struct {
	// ID contains the Git hash for the commit corresponding to the entry.
	ID githash.Hash

	// Updates lists the reference updates in the order they were recorded.
	Updates []ReferenceUpdate

	// Number contains a strictly increasing number that hints at entry ordering.
	Number uint64
}

var _ Entry = (*BulkReferenceEntry)(nil)

// NewBulkReferenceEntry returns a BulkReferenceEntry for the specified
// updates. Validation happens when the entry is committed.
func NewBulkReferenceEntry(updates []ReferenceUpdate) *BulkReferenceEntry {
	return &BulkReferenceEntry{Updates: updates}
}

func (e *BulkReferenceEntry) GetID() githash.Hash {
	return e.ID
}

func (e *BulkReferenceEntry) GetNumber() uint64 {
	return e.Number
}

// ReferenceEntries returns one read-only view per update. Each view carries
// the bulk entry's ID and Number and reports FromBulkEntry as true. Views
// cannot be committed.
func (e *BulkReferenceEntry) ReferenceEntries() []*ReferenceEntry {
	views := make([]*ReferenceEntry, 0, len(e.Updates))
	for _, update := range e.Updates {
		views = append(views, e.referenceEntryForUpdate(update))
	}
	return views
}

func (e *BulkReferenceEntry) referenceEntryForUpdate(update ReferenceUpdate) *ReferenceEntry {
	return &ReferenceEntry{
		ID:       e.ID,
		RefName:  update.RefName,
		TargetID: update.TargetID,
		Number:   e.Number,
		isView:   true,
	}
}

// Commit creates a commit object in the RSL for the BulkReferenceEntry. The
// number is assigned the same way as for the other entry types.
func (e *BulkReferenceEntry) Commit(storer gitstore.Storer, sign bool) error {
	expectedTip, err := e.setEntryNumber(storer)
	if err != nil {
		return err
	}

	message, err := e.createCommitMessage(true)
	if err != nil {
		return err
	}

	return commitEntry(storer, message, sign, expectedTip)
}

// CommitUsingSpecificKey creates a commit object in the RSL for the
// BulkReferenceEntry, signed with the provided PEM encoded key. Intended for
// developer mode and tests.
func (e *BulkReferenceEntry) CommitUsingSpecificKey(storer gitstore.Storer, signingKeyBytes []byte) error {
	expectedTip, err := e.setEntryNumber(storer)
	if err != nil {
		return err
	}

	message, err := e.createCommitMessage(true)
	if err != nil {
		return err
	}

	return commitEntryUsingSpecificKey(storer, message, signingKeyBytes, expectedTip)
}

func (e *BulkReferenceEntry) setEntryNumber(storer gitstore.Storer) (githash.Hash, error) {
	number, tip, err := nextEntryNumber(storer)
	if err != nil {
		return tip, err
	}

	e.Number = number
	return tip, nil
}

// createCommitMessage renders the bulk entry's wire format. The number is
// mandatory for a bulk entry, so the includeNumber parameter that the Entry
// interface requires is ignored here. The older entry types honour it because
// they must still be able to render the numberless form they shipped with.
// Commit and CommitUsingSpecificKey assign the number before they render, so
// a zero number can only come from misuse and is an error.
func (e *BulkReferenceEntry) createCommitMessage(bool) (string, error) {
	if err := validateReferenceUpdates(e.Updates); err != nil {
		return "", err
	}
	if e.Number == 0 {
		return "", fmt.Errorf("%w: bulk reference entry has no number", ErrInvalidRSLEntry)
	}

	var message strings.Builder
	message.WriteString(BulkReferenceEntryHeader)
	message.WriteString("\n\n")
	for _, update := range e.Updates {
		fmt.Fprintf(&message, "%s: %s\n", update.RefName, update.TargetID.String())
	}
	fmt.Fprintf(&message, "\n%s: %d", NumberKey, e.Number)
	return message.String(), nil
}

// validateReferenceUpdates enforces the bulk entry invariants shared by the
// writer and the parser: at least one update, fully qualified and trimmed
// ref names, no gittuf namespace refs, and no duplicate refs.
func validateReferenceUpdates(updates []ReferenceUpdate) error {
	if len(updates) == 0 {
		return fmt.Errorf("%w: bulk reference entry has no updates", ErrInvalidRSLEntry)
	}

	seen := make(map[string]struct{}, len(updates))
	for _, update := range updates {
		if err := validateBulkRefName(update.RefName); err != nil {
			return err
		}
		if _, duplicate := seen[update.RefName]; duplicate {
			return fmt.Errorf("%w: %s: %w", ErrDuplicateReferenceInBulkEntry, update.RefName, ErrInvalidRSLEntry)
		}
		seen[update.RefName] = struct{}{}
	}

	return nil
}

func validateBulkRefName(refName string) error {
	// The single line and whitespace checks are here for the writer's sake.
	// The parser trims every field value before it validates it, so only a
	// value built in process can still carry a line break or padding.
	if err := checkSingleLineField(refName); err != nil {
		return err
	}
	if refName == "" {
		return fmt.Errorf("%w: reference name is empty", ErrInvalidRSLEntry)
	}
	if refName != strings.TrimSpace(refName) {
		return fmt.Errorf("%w: reference name %q has surrounding whitespace", ErrInvalidRSLEntry, refName)
	}
	if !strings.HasPrefix(refName, refPrefix) {
		return fmt.Errorf("%w: reference name %q is not fully qualified", ErrInvalidRSLEntry, refName)
	}
	if strings.HasPrefix(refName, gittufNamespacePrefix) {
		return fmt.Errorf("%w: %s: %w", ErrGittufReferenceInBulkEntry, refName, ErrInvalidRSLEntry)
	}
	return nil
}

// parseBulkReferenceEntryText parses a bulk reference entry. After the header
// and its mandatory blank line the body is one or more update lines, exactly
// one blank line, a number line, and nothing but blank lines after that. An
// update line is split on its first colon. The key is the reference name and
// the value is the target ID. Git reference names cannot contain a colon, so
// the split is unambiguous. Anything else is rejected: there are no unknown
// keys to skip and no optional fields.
//
// The shape is chosen so that older clients fail closed. gittuf v0.9.0 through
// v0.16.0 reject an unrecognised header outright. gittuf up to v0.8.1 instead
// routes every non-annotation header into its lenient reference entry parser,
// which ignores keys it does not know but returns ErrInvalidRSLEntry for any
// body line without a colon. The blank line before the number is therefore
// load bearing: without it, those clients would skip every update line as an
// unknown key and silently accept an entry with an empty ref and a zero
// target. Making the number mandatory is what guarantees the blank line is
// always present.
func parseBulkReferenceEntryText(id githash.Hash, text string) (*BulkReferenceEntry, error) {
	body, err := entryBody(text, BulkReferenceEntryHeader)
	if err != nil {
		return nil, err
	}

	const (
		expectUpdate = iota // update lines, then the blank separator
		expectNumber        // separator seen, the number line must follow
		done                // number seen, only blank lines may follow
	)

	entry := &BulkReferenceEntry{ID: id, Updates: make([]ReferenceUpdate, 0, len(body))}
	state := expectUpdate

	for _, rawLine := range body {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			switch state {
			case expectUpdate:
				if len(entry.Updates) == 0 {
					return nil, fmt.Errorf("%w: bulk reference entry has no updates", ErrInvalidRSLEntry)
				}
				state = expectNumber
			case done:
				// Trailing blank lines after the number are harmless.
			default:
				return nil, fmt.Errorf("%w: unexpected blank line in bulk reference entry", ErrInvalidRSLEntry)
			}
			continue
		}

		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("%w: bulk reference entry line has no colon", ErrInvalidRSLEntry)
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)

		switch state {
		case expectUpdate:
			if err := validateBulkRefName(key); err != nil {
				return nil, err
			}
			update := ReferenceUpdate{RefName: key}
			if err := setHash(&update.TargetID, value); err != nil {
				return nil, err
			}
			entry.Updates = append(entry.Updates, update)

		case expectNumber:
			if key != NumberKey {
				return nil, fmt.Errorf("%w: expected %s after the blank line in bulk reference entry", ErrInvalidRSLEntry, NumberKey)
			}
			if err := setNumber(&entry.Number, value); err != nil {
				return nil, err
			}
			state = done

		default:
			return nil, fmt.Errorf("%w: bulk reference entry has a line after its number", ErrInvalidRSLEntry)
		}
	}

	if state != done {
		return nil, fmt.Errorf("%w: bulk reference entry has no number", ErrInvalidRSLEntry)
	}
	if err := validateReferenceUpdates(entry.Updates); err != nil {
		return nil, err
	}
	return entry, nil
}

// referenceUpdaters is the single point where the RSL walkers fold an entry
// into the reference updaters it carries. Reference and propagation entries
// return themselves. Bulk reference entries return their per-ref views in
// listed order. Annotations carry no updates and return nil. Any other type
// is an error so that a future entry type can never be dropped silently by a
// walker before verification sees it. The returned slice is freshly
// allocated, so callers may reorder or otherwise mutate it.
func referenceUpdaters(entry Entry) ([]ReferenceUpdaterEntry, error) {
	if entry == nil {
		return nil, fmt.Errorf("%w: nil entry", ErrUnknownRSLEntryType)
	}

	switch e := entry.(type) {
	case *ReferenceEntry:
		return []ReferenceUpdaterEntry{e}, nil
	case *PropagationEntry:
		return []ReferenceUpdaterEntry{e}, nil
	case *BulkReferenceEntry:
		updaters := make([]ReferenceUpdaterEntry, 0, len(e.Updates))
		for _, update := range e.Updates {
			updaters = append(updaters, e.referenceEntryForUpdate(update))
		}
		return updaters, nil
	case *AnnotationEntry:
		return nil, nil
	default:
		return nil, fmt.Errorf("%w: entry %s has unhandled type %T", ErrUnknownRSLEntryType, entry.GetID(), entry)
	}
}

// referenceUpdatersNewestFirst is referenceUpdaters in reverse listed order.
// Walkers that iterate the RSL from the tip backwards use it so that the last
// listed update in a bulk entry is encountered first. Two idioms follow from
// that. Iterating newest first and breaking on the first match yields the
// latest matching update in the entry. Iterating listed order with
// referenceUpdaters and breaking on the first match yields the first.
func referenceUpdatersNewestFirst(entry Entry) ([]ReferenceUpdaterEntry, error) {
	updaters, err := referenceUpdaters(entry)
	if err != nil {
		return nil, err
	}

	slices.Reverse(updaters)
	return updaters, nil
}

// GetReferenceUpdaterEntryForRef loads the entry with the specified ID and
// returns the reference updater in it for refName. For a bulk reference
// entry this is the view for that ref. For a single-ref entry the ref must
// match. It is the loader to use wherever an (entry ID, ref) pair has been
// persisted, such as the persistent cache, because an entry ID alone no
// longer identifies one reference update.
func GetReferenceUpdaterEntryForRef(storer gitstore.Storer, entryID githash.Hash, refName string) (ReferenceUpdaterEntry, error) {
	entry, err := GetEntry(storer, entryID)
	if err != nil {
		return nil, err
	}

	updaters, err := referenceUpdaters(entry)
	if err != nil {
		return nil, err
	}

	for _, updater := range updaters {
		if updater.GetRefName() == refName {
			return updater, nil
		}
	}

	return nil, fmt.Errorf("%w: entry %s does not update %s", ErrRSLEntryDoesNotMatchRef, entryID.String(), refName)
}
