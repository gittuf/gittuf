// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package rsl

import (
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/tuf"
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
)

// RemoteTrackerRef returns the remote tracking ref for the specified remote
// name. For example, for 'origin', the remote tracker ref is
// 'refs/remotes/origin/gittuf/reference-state-log'.
func RemoteTrackerRef(remote string) string {
	return fmt.Sprintf(remoteTrackerRef, remote)
}

// Entry is the abstract representation of an object in the RSL.
type Entry interface {
	GetID() gitinterface.Hash
	Commit(*gitinterface.Repository, bool) error
	GetNumber() uint64
	createCommitMessage(bool) (string, error)
}

// ReferenceUpdaterEntry represents RSL entry types that can record an update to
// a Git reference. Some examples are the reference entry and the propagation
// entry.
type ReferenceUpdaterEntry interface {
	Entry
	GetRefName() string
	GetTargetID() gitinterface.Hash
}

// ReferenceEntry represents a record of a reference state in the RSL. It
// implements the Entry interface.
type ReferenceEntry struct {
	// ID contains the Git hash for the commit corresponding to the entry.
	ID gitinterface.Hash

	// RefName contains the Git reference the entry is for.
	RefName string

	// TargetID contains the Git hash for the object expected at RefName.
	TargetID gitinterface.Hash

	// Number contains a strictly increasing number that hints at entry ordering.
	Number uint64
}

// NewReferenceEntry returns a ReferenceEntry object for a normal RSL entry.
func NewReferenceEntry(refName string, targetID gitinterface.Hash) *ReferenceEntry {
	return &ReferenceEntry{RefName: refName, TargetID: targetID}
}

func (e *ReferenceEntry) GetID() gitinterface.Hash {
	return e.ID
}

func (e *ReferenceEntry) GetRefName() string {
	return e.RefName
}

func (e *ReferenceEntry) GetTargetID() gitinterface.Hash {
	return e.TargetID
}

// Commit creates a commit object in the RSL for the ReferenceEntry. The
// function looks up the latest committed entry in the RSL and increments the
// number in the new entry. If a parent entry does not exist or the parent
// entry's number is 0 (unset), the current entry's number is set to 1. The
// numbering starts from 1 as 0 is used to signal the lack of numbering.
func (e *ReferenceEntry) Commit(repo *gitinterface.Repository, sign bool) error {
	if err := e.setEntryNumber(repo); err != nil {
		return err
	}

	message, _ := e.createCommitMessage(true) // we have an error return for annotations, always nil here

	emptyTreeID, err := repo.EmptyTree()
	if err != nil {
		return err
	}

	_, err = repo.Commit(emptyTreeID, Ref, message, sign)
	return err
}

// CommitUsingSpecificKey creates a commit object in the RSL for the
// ReferenceEntry. The commit is signed using the provided PEM encoded SSH or
// GPG private key. This is only intended for use in gittuf's developer mode or
// in tests. The function looks up the latest committed entry in the RSL and
// increments the number in the new entry. If a parent entry does not exist or
// the parent entry's number is 0 (unset), the current entry's number is set to
// 1. The numbering starts from 1 as 0 is used to signal the lack of numbering.
func (e *ReferenceEntry) CommitUsingSpecificKey(repo *gitinterface.Repository, signingKeyBytes []byte) error {
	if err := e.setEntryNumber(repo); err != nil {
		return err
	}

	message, _ := e.createCommitMessage(true) // we have an error return for annotations, always nil here

	emptyTreeID, err := repo.EmptyTree()
	if err != nil {
		return err
	}

	_, err = repo.CommitUsingSpecificKey(emptyTreeID, Ref, message, signingKeyBytes)
	return err
}

func (e *ReferenceEntry) GetNumber() uint64 {
	return e.Number
}

// Skipped returns true if any of the annotations mark the entry as
// to-be-skipped.
func (e *ReferenceEntry) SkippedBy(annotations []*AnnotationEntry) bool {
	for _, annotation := range annotations {
		if annotation.RefersTo(e.ID) && annotation.Skip {
			return true
		}
	}

	return false
}

func (e *ReferenceEntry) setEntryNumber(repo *gitinterface.Repository) error {
	latestEntry, err := GetLatestEntry(repo)
	if err == nil {
		e.Number = latestEntry.GetNumber() + 1
	} else {
		if errors.Is(err, ErrRSLEntryNotFound) {
			// First entry
			e.Number = 1
		} else {
			return err
		}
	}

	return nil
}

