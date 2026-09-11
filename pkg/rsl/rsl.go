// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

// Package rsl implements the gittuf Reference State Log (RSL) entry model:
// entry types, their commit-message codec, the parsing state machines, and the
// readers that walk the log. It operates entirely over the gitstore.Storer
// interface, so it carries no dependency on gitinterface (and thus none of
// gittuf's signing/attestation stack). A gitinterface-backed Repository
// satisfies the interface directly.
package rsl

import (
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"

	"github.com/gittuf/gittuf/pkg/customfields"
	"github.com/gittuf/gittuf/pkg/githash"
	"github.com/gittuf/gittuf/pkg/gitstore"
)

const (
	Ref       = "refs/gittuf/reference-state-log"
	NumberKey = "number"

	ReferenceEntryHeader = "RSL Reference Entry"
	RefKey               = "ref"
	TargetIDKey          = "targetID"

	AnnotationEntryHeader      = "RSL Annotation Entry"
	AnnotationMessageBlockType = "MESSAGE"
	BeginMessage               = "-----BEGIN MESSAGE-----"
	EndMessage                 = "-----END MESSAGE-----"
	EntryIDKey                 = "entryID"
	SkipKey                    = "skip"

	PropagationEntryHeader = "RSL Propagation Entry"
	UpstreamRepositoryKey  = "upstreamRepository"
	UpstreamEntryIDKey     = "upstreamEntryID"

	remoteTrackerRef       = "refs/remotes/%s/gittuf/reference-state-log"
	refPrefix              = "refs/"
	gittufNamespacePrefix  = "refs/gittuf/"
	gittufPolicyStagingRef = "refs/gittuf/policy-staging"
)

var (
	ErrRSLEntryNotFound                             = errors.New("unable to find RSL entry")
	ErrRSLBranchDetected                            = errors.New("potential RSL branch detected, entry has more than one parent")
	ErrInvalidRSLEntry                              = errors.New("RSL entry has invalid format or is of unexpected type")
	ErrRSLEntryDoesNotMatchRef                      = errors.New("RSL entry does not match requested ref")
	ErrNoRecordOfCommit                             = errors.New("commit has not been encountered before")
	ErrInvalidGetLatestReferenceUpdaterEntryOptions = errors.New("invalid options presented for getting latest reference updater entry (are both before or until conditions set or is the before number less than the until number?)")
	ErrCannotUseEntryNumberFilter                   = errors.New("current RSL entries are not numbered, cannot use number range options")
	ErrInvalidUntilEntryNumberCondition             = errors.New("cannot meet until entry number condition")
	ErrUnknownRSLEntryType                          = fmt.Errorf("%w: RSL entry is of an unknown type", ErrInvalidRSLEntry)
	// ErrInvalidAnnotationQualifier is returned only when committing an
	// annotation, as the parser cannot validate qualifiers without a storer.
	ErrInvalidAnnotationQualifier = errors.New("annotation ref qualifier does not match a reference update in the target bulk entry")
)

// nextEntryNumber returns the number to assign to a new entry: one more than
// the latest entry's number, or 1 when the RSL is empty. The numbering starts
// from 1 as 0 is used to signal the lack of numbering. It also returns the RSL
// tip it observed, which is the zero hash when the RSL is empty. Callers pass
// the tip to the commit helpers so the write is rejected if another writer
// advanced the RSL in the meantime.
func nextEntryNumber(storer gitstore.Storer) (uint64, githash.Hash, error) {
	latestEntry, err := GetLatestEntry(storer)
	if err != nil {
		if errors.Is(err, ErrRSLEntryNotFound) {
			return 1, storer.ZeroHash(), nil
		}
		return 0, storer.ZeroHash(), err
	}

	return latestEntry.GetNumber() + 1, latestEntry.GetID(), nil
}

// currentTip returns the RSL's tip, or the zero hash when the RSL does not
// exist yet.
func currentTip(storer gitstore.Storer) (githash.Hash, error) {
	tip, err := storer.GetReference(Ref)
	if err != nil {
		if errors.Is(err, ErrReferenceNotFound) {
			return storer.ZeroHash(), nil
		}
		return storer.ZeroHash(), err
	}

	return tip, nil
}

// commitEntry commits an RSL entry: an empty-tree commit on Ref carrying the
// entry in its message. This is the storage shape of every RSL entry.
func commitEntry(storer gitstore.Storer, message string, sign bool, expectedTip githash.Hash) error {
	emptyTreeID, err := storer.EmptyTree()
	if err != nil {
		return err
	}

	if pinning, ok := storer.(gitstore.TipPinningStorer); ok {
		_, err = pinning.CommitWithExpectedTip(emptyTreeID, Ref, message, sign, expectedTip)
		return err
	}

	slog.Warn("Storer does not support tip-pinned commits, an RSL write may record a stale entry number under concurrent writers")
	_, err = storer.Commit(emptyTreeID, Ref, message, sign)
	return err
}

// commitEntryUsingSpecificKey is commitEntry signing with the provided PEM
// encoded key. It is intended for gittuf's developer mode and tests.
func commitEntryUsingSpecificKey(storer gitstore.Storer, message string, signingKeyBytes []byte, expectedTip githash.Hash) error {
	emptyTreeID, err := storer.EmptyTree()
	if err != nil {
		return err
	}

	if pinning, ok := storer.(gitstore.TipPinningStorer); ok {
		_, err = pinning.CommitUsingSpecificKeyWithExpectedTip(emptyTreeID, Ref, message, signingKeyBytes, expectedTip)
		return err
	}

	slog.Warn("Storer does not support tip-pinned commits, an RSL write may record a stale entry number under concurrent writers")
	_, err = storer.CommitUsingSpecificKey(emptyTreeID, Ref, message, signingKeyBytes)
	return err
}

// RemoteTrackerRef returns the remote tracking ref for the specified remote's
// name. For example, for 'origin', the remote tracker ref is
// 'refs/remotes/origin/gittuf/reference-state-log'.
func RemoteTrackerRef(remote string) string {
	return fmt.Sprintf(remoteTrackerRef, remote)
}

// Entry is the abstract representation of an object in the RSL.
type Entry interface {
	GetID() githash.Hash
	Commit(gitstore.Storer, bool) error
	GetNumber() uint64
	createCommitMessage(bool) (string, error)
}

// ReferenceUpdaterEntry represents RSL entry types that can record an update to
// a Git reference. Some examples are the reference entry and the propagation
// entry.
type ReferenceUpdaterEntry interface {
	Entry
	GetRefName() string
	GetTargetID() githash.Hash
}

// ReferenceEntry represents a record of a reference state in the RSL. It
// implements the Entry interface.
//
// A *ReferenceEntry may also be a read-only per-ref view of a
// BulkReferenceEntry. A view carries the bulk entry's ID and Number, reports
// FromBulkEntry as true, and is refused with ErrCannotCommitView by Commit and
// CommitUsingSpecificKey. An entry ID therefore no longer identifies a single
// reference update: several views can share one ID. Code that persists an
// (entry ID, ref) pair must load it back with GetReferenceUpdaterEntryForRef.
type ReferenceEntry struct {
	// ID contains the Git hash for the commit corresponding to the entry.
	ID githash.Hash

	// RefName contains the Git reference the entry is for.
	RefName string

	// TargetID contains the Git hash for the object expected at RefName.
	TargetID githash.Hash

	// Number contains a strictly increasing number that hints at entry ordering.
	Number uint64

	// CustomFields contains application-defined metadata for the entry.
	CustomFields CustomFields

	// isView is true when this value is a per-ref projection of a
	// BulkReferenceEntry. Views share the bulk entry's ID and Number and
	// cannot be committed.
	isView bool
}