func (e *ReferenceEntry) createCommitMessage(includeNumber bool) (string, error) {
	lines := []string{
		ReferenceEntryHeader,
		"",
		fmt.Sprintf("%s: %s", RefKey, e.RefName),
		fmt.Sprintf("%s: %s", TargetIDKey, e.TargetID.String()),
	}
	if includeNumber && e.Number > 0 {
		lines = append(lines, fmt.Sprintf("%s: %d", NumberKey, e.Number))
	}
	return strings.Join(lines, "\n"), nil
}

// commitWithoutNumber is used to test the RSL's support for entry numbers in
// repositories that switch from not having numbered entries to having numbered
// entries.
func (e *ReferenceEntry) commitWithoutNumber(repo *gitinterface.Repository) error {
	message, _ := e.createCommitMessage(true) // we have an error return for annotations, always nil here

	emptyTreeID, err := repo.EmptyTree()
	if err != nil {
		return err
	}

	_, err = repo.Commit(emptyTreeID, Ref, message, false)
	return err
}

// AnnotationEntry is a type of RSL record that references prior items in the
// RSL. It can be used to add extra information for the referenced items.
// Annotations can also be used to "skip", i.e. revoke, the referenced items. It
// implements the Entry interface.
type AnnotationEntry struct {
	// ID contains the Git hash for the commit corresponding to the annotation.
	ID gitinterface.Hash

	// RSLEntryIDs contains one or more Git hashes for the RSL entries the annotation applies to.
	RSLEntryIDs []gitinterface.Hash

	// Skip indicates if the RSLEntryIDs must be skipped during gittuf workflows.
	Skip bool

	// Message contains any messages or notes added by a user for the annotation.
	Message string

	// Number contains a strictly increasing number that hints at entry ordering.
	Number uint64
}

// NewAnnotationEntry returns an Annotation object that applies to one or more
// prior RSL entries.
func NewAnnotationEntry(rslEntryIDs []gitinterface.Hash, skip bool, message string) *AnnotationEntry {
	return &AnnotationEntry{RSLEntryIDs: rslEntryIDs, Skip: skip, Message: message}
}

func (a *AnnotationEntry) GetID() gitinterface.Hash {
	return a.ID
}

// Commit creates a commit object in the RSL for the Annotation. The function
// looks up the latest committed entry in the RSL and increments the number in
// the new entry. If a parent entry does not exist or the parent entry's number
// is 0 (unset), the current entry's number is set to 1. The numbering starts
// from 1 as 0 is used to signal the lack of numbering.
func (a *AnnotationEntry) Commit(repo *gitinterface.Repository, sign bool) error {
	// Check if referred entries exist in the RSL namespace.
	for _, id := range a.RSLEntryIDs {
		if _, err := GetEntry(repo, id); err != nil {
			return err
		}
	}

	if err := a.setEntryNumber(repo); err != nil {
		return err
	}

	message, err := a.createCommitMessage(true)
	if err != nil {
		return err
	}

	emptyTreeID, err := repo.EmptyTree()
	if err != nil {
		return err
	}

	_, err = repo.Commit(emptyTreeID, Ref, message, sign)
	return err
}

// CommitUsingSpecificKey creates a commit object in the RSL for the
// AnnotationEntry. The commit is signed using the provided PEM encoded SSH or
// GPG private key. This is only intended for use in gittuf's developer mode or
// in tests. The function looks up the latest committed entry in the RSL and
// increments the number in the new entry. If a parent entry does not exist or
// the parent entry's number is 0 (unset), the current entry's number is set to
// 1. The numbering starts from 1 as 0 is used to signal the lack of numbering.
func (a *AnnotationEntry) CommitUsingSpecificKey(repo *gitinterface.Repository, signingKeyBytes []byte) error {
	// Check if referred entries exist in the RSL namespace.
	for _, id := range a.RSLEntryIDs {
		if _, err := GetEntry(repo, id); err != nil {
			return err
		}
	}

	if err := a.setEntryNumber(repo); err != nil {
		return err
	}

	message, err := a.createCommitMessage(true)
	if err != nil {
		return err
	}

	emptyTreeID, err := repo.EmptyTree()
	if err != nil {
		return err
	}

	_, err = repo.CommitUsingSpecificKey(emptyTreeID, Ref, message, signingKeyBytes)
	return err
}

func (a *AnnotationEntry) GetNumber() uint64 {
	return a.Number
}

// RefersTo returns true if the specified entryID is referred to by the
// annotation.
func (a *AnnotationEntry) RefersTo(entryID gitinterface.Hash) bool {
	for _, id := range a.RSLEntryIDs {
		if id.Equal(entryID) {
			return true
		}
	}

	return false
}

func (a *AnnotationEntry) setEntryNumber(repo *gitinterface.Repository) error {
	latestEntry, err := GetLatestEntry(repo)
	if err == nil {
		a.Number = latestEntry.GetNumber() + 1
	} else {
		if errors.Is(err, ErrRSLEntryNotFound) {
			// First entry -> can an annotation actually be first? TODO
			a.Number = 1
		} else {
			return err
		}
	}

	return err
}

func (a *AnnotationEntry) createCommitMessage(includeNumber bool) (string, error) {
	lines := []string{
		AnnotationEntryHeader,
		"",
	}

	for _, entry := range a.RSLEntryIDs {
		lines = append(lines, fmt.Sprintf("%s: %s", EntryIDKey, entry.String()))
	}

	if a.Skip {
		lines = append(lines, fmt.Sprintf("%s: true", SkipKey))
	} else {
		lines = append(lines, fmt.Sprintf("%s: false", SkipKey))
	}

	if includeNumber && a.Number > 0 {
		lines = append(lines, fmt.Sprintf("%s: %d", NumberKey, a.Number))
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

// commitWithoutNumber is used to test the RSL's support for entry numbers in
// repositories that switch from not having numbered entries to having numbered
// entries.
func (a *AnnotationEntry) commitWithoutNumber(repo *gitinterface.Repository) error {
	// Check if referred entries exist in the RSL namespace.
	for _, id := range a.RSLEntryIDs {
		if _, err := GetEntry(repo, id); err != nil {
			return err
		}
	}

	message, err := a.createCommitMessage(true)
	if err != nil {
		return err
	}

	emptyTreeID, err := repo.EmptyTree()
	if err != nil {
		return err
	}

	_, err = repo.Commit(emptyTreeID, Ref, message, false)
	return err
}

// PropagationEntry represents a record of execution of gittuf's repository
// propagation workflow. It indicates which reference was updated with an
// upstream repository's contents, as well as details about the upstream
// repository such as its location and the specific entry whose contents were
// propagated.
type PropagationEntry struct {
	// ID contains the Git hash for the commit corresponding to the entry.
	ID gitinterface.Hash

	// RefName contains the Git reference the entry is for.
	RefName string

	// TargetID contains the Git hash for the object expected at RefName.
	TargetID gitinterface.Hash

	// UpstreamRepository records the location of the upstream repository.
	UpstreamRepository string

	// UpstreamEntryID records the upstream repository's RSL entry ID whose
	// contents were propagated.
	UpstreamEntryID gitinterface.Hash

	// Number contains a strictly increasing number that hints at entry ordering.
	Number uint64
}

func NewPropagationEntry(refName string, targetID gitinterface.Hash, upstreamRepository string, upstreamEntryID gitinterface.Hash) *PropagationEntry {
	return &PropagationEntry{
		RefName:            refName,
		TargetID:           targetID,
		UpstreamRepository: upstreamRepository,
		UpstreamEntryID:    upstreamEntryID,
	}
}

func (e *PropagationEntry) GetID() gitinterface.Hash {
	return e.ID
}

func (e *PropagationEntry) GetRefName() string {
	return e.RefName
}

func (e *PropagationEntry) GetTargetID() gitinterface.Hash {
	return e.TargetID
}

// Commit creates a commit object in the RSL for the PropagationEntry. The
// function looks up the latest committed entry in the RSL and increments the
// number in the new entry. If a parent entry does not exist or the parent
// entry's number is 0 (unset), the current entry's number is set to 1. The
// numbering starts from 1 as 0 is used to signal the lack of numbering.
func (e *PropagationEntry) Commit(repo *gitinterface.Repository, sign bool) error {
	if err := e.setEntryNumber(repo); err != nil {
		return err
	}

	message, _ := e.createCommitMessage(true) // we have an error return for annotations, always nil here

	emptyTreeID, err := repo.EmptyTree()
	if err != nil {
		return err
	}

	_, err = repo.Commit(emptyTreeID, Ref, message, sign)
	return err
}

// CommitUsingSpecificKey creates a commit object in the RSL for the
// PropagationEntry. The commit is signed using the provided PEM encoded SSH or
// GPG private key. This is only intended for use in gittuf's developer mode or
// in tests. The function looks up the latest committed entry in the RSL and
// increments the number in the new entry. If a parent entry does not exist or
// the parent entry's number is 0 (unset), the current entry's number is set to
// 1. The numbering starts from 1 as 0 is used to signal the lack of numbering.
func (e *PropagationEntry) CommitUsingSpecificKey(repo *gitinterface.Repository, signingKeyBytes []byte) error {
	if err := e.setEntryNumber(repo); err != nil {
		return err
	}

	message, _ := e.createCommitMessage(true) // we have an error return for annotations, always nil here

	emptyTreeID, err := repo.EmptyTree()
	if err != nil {
		return err
	}

	_, err = repo.CommitUsingSpecificKey(emptyTreeID, Ref, message, signingKeyBytes)
	return err
}

func (e PropagationEntry) GetNumber() uint64 {
	return e.Number
}

func (e *PropagationEntry) setEntryNumber(repo *gitinterface.Repository) error {
	latestEntry, err := GetLatestEntry(repo)
	if err == nil {
		e.Number = latestEntry.GetNumber() + 1
	} else {
		if errors.Is(err, ErrRSLEntryNotFound) {
			// First entry
			e.Number = 1
		} else {
			return err
		}
	}

	return nil
}

func (e *PropagationEntry) createCommitMessage(includeNumber bool) (string, error) {
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
	return strings.Join(lines, "\n"), nil
}

// GetEntry returns the entry corresponding to entryID.
func GetEntry(repo *gitinterface.Repository, entryID gitinterface.Hash) (Entry, error) {
	entry, has := cache.getEntry(entryID)
	if has {
		return entry, nil
	}

	commitMessage, err := repo.GetCommitMessage(entryID)
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
func GetParentForEntry(repo *gitinterface.Repository, entry Entry) (Entry, error) {
	parentID, has, err := cache.getParent(entry.GetID())
	if err == nil && has {
		// We don't need to check the parent's Number here because it was
		// checked when this was set in the cache
		return GetEntry(repo, parentID)
	}

	parentIDs, err := repo.GetCommitParentIDs(entry.GetID())
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
	parentEntry, err := GetEntry(repo, parentID)
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
func GetNonGittufParentReferenceUpdaterEntryForEntry(repo *gitinterface.Repository, entry Entry) (ReferenceUpdaterEntry, []*AnnotationEntry, error) {
	it, err := GetLatestEntry(repo)
	if err != nil {
		return nil, nil, err
	}

	parentEntry, err := GetParentForEntry(repo, entry)
	if err != nil {
		return nil, nil, err
	}

	allAnnotations := []*AnnotationEntry{}

	for {
		if annotation, isAnnotation := it.(*AnnotationEntry); isAnnotation {
			allAnnotations = append(allAnnotations, annotation)
		}

		it, err = GetParentForEntry(repo, it)
		if err != nil {
			return nil, nil, err
		}

		if it.GetID().Equal(parentEntry.GetID()) {
			break
		}
	}

	var targetEntry ReferenceUpdaterEntry
	for {
		switch iterator := it.(type) {
		case ReferenceUpdaterEntry:
			if !strings.HasPrefix(iterator.GetRefName(), gittufNamespacePrefix) {
				targetEntry = iterator
			}
		case *AnnotationEntry:
			allAnnotations = append(allAnnotations, iterator)
		}

		if targetEntry != nil {
			// we've found the target entry, stop walking the RSL
			break
		}

		it, err = GetParentForEntry(repo, it)
		if err != nil {
			return nil, nil, err
		}
	}

	annotations := filterAnnotationsForRelevantAnnotations(allAnnotations, targetEntry.GetID())

	return targetEntry, annotations, nil
}

// GetLatestEntry returns the latest entry available locally in the RSL.
func GetLatestEntry(repo *gitinterface.Repository) (Entry, error) {
	commitID, err := repo.GetReference(Ref)
	if err != nil {
		if errors.Is(err, gitinterface.ErrReferenceNotFound) {
			return nil, ErrRSLEntryNotFound
		}
		return nil, err
	}

	return GetEntry(repo, commitID)
}

// GetLatestReferenceUpdaterEntry returns the latest reference updater entry in
// the local RSL that matches the specified conditions.
func GetLatestReferenceUpdaterEntry(repo *gitinterface.Repository, opts ...GetLatestReferenceUpdaterEntryOption) (ReferenceUpdaterEntry, []*AnnotationEntry, error) {
	options := GetLatestReferenceUpdaterEntryOptions{
		BeforeEntryID: gitinterface.ZeroHash,
		UntilEntryID:  gitinterface.ZeroHash,
	}
	for _, fn := range opts {
		fn(&options)
	}

	if !options.BeforeEntryID.IsZero() && options.BeforeEntryNumber != 0 {
		// Only one of the Before options can be set
		slog.Debug("Found both before entry ID and before entry number conditions, aborting...")
		return nil, nil, ErrInvalidGetLatestReferenceUpdaterEntryOptions
	}
	if !options.UntilEntryID.IsZero() && options.UntilEntryNumber != 0 {
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

	iteratorT, err := GetLatestEntry(repo)
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
	if !options.BeforeEntryID.IsZero() || options.BeforeEntryNumber != 0 {
		slog.Debug("Scanning RSL for search start point using before condition...")
		for !iteratorT.GetID().Equal(options.BeforeEntryID) && (iteratorT.GetNumber() == 0 || iteratorT.GetNumber() != options.BeforeEntryNumber) {
			if annotation, isAnnotation := iteratorT.(*AnnotationEntry); isAnnotation {
				allAnnotations = append(allAnnotations, annotation)
			}

			iteratorT, err = GetParentForEntry(repo, iteratorT)
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
		iteratorT, err = GetParentForEntry(repo, iteratorT)
		if err != nil {
			return nil, nil, err
		}
	}

	var targetEntry ReferenceUpdaterEntry
	for {
		switch iterator := iteratorT.(type) {
		case ReferenceUpdaterEntry:
			matchesConditions := true

			if options.Reference != "" && iterator.GetRefName() != options.Reference {
				matchesConditions = false
			}

			if matchesConditions && options.IsReferenceEntry {
				if _, isReferenceEntry := iterator.(*ReferenceEntry); !isReferenceEntry {
					matchesConditions = false
				}
			}

			// Only reference entry can be skipped
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
			}

		case *AnnotationEntry:
			allAnnotations = append(allAnnotations, iterator)
		}

		if targetEntry != nil {
			// We've found the target entry, stop walking the RSL
			break
		}

		iteratorT, err = GetParentForEntry(repo, iteratorT)
		if err != nil {
			return nil, nil, err
		}

		if options.UntilEntryNumber != 0 && iteratorT.GetNumber() < options.UntilEntryNumber {
			return nil, nil, ErrRSLEntryNotFound
		}

		if !options.UntilEntryID.IsZero() && iteratorT.GetID().Equal(options.UntilEntryID) {
			return nil, nil, ErrRSLEntryNotFound
		}
	}

	annotations := filterAnnotationsForRelevantAnnotations(allAnnotations, targetEntry.GetID())

	return targetEntry, annotations, nil
}

// GetFirstEntry returns the very first entry in the RSL. It is expected to be a
// reference updater entry as the first entry in the RSL cannot be an
// annotation.
func GetFirstEntry(repo *gitinterface.Repository) (ReferenceUpdaterEntry, []*AnnotationEntry, error) {
	return GetFirstReferenceUpdaterEntryForRef(repo, "")
}

// GetFirstReferenceEntryForRef returns the very first entry in the RSL for the
// specified ref. It is expected to be a reference entry as the first entry in
// the RSL for a reference cannot be an annotation.
func GetFirstReferenceUpdaterEntryForRef(repo *gitinterface.Repository, targetRef string) (ReferenceUpdaterEntry, []*AnnotationEntry, error) {
	iteratorT, err := GetLatestEntry(repo)
	if err != nil {
		return nil, nil, err
	}

	allAnnotations := []*AnnotationEntry{}
	var firstEntry ReferenceUpdaterEntry

	for {
		switch entry := iteratorT.(type) {
		case ReferenceUpdaterEntry:
			if targetRef == "" || entry.GetRefName() == targetRef {
				firstEntry = entry
			}
		case *AnnotationEntry:
			allAnnotations = append(allAnnotations, entry)
		}

		parentT, err := GetParentForEntry(repo, iteratorT)
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

// SkipAllInvalidReferenceEntriesForRef identifies invalid RSL reference entries.
// Each invalid entry points to a target that is not reachable for the current
// target of the same reference, indicating that history has been rewritten via a
// rebase for the reference. After the invalid entries are identified, an annotation
// entry is created that marks all of these entries as to be skipped.
func SkipAllInvalidReferenceEntriesForRef(repo *gitinterface.Repository, targetRef string, signCommit bool) error {
	slog.Debug("Checking if RSL entries point to commits not in the target ref...")

	latestEntry, _, err := GetLatestReferenceUpdaterEntry(repo, ForReference(targetRef))
	if err != nil {
		return err
	}

	iteratorEntry, _, err := GetLatestReferenceUpdaterEntry(repo, ForReference(targetRef), BeforeEntryID(latestEntry.GetID()))
	if err != nil {
		if errors.Is(err, ErrRSLEntryNotFound) {
			// We don't have a parent to check if invalid
			// So we assume the current one is valid
			// TODO: should we cross reference state of the branch?
			return nil
		}

		return err
	}
	iterator := Entry(iteratorEntry)

	entriesToSkip := []gitinterface.Hash{}

	for {
		if entry, ok := iterator.(*ReferenceEntry); ok {
			isAncestor, err := repo.KnowsCommit(latestEntry.GetTargetID(), entry.TargetID)
			if err != nil {
				return err
			}

			if !isAncestor {
				slog.Debug(fmt.Sprintf("For target ref %s, found RSL entry '%s' pointing to a commit, '%s', that does not exist in the target ref.", targetRef, entry.ID, entry.TargetID))
				entriesToSkip = append(entriesToSkip, entry.ID)
			} else {
				slog.Debug(fmt.Sprintf("For target ref %s, found RSL entry '%s' pointing to a commit, '%s', that exists in the target ref. No more commits to skip.", targetRef, entry.ID, entry.TargetID))
				break
			}
		}
		iterator, err = GetParentForEntry(repo, iterator)
		if err != nil {
			if errors.Is(err, ErrRSLEntryNotFound) {
				break
			}
			return err
		}
	}

	if len(entriesToSkip) == 0 {
		return nil
	}

	return NewAnnotationEntry(entriesToSkip, true, "Automated skip of reference entries pointing to non-existent entries").Commit(repo, signCommit)
}

// GetFirstReferenceUpdaterEntryForCommit returns the first reference entry in
// the RSL that either records the commit itself or a descendent of the commit.
// This establishes the first time a commit was seen in the repository,
// irrespective of the ref it was associated with, and we can infer things like
// the active developers who could have signed the commit.
func GetFirstReferenceUpdaterEntryForCommit(repo *gitinterface.Repository, commitID gitinterface.Hash) (ReferenceUpdaterEntry, []*AnnotationEntry, error) {
	// We check entries in pairs. In the initial case, we have the latest entry
	// and its parent. At all times, the parent in the pair is being tested.
	// If the latest entry is a descendant of the target commit, we start
	// checking the parent. The first pair where the parent entry is not
	// descended from the target commit, we return the other entry in the pair.

	firstEntry, firstAnnotations, err := GetLatestReferenceUpdaterEntry(repo, ForNonGittufReference())
	if err != nil {
		if errors.Is(err, ErrRSLEntryNotFound) {
			return nil, nil, ErrNoRecordOfCommit
		}
		return nil, nil, err
	}

	knowsCommit, err := repo.KnowsCommit(firstEntry.GetTargetID(), commitID)
	if err != nil {
		return nil, nil, err
	}
	if !knowsCommit {
		return nil, nil, ErrNoRecordOfCommit
	}

	for {
		iteratorEntry, iteratorAnnotations, err := GetNonGittufParentReferenceUpdaterEntryForEntry(repo, firstEntry)
		if err != nil {
			if errors.Is(err, ErrRSLEntryNotFound) {
				return firstEntry, firstAnnotations, nil
			}
			return nil, nil, err
		}

		knowsCommit, err := repo.KnowsCommit(iteratorEntry.GetTargetID(), commitID)
		if err != nil {
			return nil, nil, err
		}
		if !knowsCommit {
			return firstEntry, firstAnnotations, nil
		}

		firstEntry = iteratorEntry
		firstAnnotations = iteratorAnnotations
	}
}

// GetReferenceUpdaterEntriesInRange returns a list of reference entries between
// the specified range and a map of annotations that refer to each reference
// entry in the range. The annotations map is keyed by the ID of the reference
// entry, with the value being a list of annotations that apply to that
// reference entry.
func GetReferenceUpdaterEntriesInRange(repo *gitinterface.Repository, firstID, lastID gitinterface.Hash) ([]ReferenceUpdaterEntry, map[string][]*AnnotationEntry, error) {
	return GetReferenceUpdaterEntriesInRangeForRef(repo, firstID, lastID, "")
}

// GetReferenceUpdaterEntriesInRangeForRef returns a list of reference entries
// for the ref between the specified range and a map of annotations that refer
// to each reference entry in the range. The annotations map is keyed by the ID
// of the reference entry, with the value being a list of annotations that apply
// to that reference entry.
func GetReferenceUpdaterEntriesInRangeForRef(repo *gitinterface.Repository, firstID, lastID gitinterface.Hash, refName string) ([]ReferenceUpdaterEntry, map[string][]*AnnotationEntry, error) {
	// We have to iterate from latest to get the annotations that refer to the
	// last requested entry
	iterator, err := GetLatestEntry(repo)
	if err != nil {
		return nil, nil, err
	}

	allAnnotations := []*AnnotationEntry{}
	for !iterator.GetID().Equal(lastID) {
		// Until we find the entry corresponding to lastID, we just store
		// annotations
		if annotation, isAnnotation := iterator.(*AnnotationEntry); isAnnotation {
			allAnnotations = append(allAnnotations, annotation)
		}

		parent, err := GetParentForEntry(repo, iterator)
		if err != nil {
			return nil, nil, err
		}
		iterator = parent
	}

	entryStack := []ReferenceUpdaterEntry{}
	inRange := map[string]bool{}
	for !iterator.GetID().Equal(firstID) {
		// Here, all items are relevant until the one corresponding to first is
		// found
		switch it := iterator.(type) {
		case ReferenceUpdaterEntry:
			if len(refName) == 0 || it.GetRefName() == refName || isRelevantGittufRef(it.GetRefName()) {
				// It's a relevant entry if:
				// a) there's no refName set, or
				// b) the entry's refName matches the set refName, or
				// c) the entry is for a gittuf namespace
				entryStack = append(entryStack, it)
				inRange[it.GetID().String()] = true
			}
		case *AnnotationEntry:
			allAnnotations = append(allAnnotations, it)
		}

		parent, err := GetParentForEntry(repo, iterator)
		if err != nil {
			return nil, nil, err
		}
		iterator = parent
	}

	// Handle the item corresponding to first explicitly
	// If it's an annotation, ignore it as it refers to something before the
	// range we care about
	if entry, isEntry := iterator.(ReferenceUpdaterEntry); isEntry {
		if len(refName) == 0 || entry.GetRefName() == refName || isRelevantGittufRef(entry.GetRefName()) {
			// It's a relevant entry if:
			// a) there's no refName set, or
			// b) the entry's refName matches the set refName, or
			// c) the entry is for a gittuf namespace
			entryStack = append(entryStack, entry)
			inRange[entry.GetID().String()] = true
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

// PropagateChangesFromUpstreamRepository executes gittuf's propagation workflow
// to create a subtree of the contents of an upstream repository's reference
// into the specified reference and path in the downstream repository.
func PropagateChangesFromUpstreamRepository(downstreamRepo, upstreamRepo *gitinterface.Repository, details []tuf.PropagationDirective, sign bool) error {
	// FIXME: We assume here that downstreamRepo and upstreamRepo have their
	// gittuf refs already synced.

	for _, detail := range details {
		latestUpstreamEntry, _, err := GetLatestReferenceUpdaterEntry(upstreamRepo, ForReference(detail.GetUpstreamReference()), IsUnskipped())
		if err != nil {
			if !errors.Is(err, ErrRSLEntryNotFound) {
				return err
			}

			continue
		}

		// We want to check if propagation is necessary
		// What if it's already been propagated?

		// TODO: handle divergence from latest RSL entry for ref downstream?
		currentRefTip, err := downstreamRepo.GetReference(detail.GetDownstreamReference())
		if err != nil {
			return err // TODO: should we handle this differently?
		}

		currentTreeID, err := downstreamRepo.GetCommitTreeID(currentRefTip)
		if err != nil {
			return err // TODO: should we handle this differently?
		}

		currentPathTreeID, err := downstreamRepo.GetPathIDInTree(detail.GetDownstreamPath(), currentTreeID)
		if err != nil {
			if !errors.Is(err, gitinterface.ErrTreeDoesNotHavePath) {
				return err
			}
		}

		upstreamTreeID, err := upstreamRepo.GetCommitTreeID(latestUpstreamEntry.GetTargetID())
		if err != nil {
			return err
		}

		if !currentPathTreeID.IsZero() && currentPathTreeID.Equal(upstreamTreeID) {
			// Nothing to do
			continue
		}

		commitID, err := downstreamRepo.CreateSubtreeFromUpstreamRepository(upstreamRepo, latestUpstreamEntry.GetTargetID(), detail.GetUpstreamPath(), detail.GetDownstreamReference(), detail.GetDownstreamPath())
		if err != nil {
			return err
		}

		if err := NewPropagationEntry(detail.GetDownstreamReference(), commitID, detail.GetUpstreamRepository(), latestUpstreamEntry.GetID()).Commit(downstreamRepo, sign); err != nil {
			return err
		}

		// TODO: error management should revert propagation entries?
		// atomicity?
	}

	return nil
}

func parseRSLEntryText(id gitinterface.Hash, text string) (Entry, error) {
	switch {
	case strings.HasPrefix(text, ReferenceEntryHeader):
		return parseReferenceEntryText(id, text)
	case strings.HasPrefix(text, AnnotationEntryHeader):
		return parseAnnotationEntryText(id, text)
	case strings.HasPrefix(text, PropagationEntryHeader):
		return parsePropagationEntryText(id, text)
	default:
		return nil, ErrInvalidRSLEntry
	}
}

func parseReferenceEntryText(id gitinterface.Hash, text string) (*ReferenceEntry, error) {
	lines := strings.Split(text, "\n")
	if len(lines) < 4 {
		return nil, ErrInvalidRSLEntry
	}
	lines = lines[2:]

	entry := &ReferenceEntry{ID: id}
	for _, l := range lines {
		l = strings.TrimSpace(l)

		ls := strings.Split(l, ":")
		if len(ls) < 2 {
			return nil, ErrInvalidRSLEntry
		}

		switch strings.TrimSpace(ls[0]) {
		case RefKey:
			entry.RefName = strings.TrimSpace(ls[1])

		case TargetIDKey:
			targetHash, err := gitinterface.NewHash(strings.TrimSpace(ls[1]))
			if err != nil {
				return nil, err
			}

			entry.TargetID = targetHash

		case NumberKey:
			number, err := strconv.ParseUint(strings.TrimSpace(ls[1]), 10, 64)
			if err != nil {
				return nil, err
			}

			entry.Number = number
		}
	}

	return entry, nil
}

func parseAnnotationEntryText(id gitinterface.Hash, text string) (*AnnotationEntry, error) {
	annotation := &AnnotationEntry{
		ID:          id,
		RSLEntryIDs: []gitinterface.Hash{},
	}

	messageBlock, _ := pem.Decode([]byte(text)) // rest doesn't seem to work when the PEM block is at the end of text, see: https://go.dev/play/p/oZysAfemA-v
	if messageBlock != nil {
		annotation.Message = string(messageBlock.Bytes)
	}

	lines := strings.Split(text, "\n")
	if len(lines) < 4 {
		return nil, ErrInvalidRSLEntry
	}
	lines = lines[2:]

	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == BeginMessage {
			break
		}

		ls := strings.Split(l, ":")
		if len(ls) < 2 {
			return nil, ErrInvalidRSLEntry
		}

		switch strings.TrimSpace(ls[0]) {
		case EntryIDKey:
			hash, err := gitinterface.NewHash(strings.TrimSpace(ls[1]))
			if err != nil {
				return nil, err
			}

			annotation.RSLEntryIDs = append(annotation.RSLEntryIDs, hash)

		case SkipKey:
			if strings.TrimSpace(ls[1]) == "true" {
				annotation.Skip = true
			} else {
				annotation.Skip = false
			}

		case NumberKey:
			number, err := strconv.ParseUint(strings.TrimSpace(ls[1]), 10, 64)
			if err != nil {
				return nil, err
			}

			annotation.Number = number
		}
	}

	return annotation, nil
}

func parsePropagationEntryText(id gitinterface.Hash, text string) (*PropagationEntry, error) {
	lines := strings.Split(text, "\n")
	if len(lines) < 6 {
		return nil, ErrInvalidRSLEntry
	}
	lines = lines[2:]

	entry := &PropagationEntry{ID: id}
	for _, l := range lines {
		l = strings.TrimSpace(l)

		ls := strings.Split(l, ":")
		if len(ls) < 2 {
			return nil, ErrInvalidRSLEntry
		}

		switch strings.TrimSpace(ls[0]) {
		case RefKey:
			entry.RefName = strings.TrimSpace(ls[1])

		case TargetIDKey:
			targetHash, err := gitinterface.NewHash(strings.TrimSpace(ls[1]))
			if err != nil {
				return nil, err
			}

			entry.TargetID = targetHash

		case UpstreamRepositoryKey:
			// The location may also have `:`, so we need to handle all items in ls
			entry.UpstreamRepository = strings.TrimSpace(strings.Join(ls[1:], ":"))

		case UpstreamEntryIDKey:
			upstreamEntryIDHash, err := gitinterface.NewHash(strings.TrimSpace(ls[1]))
			if err != nil {
				return nil, err
			}

			entry.UpstreamEntryID = upstreamEntryIDHash

		case NumberKey:
			number, err := strconv.ParseUint(strings.TrimSpace(ls[1]), 10, 64)
			if err != nil {
				return nil, err
			}

			entry.Number = number
		}
	}

	return entry, nil
}

func filterAnnotationsForRelevantAnnotations(allAnnotations []*AnnotationEntry, entryID gitinterface.Hash) []*AnnotationEntry {
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