// FromBulkEntry reports whether this entry is a per-ref view of a
// BulkReferenceEntry. The ID of a view is the bulk entry's commit ID.
func (e *ReferenceEntry) FromBulkEntry() bool {
	return e.isView
}

// NewReferenceEntry returns a ReferenceEntry object for a normal RSL entry.
func NewReferenceEntry(refName string, targetID githash.Hash, opts ...EntryOption) *ReferenceEntry {
	options := applyEntryOptions(opts)
	return &ReferenceEntry{RefName: refName, TargetID: targetID, CustomFields: options.customFields}
}

func (e *ReferenceEntry) GetID() githash.Hash {
	return e.ID
}

func (e *ReferenceEntry) GetRefName() string {
	return e.RefName
}

func (e *ReferenceEntry) GetTargetID() githash.Hash {
	return e.TargetID
}

// Commit creates a commit object in the RSL for the ReferenceEntry. The
// function looks up the latest committed entry in the RSL and increments the
// number in the new entry. If a parent entry does not exist or the parent
// entry's number is 0 (unset), the current entry's number is set to 1. The
// numbering starts from 1 as 0 is used to signal the lack of numbering.
func (e *ReferenceEntry) Commit(storer gitstore.Storer, sign bool) error {
	if e.isView {
		return ErrCannotCommitView
	}

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
// ReferenceEntry. The commit is signed using the provided PEM encoded SSH or
// GPG private key. This is only intended for use in gittuf's developer mode or
// in tests. The function looks up the latest committed entry in the RSL and
// increments the number in the new entry. If a parent entry does not exist or
// the parent entry's number is 0 (unset), the current entry's number is set to
// 1. The numbering starts from 1 as 0 is used to signal the lack of numbering.
func (e *ReferenceEntry) CommitUsingSpecificKey(storer gitstore.Storer, signingKeyBytes []byte) error {
	if e.isView {
		return ErrCannotCommitView
	}

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

func (e *ReferenceEntry) GetNumber() uint64 {
	return e.Number
}

func (e *ReferenceEntry) GetCustomField(key string) (string, bool) {
	value, has := e.CustomFields[key]
	return value, has
}

// Skipped returns true if any of the annotations mark the entry as
// to-be-skipped.
func (e *ReferenceEntry) SkippedBy(annotations []*AnnotationEntry) bool {
	for _, annotation := range annotations {
		if annotation.Skip && annotation.AppliesTo(e.ID, e.RefName) {
			return true
		}
	}

	return false
}

func (e *ReferenceEntry) setEntryNumber(storer gitstore.Storer) (githash.Hash, error) {
	number, tip, err := nextEntryNumber(storer)
	if err != nil {
		return tip, err
	}

	e.Number = number
	return tip, nil
}

func checkSingleLineField(value string) error {
	if strings.ContainsAny(value, "\n\r") {
		return fmt.Errorf("%w: field value contains a line break", ErrInvalidRSLEntry)
	}
	return nil
}

func (e *ReferenceEntry) createCommitMessage(includeNumber bool) (string, error) {
	if err := checkSingleLineField(e.RefName); err != nil {
		return "", err
	}
	lines := []string{
		ReferenceEntryHeader,
		"",
		fmt.Sprintf("%s: %s", RefKey, e.RefName),
		fmt.Sprintf("%s: %s", TargetIDKey, e.TargetID.String()),
	}
	if includeNumber && e.Number > 0 {
		lines = append(lines, fmt.Sprintf("%s: %d", NumberKey, e.Number))
	}
	lines, err := appendCustomFieldLines(lines, e.CustomFields)
	if err != nil {
		return "", err
	}
	return strings.Join(lines, "\n"), nil
}

// CommitWithoutNumber records the entry without assigning it a number,
// producing a legacy unnumbered entry. It exists to exercise the RSL's support
// for repositories that transition from unnumbered to numbered entries.
func (e *ReferenceEntry) CommitWithoutNumber(storer gitstore.Storer) error {
	if e.isView {
		return ErrCannotCommitView
	}

	expectedTip, err := currentTip(storer)
	if err != nil {
		return err
	}

	message, err := e.createCommitMessage(false)
	if err != nil {
		return err
	}

	return commitEntry(storer, message, false, expectedTip)
}

// AnnotationEntry is a type of RSL record that references prior items in the
// RSL. It can be used to add extra information for the referenced items.
// Annotations can also be used to "skip", i.e. revoke, the referenced items. It
// implements the Entry interface.
type AnnotationEntry struct {
	// ID contains the Git hash for the commit corresponding to the annotation.
	ID githash.Hash

	// RSLEntryIDs contains one or more Git hashes for the RSL entries the annotation applies to.
	RSLEntryIDs []githash.Hash

	// Refs holds optional ref qualifiers keyed by the hex string of an entry
	// in RSLEntryIDs. An entry ID with no key, or an empty list, is referred
	// to as a whole. An entry ID with qualifiers is referred to only for the
	// listed reference updates. Qualifiers are only valid against bulk
	// reference entries. Nil when the annotation has no qualifiers.
	// Qualifiers are not validated against the target entry at parse time.
	// That happens in validateTargets at commit time, so a qualifier naming
	// a ref the target does not update applies to nothing.
	Refs map[string][]string

	// Skip indicates if the RSLEntryIDs must be skipped during gittuf workflows.
	Skip bool

	// Message contains any messages or notes added by a user for the annotation.
	Message string

	// Number contains a strictly increasing number that hints at entry ordering.
	Number uint64

	// CustomFields contains application-defined metadata for the entry.
	CustomFields CustomFields
}

// NewAnnotationEntry returns an Annotation object that applies to one or more
// prior RSL entries.
func NewAnnotationEntry(rslEntryIDs []githash.Hash, skip bool, message string, opts ...EntryOption) *AnnotationEntry {
	options := applyEntryOptions(opts)
	return &AnnotationEntry{RSLEntryIDs: rslEntryIDs, Skip: skip, Message: message, CustomFields: options.customFields}
}

// NewAnnotationEntryWithQualifiers returns an annotation that applies to the
// listed entries, restricted for the keyed entries to the listed reference
// updates. Keys are entry ID hex strings and must also appear in rslEntryIDs.
func NewAnnotationEntryWithQualifiers(rslEntryIDs []githash.Hash, refs map[string][]string, skip bool, message string, opts ...EntryOption) *AnnotationEntry {
	// The map is cloned so the annotation does not alias the caller's, and
	// keys with no refs are dropped: they mean the whole entry, which is how
	// an absent key already reads. A nil map keeps codec round trips equal.
	var qualifiers map[string][]string
	for id, refNames := range refs {
		if len(refNames) == 0 {
			continue
		}
		if qualifiers == nil {
			qualifiers = make(map[string][]string, len(refs))
		}
		qualifiers[id] = slices.Clone(refNames)
	}

	options := applyEntryOptions(opts)
	return &AnnotationEntry{RSLEntryIDs: rslEntryIDs, Refs: qualifiers, Skip: skip, Message: message, CustomFields: options.customFields}
}

// AppliesTo reports whether the annotation applies to the reference update
// for refName in entryID. An unqualified mention applies to every update in
// the entry. A qualified mention applies only to the listed refs.
func (a *AnnotationEntry) AppliesTo(entryID githash.Hash, refName string) bool {
	if !a.RefersTo(entryID) {
		return false
	}
	if len(a.Refs) == 0 {
		return true
	}
	refs, qualified := a.Refs[entryID.String()]
	if !qualified || len(refs) == 0 {
		return true
	}
	return slices.Contains(refs, refName)
}

func (a *AnnotationEntry) GetID() githash.Hash {
	return a.ID
}

// Commit creates a commit object in the RSL for the Annotation. The function
// looks up the latest committed entry in the RSL and increments the number in
// the new entry. If a parent entry does not exist or the parent entry's number
// is 0 (unset), the current entry's number is set to 1. The numbering starts
// from 1 as 0 is used to signal the lack of numbering.
func (a *AnnotationEntry) Commit(storer gitstore.Storer, sign bool) error {
	if err := a.validateTargets(storer); err != nil {
		return err
	}

	expectedTip, err := a.setEntryNumber(storer)
	if err != nil {
		return err
	}

	message, err := a.createCommitMessage(true)
	if err != nil {
		return err
	}

	return commitEntry(storer, message, sign, expectedTip)
}

// CommitUsingSpecificKey creates a commit object in the RSL for the
// AnnotationEntry. The commit is signed using the provided PEM encoded SSH or
// GPG private key. This is only intended for use in gittuf's developer mode or
// in tests. The function looks up the latest committed entry in the RSL and
// increments the number in the new entry. If a parent entry does not exist or
// the parent entry's number is 0 (unset), the current entry's number is set to
// 1. The numbering starts from 1 as 0 is used to signal the lack of numbering.
func (a *AnnotationEntry) CommitUsingSpecificKey(storer gitstore.Storer, signingKeyBytes []byte) error {
	if err := a.validateTargets(storer); err != nil {
		return err
	}

	expectedTip, err := a.setEntryNumber(storer)
	if err != nil {
		return err
	}

	message, err := a.createCommitMessage(true)
	if err != nil {
		return err
	}

	return commitEntryUsingSpecificKey(storer, message, signingKeyBytes, expectedTip)
}

func (a *AnnotationEntry) GetNumber() uint64 {
	return a.Number
}

func (a *AnnotationEntry) GetCustomField(key string) (string, bool) {
	value, has := a.CustomFields[key]
	return value, has
}

// validateTargets checks that every referenced entry exists and that every
// ref qualifier names a reference update in a bulk reference entry.
func (a *AnnotationEntry) validateTargets(storer gitstore.Storer) error {
	knownIDs := make(map[string]struct{}, len(a.RSLEntryIDs))
	for _, id := range a.RSLEntryIDs {
		key := id.String()
		// An entry ID listed twice without qualifiers has always been
		// legal and means the same as listing it once. With qualifiers it
		// cannot round trip: createCommitMessage emits the qualifiers once
		// per occurrence and the parser attaches them to the most recent
		// entry ID, so the list would grow on every read and write cycle.
		if _, duplicate := knownIDs[key]; duplicate && len(a.Refs[key]) != 0 {
			return fmt.Errorf("%w: entry %s is listed more than once with ref qualifiers", ErrInvalidAnnotationQualifier, key)
		}
		knownIDs[key] = struct{}{}
	}

	// Keys are sorted so the reported key does not depend on map order.
	qualifiedIDs := make([]string, 0, len(a.Refs))
	for id := range a.Refs {
		qualifiedIDs = append(qualifiedIDs, id)
	}
	slices.Sort(qualifiedIDs)

	for _, id := range qualifiedIDs {
		if _, known := knownIDs[id]; !known {
			return fmt.Errorf("%w: entry %s is not referred to by the annotation", ErrInvalidAnnotationQualifier, id)
		}
	}

	for _, id := range a.RSLEntryIDs {
		target, err := GetEntry(storer, id)
		if err != nil {
			return err
		}

		refs := a.Refs[id.String()]
		if len(refs) == 0 {
			continue
		}

		bulk, isBulk := target.(*BulkReferenceEntry)
		if !isBulk {
			return fmt.Errorf("%w: entry %s is not a bulk reference entry", ErrInvalidAnnotationQualifier, id.String())
		}
		for _, refName := range refs {
			found := false
			for _, update := range bulk.Updates {
				if update.RefName == refName {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("%w: entry %s does not update %s", ErrInvalidAnnotationQualifier, id.String(), refName)
			}
		}
	}
	return nil
}

// RefersTo returns true if the specified entryID is referred to by the
// annotation.
func (a *AnnotationEntry) RefersTo(entryID githash.Hash) bool {
	for _, id := range a.RSLEntryIDs {
		if id.Equal(entryID.Bytes()) {
			return true
		}
	}

	return false
}

// TODO: can an annotation actually be the first entry in the RSL? If it cannot,
// the number 1 that nextEntryNumber assigns for an empty RSL is unreachable
// here.
func (a *AnnotationEntry) setEntryNumber(storer gitstore.Storer) (githash.Hash, error) {
	number, tip, err := nextEntryNumber(storer)
	if err != nil {
		return tip, err
	}

	a.Number = number
	return tip, nil
}

func (a *AnnotationEntry) createCommitMessage(includeNumber bool) (string, error) {
	lines := []string{
		AnnotationEntryHeader,
		"",
	}

	for _, entry := range a.RSLEntryIDs {
		lines = append(lines, fmt.Sprintf("%s: %s", EntryIDKey, entry.String()))
		for _, refName := range a.Refs[entry.String()] {
			if err := validateBulkRefName(refName); err != nil {
				return "", err
			}
			lines = append(lines, fmt.Sprintf("%s: %s", RefKey, refName))
		}
	}

	if a.Skip {
		lines = append(lines, fmt.Sprintf("%s: true", SkipKey))
	} else {
		lines = append(lines, fmt.Sprintf("%s: false", SkipKey))
	}

	if includeNumber && a.Number > 0 {
		lines = append(lines, fmt.Sprintf("%s: %d", NumberKey, a.Number))
	}

	lines, err := appendCustomFieldLines(lines, a.CustomFields)
	if err != nil {
		return "", err
	}

	if len(a.Message) != 0 {
		var message strings.Builder
		messageBlock := pem.Block{
			Type:  AnnotationMessageBlockType,
			Bytes: []byte(a.Message),
		}
		if err := pem.Encode(&message, &messageBlock); err != nil {
			return "", err
		}
		lines = append(lines, strings.TrimSpace(message.String()))
	}

	return strings.Join(lines, "\n"), nil
}

// CommitWithoutNumber records the annotation without assigning it a number,
// producing a legacy unnumbered entry. It exists to exercise the RSL's support
// for repositories that transition from unnumbered to numbered entries.
func (a *AnnotationEntry) CommitWithoutNumber(storer gitstore.Storer) error {
	if err := a.validateTargets(storer); err != nil {
		return err
	}

	expectedTip, err := currentTip(storer)
	if err != nil {
		return err
	}

	message, err := a.createCommitMessage(false)
	if err != nil {
		return err
	}

	return commitEntry(storer, message, false, expectedTip)
}

// PropagationEntry represents a record of execution of gittuf's repository
// propagation workflow. It indicates which reference was updated with an
// upstream repository's contents, as well as details about the upstream
// repository such as its location and the specific entry whose contents were
// propagated.
type PropagationEntry struct {
	// ID contains the Git hash for the commit corresponding to the entry.
	ID githash.Hash

	// RefName contains the Git reference the entry is for.
	RefName string

	// TargetID contains the Git hash for the object expected at RefName.
	TargetID githash.Hash

	// UpstreamRepository records the location of the upstream repository.
	UpstreamRepository string

	// UpstreamEntryID records the upstream repository's RSL entry ID whose
	// contents were propagated. When the upstream repository uses bulk
	// reference entries, the ID alone does not identify one reference
	// update, because a bulk entry records several. Consumers must pair the
	// ID with the upstream reference named by the propagation directive and
	// resolve it with GetReferenceUpdaterEntryForRef.
	UpstreamEntryID githash.Hash

	// Number contains a strictly increasing number that hints at entry ordering.
	Number uint64

	// CustomFields contains application-defined metadata for the entry.
	CustomFields CustomFields
}

func NewPropagationEntry(refName string, targetID githash.Hash, upstreamRepository string, upstreamEntryID githash.Hash, opts ...EntryOption) *PropagationEntry {
	options := applyEntryOptions(opts)
	return &PropagationEntry{
		RefName:            refName,
		TargetID:           targetID,
		UpstreamRepository: upstreamRepository,
		UpstreamEntryID:    upstreamEntryID,
		CustomFields:       options.customFields,
	}
}

func (e *PropagationEntry) GetID() githash.Hash {
	return e.ID
}

func (e *PropagationEntry) GetRefName() string {
	return e.RefName
}

func (e *PropagationEntry) GetTargetID() githash.Hash {
	return e.TargetID
}

// Commit creates a commit object in the RSL for the PropagationEntry. The
// function looks up the latest committed entry in the RSL and increments the
// number in the new entry. If a parent entry does not exist or the parent
// entry's number is 0 (unset), the current entry's number is set to 1. The
// numbering starts from 1 as 0 is used to signal the lack of numbering.
func (e *PropagationEntry) Commit(storer gitstore.Storer, sign bool) error {
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
// PropagationEntry. The commit is signed using the provided PEM encoded SSH or
// GPG private key. This is only intended for use in gittuf's developer mode or
// in tests. The function looks up the latest committed entry in the RSL and
// increments the number in the new entry. If a parent entry does not exist or
// the parent entry's number is 0 (unset), the current entry's number is set to
// 1. The numbering starts from 1 as 0 is used to signal the lack of numbering.
func (e *PropagationEntry) CommitUsingSpecificKey(storer gitstore.Storer, signingKeyBytes []byte) error {
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

func (e PropagationEntry) GetNumber() uint64 {
	return e.Number
}

func (e *PropagationEntry) GetCustomField(key string) (string, bool) {
	value, has := e.CustomFields[key]
	return value, has
}

func (e *PropagationEntry) setEntryNumber(storer gitstore.Storer) (githash.Hash, error) {
	number, tip, err := nextEntryNumber(storer)
	if err != nil {
		return tip, err
	}

	e.Number = number
	return tip, nil
}

func (e *PropagationEntry) createCommitMessage(includeNumber bool) (string, error) {
	if err := checkSingleLineField(e.RefName); err != nil {
		return "", err
	}
	if err := checkSingleLineField(e.UpstreamRepository); err != nil {
		return "", err
	}
	lines := []string{
		PropagationEntryHeader,
		"",
		fmt.Sprintf("%s: %s", RefKey, e.RefName),
		fmt.Sprintf("%s: %s", TargetIDKey, e.TargetID.String()),
		fmt.Sprintf("%s: %s", UpstreamRepositoryKey, e.UpstreamRepository),
		fmt.Sprintf("%s: %s", UpstreamEntryIDKey, e.UpstreamEntryID.String()),
	}
	if includeNumber && e.Number > 0 {
		lines = append(lines, fmt.Sprintf("%s: %d", NumberKey, e.Number))
	}
	lines, err := appendCustomFieldLines(lines, e.CustomFields)
	if err != nil {
		return "", err
	}
	return strings.Join(lines, "\n"), nil
}

func applyEntryOptions(opts []EntryOption) *entryOptions {
	options := &entryOptions{}
	for _, fn := range opts {
		fn(options)
	}
	return options
}

// GetEntry returns the entry corresponding to entryID.
func GetEntry(storer gitstore.Storer, entryID githash.Hash) (Entry, error) {
	entry, has := cache.getEntry(entryID)
	if has {
		return entry, nil
	}

	commitMessage, err := storer.GetCommitMessage(entryID)
	if err != nil {
		return nil, errors.Join(ErrRSLEntryNotFound, err)
	}

	entry, err = parseRSLEntryText(entryID, commitMessage)
	if err != nil {
		return nil, err
	}

	cache.setEntry(entryID, entry)
	return entry, nil
}

// GetParentForEntry returns the entry's parent RSL entry.
func GetParentForEntry(storer gitstore.Storer, entry Entry) (Entry, error) {
	parentID, has, err := cache.getParent(entry.GetID())
	if err == nil && has {
		// We don't need to check the parent's Number here because it was
		// checked when this was set in the cache
		return GetEntry(storer, parentID)
	}

	parentIDs, err := storer.GetCommitParentIDs(entry.GetID())
	if err != nil {
		return nil, err
	}

	if parentIDs == nil {
		return nil, ErrRSLEntryNotFound
	}

	if len(parentIDs) > 1 {
		return nil, ErrRSLBranchDetected
	}

	parentID = parentIDs[0]
	parentEntry, err := GetEntry(storer, parentID)
	if err != nil {
		return nil, err
	}

	switch entry.GetNumber() {
	case 0, 1:
		// parent entry has to be 0
		if parentEntry.GetNumber() != 0 {
			return nil, ErrInvalidRSLEntry
		}
	default:
		// parent entry has to be 1 less than entry
		if parentEntry.GetNumber() != entry.GetNumber()-1 {
			return nil, ErrInvalidRSLEntry
		}
	}

	cache.setParent(entry.GetID(), parentID)
	return parentEntry, nil
}

// GetNonGittufParentReferenceUpdaterEntryForEntry returns the first RSL
// reference updater entry starting from the specified entry's parent that is
// not for the gittuf namespace.
func GetNonGittufParentReferenceUpdaterEntryForEntry(storer gitstore.Storer, entry Entry) (ReferenceUpdaterEntry, []*AnnotationEntry, error) {
	it, err := GetLatestEntry(storer)
	if err != nil {
		return nil, nil, err
	}

	parentEntry, err := GetParentForEntry(storer, entry)
	if err != nil {
		return nil, nil, err
	}

	allAnnotations := []*AnnotationEntry{}

	for {
		if annotation, isAnnotation := it.(*AnnotationEntry); isAnnotation {
			allAnnotations = append(allAnnotations, annotation)
		}

		it, err = GetParentForEntry(storer, it)
		if err != nil {
			return nil, nil, err
		}

		if it.GetID().Equal(parentEntry.GetID().Bytes()) {
			break
		}
	}

	var targetEntry ReferenceUpdaterEntry
	for {
		if annotation, isAnnotation := it.(*AnnotationEntry); isAnnotation {
			allAnnotations = append(allAnnotations, annotation)
		} else {
			updaters, err := referenceUpdatersNewestFirst(it)
			if err != nil {
				return nil, nil, err
			}
			for _, iterator := range updaters {
				if !strings.HasPrefix(iterator.GetRefName(), gittufNamespacePrefix) {
					targetEntry = iterator
					break
				}
			}
		}

		if targetEntry != nil {
			// we've found the target entry, stop walking the RSL
			break
		}

		it, err = GetParentForEntry(storer, it)
		if err != nil {
			return nil, nil, err
		}
	}

	annotations := filterAnnotationsForRelevantAnnotations(allAnnotations, targetEntry.GetID())

	return targetEntry, annotations, nil
}

// GetLatestEntry returns the latest entry available locally in the RSL.
func GetLatestEntry(storer gitstore.Storer) (Entry, error) {
	commitID, err := storer.GetReference(Ref)
	if err != nil {
		if errors.Is(err, ErrReferenceNotFound) {
			return nil, ErrRSLEntryNotFound
		}
		return nil, err
	}

	return GetEntry(storer, commitID)
}

// GetLatestReferenceUpdaterEntry returns the latest reference updater entry in
// the local RSL that matches the specified conditions.
func GetLatestReferenceUpdaterEntry(storer gitstore.Storer, opts ...GetLatestReferenceUpdaterEntryOption) (ReferenceUpdaterEntry, []*AnnotationEntry, error) {
	options := GetLatestReferenceUpdaterEntryOptions{}
	for _, fn := range opts {
		fn(&options)
	}

	if len(options.BeforeEntryID) != 0 && options.BeforeEntryNumber != 0 {
		// Only one of the Before options can be set
		slog.Debug("Found both before entry ID and before entry number conditions, aborting...")
		return nil, nil, ErrInvalidGetLatestReferenceUpdaterEntryOptions
	}
	if len(options.UntilEntryID) != 0 && options.UntilEntryNumber != 0 {
		// Only one of the Until options can be set
		slog.Debug("Found both until entry ID and until entry number conditions, aborting...")
		return nil, nil, ErrInvalidGetLatestReferenceUpdaterEntryOptions
	}
	if options.BeforeEntryNumber != 0 && options.UntilEntryNumber != 0 && options.BeforeEntryNumber < options.UntilEntryNumber {
		slog.Debug(fmt.Sprintf("Cannot search for entry before entry number %d and until entry number %d, aborting...", options.BeforeEntryNumber, options.UntilEntryNumber))
		return nil, nil, ErrInvalidGetLatestReferenceUpdaterEntryOptions
	}
	if options.IsReferenceEntry && options.IsPropagationEntryForRepository != "" {
		slog.Debug("Found options to require reference entry and propagation entry, aborting...")
		return nil, nil, ErrInvalidGetLatestReferenceUpdaterEntryOptions
	}

	allAnnotations := []*AnnotationEntry{}

	iteratorT, err := GetLatestEntry(storer)
	if err != nil {
		return nil, nil, err
	}

	// Sanity check before / until number conditions
	if iteratorT.GetNumber() == 0 {
		// The repository doesn't use numbers yet
		if options.BeforeEntryNumber != 0 || options.UntilEntryNumber != 0 {
			return nil, nil, ErrCannotUseEntryNumberFilter
		}
	} else if options.UntilEntryNumber != 0 && iteratorT.GetNumber() < options.UntilEntryNumber {
		slog.Debug(fmt.Sprintf("Latest entry's number %d is less than the until number condition %d, aborting...", iteratorT.GetNumber(), options.UntilEntryNumber))
		return nil, nil, ErrInvalidUntilEntryNumberCondition
	}

	// Do initial walk if either before condition is set
	if len(options.BeforeEntryID) != 0 || options.BeforeEntryNumber != 0 {
		slog.Debug("Scanning RSL for search start point using before condition...")
		for !iteratorT.GetID().Equal(options.BeforeEntryID) && (iteratorT.GetNumber() == 0 || iteratorT.GetNumber() != options.BeforeEntryNumber) {
			if annotation, isAnnotation := iteratorT.(*AnnotationEntry); isAnnotation {
				allAnnotations = append(allAnnotations, annotation)
			}

			iteratorT, err = GetParentForEntry(storer, iteratorT)
			if err != nil {
				return nil, nil, err
			}

			if iteratorT.GetNumber() < options.UntilEntryNumber {
				return nil, nil, ErrInvalidGetLatestReferenceUpdaterEntryOptions
			}
		}

		slog.Debug(fmt.Sprintf("Found entry '%s' matching before condition...", iteratorT.GetID().String()))

		// we've found the before anchor entry, track it if it's an
		// annotation
		if annotation, isAnnotation := iteratorT.(*AnnotationEntry); isAnnotation {
			allAnnotations = append(allAnnotations, annotation)
		}

		// Set it to parent as this is the first entry considered below
		// While this entry may match equal until condition, that's fine
		// as the until condition is inclusive
		iteratorT, err = GetParentForEntry(storer, iteratorT)
		if err != nil {
			return nil, nil, err
		}
	}

	var targetEntry ReferenceUpdaterEntry
	for {
		if annotation, isAnnotation := iteratorT.(*AnnotationEntry); isAnnotation {
			allAnnotations = append(allAnnotations, annotation)
		} else {
			updaters, err := referenceUpdatersNewestFirst(iteratorT)
			if err != nil {
				return nil, nil, err
			}

			for _, iterator := range updaters {
				matchesConditions := true

				if options.Reference != "" && iterator.GetRefName() != options.Reference {
					matchesConditions = false
				}

				if matchesConditions && options.IsReferenceEntry {
					if _, isReferenceEntry := iterator.(*ReferenceEntry); !isReferenceEntry {
						matchesConditions = false
					}
				}

				// Only reference entries can be skipped
				referenceEntry, isReferenceEntry := iterator.(*ReferenceEntry)
				if isReferenceEntry {
					if matchesConditions && options.Unskipped && referenceEntry.SkippedBy(allAnnotations) {
						// SkippedBy ensures only the applicable
						// annotations that refer to the entry
						// are used
						matchesConditions = false
					}
				}

				if matchesConditions && options.IsPropagationEntryForRepository != "" {
					propagationEntry, isPropagationEntry := iterator.(*PropagationEntry)
					if !isPropagationEntry || propagationEntry.UpstreamRepository != options.IsPropagationEntryForRepository {
						matchesConditions = false
					}
				}

				if matchesConditions && options.NonGittuf && strings.HasPrefix(iterator.GetRefName(), gittufNamespacePrefix) {
					matchesConditions = false
				}

				if matchesConditions {
					targetEntry = iterator
					break
				}
			}
		}

		if targetEntry != nil {
			// We've found the target entry, stop walking the RSL
			break
		}

		iteratorT, err = GetParentForEntry(storer, iteratorT)
		if err != nil {
			return nil, nil, err
		}

		if options.UntilEntryNumber != 0 && iteratorT.GetNumber() < options.UntilEntryNumber {
			return nil, nil, ErrRSLEntryNotFound
		}

		if len(options.UntilEntryID) != 0 && iteratorT.GetID().Equal(options.UntilEntryID) {
			return nil, nil, ErrRSLEntryNotFound
		}
	}

	annotations := filterAnnotationsForRelevantAnnotations(allAnnotations, targetEntry.GetID())

	return targetEntry, annotations, nil
}

// GetFirstEntry returns the very first entry in the RSL. It is expected to be a
// reference updater entry as the first entry in the RSL cannot be an
// annotation.
func GetFirstEntry(storer gitstore.Storer) (ReferenceUpdaterEntry, []*AnnotationEntry, error) {
	return GetFirstReferenceUpdaterEntryForRef(storer, "")
}

// GetFirstReferenceEntryForRef returns the very first entry in the RSL for the
// specified ref. It is expected to be a reference entry as the first entry in
// the RSL for a reference cannot be an annotation.
func GetFirstReferenceUpdaterEntryForRef(storer gitstore.Storer, targetRef string) (ReferenceUpdaterEntry, []*AnnotationEntry, error) {
	iteratorT, err := GetLatestEntry(storer)
	if err != nil {
		return nil, nil, err
	}

	allAnnotations := []*AnnotationEntry{}
	var firstEntry ReferenceUpdaterEntry

	for {
		if annotation, isAnnotation := iteratorT.(*AnnotationEntry); isAnnotation {
			allAnnotations = append(allAnnotations, annotation)
		} else {
			// Listed order, so the first match in an entry is the oldest
			// update in it. Each entry overwrites the previous candidate
			// because the walk moves towards the start of the RSL.
			updaters, err := referenceUpdaters(iteratorT)
			if err != nil {
				return nil, nil, err
			}
			for _, entry := range updaters {
				if targetRef == "" || entry.GetRefName() == targetRef {
					firstEntry = entry
					break
				}
			}
		}

		parentT, err := GetParentForEntry(storer, iteratorT)
		if err != nil {
			if errors.Is(err, ErrRSLEntryNotFound) {
				break
			}

			return nil, nil, err
		}

		iteratorT = parentT
	}

	if firstEntry == nil {
		return nil, nil, ErrRSLEntryNotFound
	}

	annotations := filterAnnotationsForRelevantAnnotations(allAnnotations, firstEntry.GetID())

	return firstEntry, annotations, nil
}

// SkipAllInvalidReferenceEntriesForRef identifies RSL reference entries for
// targetRef whose targets are not reachable from the ref's latest recorded
// target, which indicates the ref's history was rewritten. It records one
// annotation skipping them. Only entries for targetRef are inspected. When an
// invalid entry is a view of a bulk reference entry, the annotation is
// qualified with targetRef so the other updates in that bulk entry are
// unaffected.
func SkipAllInvalidReferenceEntriesForRef(storer gitstore.Storer, targetRef string, signCommit bool) error {
	slog.Debug("Checking if RSL entries point to commits not in the target ref...")

	latestEntry, _, err := GetLatestReferenceUpdaterEntry(storer, ForReference(targetRef))
	if err != nil {
		return err
	}

	entriesToSkip := []githash.Hash{}
	qualifiers := map[string][]string{}
	sawPriorEntry := false

	// One backwards walk from the entry holding the ref's latest recorded
	// target. GetParentForEntry resolves by commit ID, so it accepts a view
	// of a bulk entry and returns that bulk entry's parent. At most one
	// update in each parent can be for targetRef because refs are unique
	// within an entry.
	var iterator Entry = latestEntry
	foundValidEntry := false
	for !foundValidEntry {
		iterator, err = GetParentForEntry(storer, iterator)
		if err != nil {
			if errors.Is(err, ErrRSLEntryNotFound) {
				break
			}
			return err
		}

		updaters, err := referenceUpdaters(iterator)
		if err != nil {
			return err
		}

		for _, updater := range updaters {
			if updater.GetRefName() != targetRef {
				continue
			}

			entry, isReferenceEntry := updater.(*ReferenceEntry)
			if !isReferenceEntry {
				// Only reference entries and their views can be skipped.
				break
			}
			sawPriorEntry = true

			isAncestor, err := storer.KnowsCommit(latestEntry.GetTargetID(), entry.TargetID)
			if err != nil {
				return err
			}

			if !isAncestor {
				slog.Debug(fmt.Sprintf("For target ref %s, found RSL entry '%s' pointing to a commit, '%s', that does not exist in the target ref.", targetRef, entry.ID, entry.TargetID))
				entriesToSkip = append(entriesToSkip, entry.ID)
				if entry.FromBulkEntry() {
					qualifiers[entry.ID.String()] = []string{targetRef}
				}
			} else {
				slog.Debug(fmt.Sprintf("For target ref %s, found RSL entry '%s' pointing to a commit, '%s', that exists in the target ref. No more commits to skip.", targetRef, entry.ID, entry.TargetID))
				foundValidEntry = true
			}

			break
		}
	}

	if !sawPriorEntry {
		// We don't have a prior entry for the ref to check if invalid, so we
		// assume the current one is valid.
		// TODO: should we cross reference state of the branch?
		return nil
	}

	if len(entriesToSkip) == 0 {
		return nil
	}

	return NewAnnotationEntryWithQualifiers(entriesToSkip, qualifiers, true, "Automated skip of reference entries pointing to non-existent entries").Commit(storer, signCommit)
}

// GetFirstReferenceUpdaterEntryForCommit returns the first reference updater
// that either records the commit itself or a descendant of the commit. It
// walks the RSL from the tip and inspects every non-gittuf update in each
// entry, so a commit reachable only through one update of a bulk entry is
// found. Like the previous implementation it assumes that once a commit is
// recorded, every later entry carrying non-gittuf updates still contains it,
// and it reports ErrNoRecordOfCommit when the most recent non-gittuf updates
// do not contain the commit.
func GetFirstReferenceUpdaterEntryForCommit(storer gitstore.Storer, commitID githash.Hash) (ReferenceUpdaterEntry, []*AnnotationEntry, error) {
	iterator, err := GetLatestEntry(storer)
	if err != nil {
		if errors.Is(err, ErrRSLEntryNotFound) {
			return nil, nil, ErrNoRecordOfCommit
		}
		return nil, nil, err
	}

	allAnnotations := []*AnnotationEntry{}
	var candidate ReferenceUpdaterEntry

	for {
		if annotation, isAnnotation := iterator.(*AnnotationEntry); isAnnotation {
			allAnnotations = append(allAnnotations, annotation)
		} else {
			updaters, err := referenceUpdatersNewestFirst(iterator)
			if err != nil {
				return nil, nil, err
			}

			hasNonGittuf := false
			knowsInThisEntry := false
			for _, updater := range updaters {
				if strings.HasPrefix(updater.GetRefName(), gittufNamespacePrefix) {
					continue
				}
				hasNonGittuf = true

				knows, err := storer.KnowsCommit(updater.GetTargetID(), commitID)
				if err != nil {
					return nil, nil, err
				}
				if knows {
					knowsInThisEntry = true
					// Newest first, so the final assignment is the first
					// listed update in this entry.
					candidate = updater
				}
			}

			if hasNonGittuf && !knowsInThisEntry {
				if candidate == nil {
					return nil, nil, ErrNoRecordOfCommit
				}
				break
			}
		}

		iterator, err = GetParentForEntry(storer, iterator)
		if err != nil {
			if errors.Is(err, ErrRSLEntryNotFound) {
				break
			}
			return nil, nil, err
		}
	}

	if candidate == nil {
		return nil, nil, ErrNoRecordOfCommit
	}

	return candidate, filterAnnotationsForRelevantAnnotations(allAnnotations, candidate.GetID()), nil
}

// GetReferenceUpdaterEntriesInRange returns a list of reference entries between
// the specified range and a map of annotations that refer to each reference
// entry in the range. The annotations map is keyed by the ID of the reference
// entry, with the value being a list of annotations that apply to that
// reference entry.
//
// The annotation lists are per entry, not per reference update. A bulk
// reference entry contributes one view per ref and they all share an entry ID,
// so a list may hold annotations qualified to a different ref in the same
// entry. Consumers must narrow it with AnnotationEntry.AppliesTo or
// ReferenceEntry.SkippedBy.
func GetReferenceUpdaterEntriesInRange(storer gitstore.Storer, firstID, lastID githash.Hash) ([]ReferenceUpdaterEntry, map[string][]*AnnotationEntry, error) {
	return GetReferenceUpdaterEntriesInRangeForRef(storer, firstID, lastID, "")
}

// GetReferenceUpdaterEntriesInRangeForRef returns a list of reference entries
// for the ref between the specified range and a map of annotations that refer
// to each reference entry in the range. The annotations map is keyed by the ID
// of the reference entry, with the value being a list of annotations that apply
// to that reference entry.
//
// Filtering by ref narrows the returned entries but not the annotation lists.
// They stay keyed by entry ID and are therefore per entry, so an annotation
// qualified to another ref in the same bulk reference entry is still present.
// Consumers must narrow it with AnnotationEntry.AppliesTo or
// ReferenceEntry.SkippedBy.
func GetReferenceUpdaterEntriesInRangeForRef(storer gitstore.Storer, firstID, lastID githash.Hash, refName string) ([]ReferenceUpdaterEntry, map[string][]*AnnotationEntry, error) {
	// We have to iterate from latest to get the annotations that refer to the
	// last requested entry
	iterator, err := GetLatestEntry(storer)
	if err != nil {
		return nil, nil, err
	}

	allAnnotations := []*AnnotationEntry{}
	for !iterator.GetID().Equal(lastID.Bytes()) {
		// Until we find the entry corresponding to lastID, we just store
		// annotations
		if annotation, isAnnotation := iterator.(*AnnotationEntry); isAnnotation {
			allAnnotations = append(allAnnotations, annotation)
		}

		parent, err := GetParentForEntry(storer, iterator)
		if err != nil {
			return nil, nil, err
		}
		iterator = parent
	}

	entryStack := []ReferenceUpdaterEntry{}
	inRange := map[string]bool{}
	isRelevant := func(entry ReferenceUpdaterEntry) bool {
		// It's a relevant entry if:
		// a) there's no refName set, or
		// b) the entry's refName matches the set refName, or
		// c) the entry is for a gittuf namespace
		return len(refName) == 0 || entry.GetRefName() == refName || isRelevantGittufRef(entry.GetRefName())
	}
	pushRelevant := func(entry Entry) error {
		if annotation, isAnnotation := entry.(*AnnotationEntry); isAnnotation {
			allAnnotations = append(allAnnotations, annotation)
			return nil
		}
		// Views are pushed newest first because entryStack is reversed
		// below, which restores listed order.
		updaters, err := referenceUpdatersNewestFirst(entry)
		if err != nil {
			return err
		}
		for _, updater := range updaters {
			if isRelevant(updater) {
				entryStack = append(entryStack, updater)
				inRange[updater.GetID().String()] = true
			}
		}
		return nil
	}
	for !iterator.GetID().Equal(firstID.Bytes()) {
		// Here, all items are relevant until the one corresponding to first is
		// found
		if err := pushRelevant(iterator); err != nil {
			return nil, nil, err
		}

		parent, err := GetParentForEntry(storer, iterator)
		if err != nil {
			return nil, nil, err
		}
		iterator = parent
	}

	// Handle the item corresponding to first explicitly. If it's an
	// annotation, ignore it as it refers to something before the range we
	// care about, so pushRelevant is only called for updater entries.
	if _, isAnnotation := iterator.(*AnnotationEntry); !isAnnotation {
		if err := pushRelevant(iterator); err != nil {
			return nil, nil, err
		}
	}

	// For each annotation, add the entry to each relevant entry it refers to
	// Process annotations in reverse order so that annotations are listed in
	// order of occurrence in the map
	annotationMap := map[string][]*AnnotationEntry{}
	for i := len(allAnnotations) - 1; i >= 0; i-- {
		annotation := allAnnotations[i]
		for _, entryID := range annotation.RSLEntryIDs {
			if _, relevant := inRange[entryID.String()]; relevant {
				// Annotation is relevant because the entry it refers to was in
				// the specified range
				if _, exists := annotationMap[entryID.String()]; !exists {
					annotationMap[entryID.String()] = []*AnnotationEntry{}
				}

				annotationMap[entryID.String()] = append(annotationMap[entryID.String()], annotation)
			}
		}
	}

	// Reverse entryStack so that it's in order of occurrence rather than in
	// order of walking back the RSL
	allEntries := make([]ReferenceUpdaterEntry, 0, len(entryStack))
	for i := len(entryStack) - 1; i >= 0; i-- {
		allEntries = append(allEntries, entryStack[i])
	}

	return allEntries, annotationMap, nil
}

// ParseEntryText parses a single RSL commit message into its typed Entry
// without any repository access. id is recorded as the entry's ID. It returns
// ErrInvalidRSLEntry if text is not a well-formed RSL entry of any known type.
// This is the entry point for consumers that already hold the commit message
// (e.g. a server walking its own RSL) and do not want a gitstore.Storer or
// the entry cache.
func ParseEntryText(id githash.Hash, text string) (Entry, error) {
	return parseRSLEntryText(id, text)
}

const unknownEntryTypeGuidance = "This repository may use a gittuf feature newer than this client. Upgrade gittuf to the latest release and retry. If this client is already the latest release, the entry may be corrupt or malicious and should be reported to the repository owners."

const maxHeaderInError = 80

func newUnknownEntryTypeError(id githash.Hash, text string) error {
	header, _, _ := strings.Cut(text, "\n")
	if len(header) > maxHeaderInError {
		header = header[:maxHeaderInError] + "..."
	}
	return fmt.Errorf("%w: entry %s has header %q. %s", ErrUnknownRSLEntryType, id.String(), header, unknownEntryTypeGuidance)
}

func parseRSLEntryText(id githash.Hash, text string) (Entry, error) {
	// Each parser returns a concrete pointer type. Assign to a local and return
	// an explicit nil interface on error: returning the typed nil pointer
	// directly would yield a non-nil Entry wrapping a nil pointer.
	switch {
	case strings.HasPrefix(text, ReferenceEntryHeader):
		entry, err := parseReferenceEntryText(id, text)
		if err != nil {
			return nil, err
		}
		return entry, nil
	case strings.HasPrefix(text, AnnotationEntryHeader):
		entry, err := parseAnnotationEntryText(id, text)
		if err != nil {
			return nil, err
		}
		return entry, nil
	case strings.HasPrefix(text, PropagationEntryHeader):
		entry, err := parsePropagationEntryText(id, text)
		if err != nil {
			return nil, err
		}
		return entry, nil
	case strings.HasPrefix(text, BulkReferenceEntryHeader):
		entry, err := parseBulkReferenceEntryText(id, text)
		if err != nil {
			return nil, err
		}
		return entry, nil
	default:
		return nil, newUnknownEntryTypeError(id, text)
	}
}

// parseReferenceEntryText parses a reference entry as a state machine. The
// fields must appear in the order ref, targetID, number, each at most once;
// number is optional and trailing. Out-of-order fields and duplicates are
// rejected. Custom fields must appear after the built-in fields. Among
// themselves, they may appear in any order and use the first value when
// repeated. Other unknown keys are ignored for forward compatibility.
func parseReferenceEntryText(id githash.Hash, text string) (*ReferenceEntry, error) {
	body, err := entryBody(text, ReferenceEntryHeader)
	if err != nil {
		return nil, err
	}

	const (
		expectRef = iota
		expectTargetID
		expectNumber
		done
	)

	entry := &ReferenceEntry{ID: id}
	state := expectRef
	for _, line := range body {
		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			return nil, ErrInvalidRSLEntry
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)

		switch key {
		case RefKey:
			if state != expectRef {
				return nil, ErrInvalidRSLEntry
			}
			entry.RefName = value
			state = expectTargetID

		case TargetIDKey:
			if state != expectTargetID {
				return nil, ErrInvalidRSLEntry
			}
			if err := setHash(&entry.TargetID, value); err != nil {
				return nil, err
			}
			state = expectNumber

		case NumberKey:
			if state != expectNumber {
				return nil, ErrInvalidRSLEntry
			}
			if err := setNumber(&entry.Number, value); err != nil {
				return nil, err
			}
			state = done

		default:
			if strings.HasPrefix(key, customfields.Prefix) {
				if state < expectNumber {
					return nil, ErrInvalidRSLEntry
				}
				state = done
				setCustomField(&entry.CustomFields, key, value)
			}
		}
	}

	if state < expectNumber {
		// ref and/or targetID were not seen.
		return nil, ErrInvalidRSLEntry
	}
	return entry, nil
}

// parseAnnotationEntryText parses an annotation entry as a state machine. One or
// more entryID fields come first, followed by skip, then an optional number,
// then an optional PEM message block. The message is decoded separately, so the
// state machine stops at its begin marker. Custom fields must appear after the
// built-in fields and before the message block. Among themselves they may
// appear in any order and use the first value when repeated.
func parseAnnotationEntryText(id githash.Hash, text string) (*AnnotationEntry, error) {
	annotation := &AnnotationEntry{
		ID:          id,
		RSLEntryIDs: []githash.Hash{},
	}

	body, err := entryBody(text, AnnotationEntryHeader)
	if err != nil {
		return nil, err
	}

	// Only attempt to decode a message when the begin marker is present; most
	// annotations carry no message, so this avoids copying the text and running
	// the PEM scanner for them.
	if strings.Contains(text, BeginMessage) {
		messageBlock, _ := pem.Decode([]byte(text)) // rest doesn't seem to work when the PEM block is at the end of text, see: https://go.dev/play/p/oZysAfemA-v
		if messageBlock != nil {
			annotation.Message = string(messageBlock.Bytes)
		}
	}

	const (
		expectEntryID = iota // one or more entryIDs, then skip
		expectNumber         // entryIDs and skip seen; optional number
		done
	)

	state := expectEntryID
	for _, line := range body {
		line = strings.TrimSpace(line)
		if line == BeginMessage {
			break
		}

		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return nil, ErrInvalidRSLEntry
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)

		switch key {
		case EntryIDKey:
			if state != expectEntryID {
				return nil, ErrInvalidRSLEntry
			}
			hash, err := NewHash(value)
			if err != nil {
				return nil, err
			}
			annotation.RSLEntryIDs = append(annotation.RSLEntryIDs, hash)

		case RefKey:
			if state != expectEntryID || len(annotation.RSLEntryIDs) == 0 {
				return nil, ErrInvalidRSLEntry
			}
			if err := validateBulkRefName(value); err != nil {
				return nil, err
			}
			if annotation.Refs == nil {
				annotation.Refs = map[string][]string{}
			}
			lastID := annotation.RSLEntryIDs[len(annotation.RSLEntryIDs)-1].String()
			// The same ref for the same entry is recorded once. An entry ID
			// may appear more than once in the message, and the writer
			// re-emits its qualifiers each time, so appending blindly would
			// double the list on every round trip.
			if !slices.Contains(annotation.Refs[lastID], value) {
				annotation.Refs[lastID] = append(annotation.Refs[lastID], value)
			}

		case SkipKey:
			if state != expectEntryID || len(annotation.RSLEntryIDs) == 0 {
				return nil, ErrInvalidRSLEntry
			}
			switch value {
			case "true":
				annotation.Skip = true
			case "false":
				annotation.Skip = false
			default:
				return nil, ErrInvalidRSLEntry
			}
			state = expectNumber

		case NumberKey:
			if state != expectNumber {
				return nil, ErrInvalidRSLEntry
			}
			if err := setNumber(&annotation.Number, value); err != nil {
				return nil, err
			}
			state = done

		default:
			if strings.HasPrefix(key, customfields.Prefix) {
				if state < expectNumber {
					return nil, ErrInvalidRSLEntry
				}
				state = done
				setCustomField(&annotation.CustomFields, key, value)
			}
		}
	}

	if state < expectNumber {
		// entryID(s) and/or skip were not seen.
		return nil, ErrInvalidRSLEntry
	}
	return annotation, nil
}

// parsePropagationEntryText parses a propagation entry as a state machine. The
// fields must appear in the order ref, targetID, upstreamRepository,
// upstreamEntryID, number, each at most once; number is optional and trailing.
// Custom fields must appear after the built-in fields. Among themselves they
// may appear in any order and use the first value when repeated.
func parsePropagationEntryText(id githash.Hash, text string) (*PropagationEntry, error) {
	body, err := entryBody(text, PropagationEntryHeader)
	if err != nil {
		return nil, err
	}

	const (
		expectRef = iota
		expectTargetID
		expectUpstreamRepository
		expectUpstreamEntryID
		expectNumber
		done
	)

	entry := &PropagationEntry{ID: id}
	state := expectRef
	for _, line := range body {
		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			return nil, ErrInvalidRSLEntry
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)

		switch key {
		case RefKey:
			if state != expectRef {
				return nil, ErrInvalidRSLEntry
			}
			entry.RefName = value
			state = expectTargetID

		case TargetIDKey:
			if state != expectTargetID {
				return nil, ErrInvalidRSLEntry
			}
			if err := setHash(&entry.TargetID, value); err != nil {
				return nil, err
			}
			state = expectUpstreamRepository

		case UpstreamRepositoryKey:
			if state != expectUpstreamRepository {
				return nil, ErrInvalidRSLEntry
			}
			// The location may also contain ':', so value retains everything
			// after the first separator.
			entry.UpstreamRepository = value
			state = expectUpstreamEntryID

		case UpstreamEntryIDKey:
			if state != expectUpstreamEntryID {
				return nil, ErrInvalidRSLEntry
			}
			if err := setHash(&entry.UpstreamEntryID, value); err != nil {
				return nil, err
			}
			state = expectNumber

		case NumberKey:
			if state != expectNumber {
				return nil, ErrInvalidRSLEntry
			}
			if err := setNumber(&entry.Number, value); err != nil {
				return nil, err
			}
			state = done

		default:
			if strings.HasPrefix(key, customfields.Prefix) {
				if state < expectNumber {
					return nil, ErrInvalidRSLEntry
				}
				state = done
				setCustomField(&entry.CustomFields, key, value)
			}
		}
	}

	if state < expectNumber {
		// A required field before number was not seen.
		return nil, ErrInvalidRSLEntry
	}
	return entry, nil
}

// entryBody validates the entry's header line and the mandatory blank line that
// follows it, returning the remaining body lines for the state machine.
func entryBody(text, header string) ([]string, error) {
	lines := strings.Split(text, "\n")
	if len(lines) < 2 || lines[0] != header || strings.TrimSpace(lines[1]) != "" {
		return nil, ErrInvalidRSLEntry
	}
	return lines[2:], nil
}

func setHash(dst *githash.Hash, value string) error {
	h, err := NewHash(value)
	if err != nil {
		return err
	}
	*dst = h
	return nil
}

func setNumber(dst *uint64, value string) error {
	number, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return err
	}
	*dst = number
	return nil
}

// filterAnnotationsForRelevantAnnotations returns the annotations that refer
// to entryID. The result is per entry, not per reference update: a qualified
// annotation is kept for the whole entry, so every per-ref view of a bulk
// reference entry receives it. Consumers must narrow the list themselves with
// AnnotationEntry.AppliesTo or ReferenceEntry.SkippedBy.
func filterAnnotationsForRelevantAnnotations(allAnnotations []*AnnotationEntry, entryID githash.Hash) []*AnnotationEntry {
	annotations := []*AnnotationEntry{}
	for _, annotation := range allAnnotations {
		annotation := annotation
		if annotation.RefersTo(entryID) {
			annotations = append(annotations, annotation)
		}
	}

	if len(annotations) == 0 {
		return nil
	}

	return annotations
}

func isRelevantGittufRef(refName string) bool {
	if !strings.HasPrefix(refName, gittufNamespacePrefix) {
		return false
	}

	if refName == gittufPolicyStagingRef {
		return false
	}

	return true
}
