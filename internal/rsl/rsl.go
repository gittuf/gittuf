// SPDX-License-Identifier: Apache-2.0

package rsl

import (
	"encoding/pem"
	"errors"
	"fmt"
	"strings"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

const (
	Ref                        = "refs/gittuf/reference-state-log"
	ReferenceEntryHeader       = "RSL Reference Entry"
	RefKey                     = "ref"
	TargetIDKey                = "targetID"
	AnnotationEntryHeader      = "RSL Annotation Entry"
	AnnotationMessageBlockType = "MESSAGE"
	BeginMessage               = "-----BEGIN MESSAGE-----"
	EndMessage                 = "-----END MESSAGE-----"
	EntryIDKey                 = "entryID"
	SkipKey                    = "skip"

	remoteTrackerRef       = "refs/remotes/%s/gittuf/reference-state-log"
	gittufNamespacePrefix  = "refs/gittuf/"
	gittufPolicyStagingRef = "refs/gittuf/policy-staging"
)

var (
	ErrRSLExists               = errors.New("cannot initialize RSL namespace as it exists already")
	ErrRSLEntryNotFound        = errors.New("unable to find RSL entry")
	ErrRSLBranchDetected       = errors.New("potential RSL branch detected, entry has more than one parent")
	ErrInvalidRSLEntry         = errors.New("RSL entry has invalid format or is of unexpected type")
	ErrRSLEntryDoesNotMatchRef = errors.New("RSL entry does not match requested ref")
	ErrNoRecordOfCommit        = errors.New("commit has not been encountered before")
)

// InitializeNamespace creates a git ref for the reference state log. Initially,
// the entry has a zero hash.
func InitializeNamespace(repo *git.Repository) error {
	if ref, err := repo.Reference(plumbing.ReferenceName(Ref), true); err != nil {
		if !errors.Is(err, plumbing.ErrReferenceNotFound) {
			return err
		}
	} else if !ref.Hash().IsZero() {
		return ErrRSLExists
	}

	// Write empty tree into the object store as all RSL commits use that for
	// the tree hash. If the tree isn't in the object store, syncs will fail.
	if _, err := gitinterface.WriteTree(repo, nil); err != nil {
		return err
	}

	return repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(Ref), plumbing.ZeroHash))
}

// RemoteTrackerRef returns the remote tracking ref for the specified remote
// name. For example, for 'origin', the remote tracker ref is
// 'refs/remotes/origin/gittuf/reference-state-log'.
func RemoteTrackerRef(remote string) string {
	return fmt.Sprintf(remoteTrackerRef, remote)
}

// Entry is the abstract representation of an object in the RSL.
type Entry interface {
	GetID() plumbing.Hash
	Commit(*git.Repository, bool) error
	createCommitMessage() (string, error)
}

// ReferenceEntry represents a record of a reference state in the RSL. It
// implements the Entry interface.
type ReferenceEntry struct {
	// ID contains the Git hash for the commit corresponding to the entry.
	ID plumbing.Hash

	// RefName contains the Git reference the entry is for.
	RefName string

	// TargetID contains the Git hash for the object expected at RefName.
	TargetID plumbing.Hash
}

// NewReferenceEntry returns a ReferenceEntry object for a normal RSL entry.
func NewReferenceEntry(refName string, targetID plumbing.Hash) *ReferenceEntry {
	return &ReferenceEntry{RefName: refName, TargetID: targetID}
}

func (e *ReferenceEntry) GetID() plumbing.Hash {
	return e.ID
}

// Commit creates a commit object in the RSL for the ReferenceEntry.
func (e *ReferenceEntry) Commit(repo *git.Repository, sign bool) error {
	message, _ := e.createCommitMessage() // we have an error return for annotations, always nil here

	_, err := gitinterface.Commit(repo, gitinterface.EmptyTree(), Ref, message, sign)
	return err
}

// CommitUsingSpecificKey creates a commit object in the RSL for the
// ReferenceEmpty. The commit is signed using the provided PEM encoded SSH or
// GPG private key. This is only intended for use in gittuf's developer mode.
func (e *ReferenceEntry) CommitUsingSpecificKey(repo *git.Repository, signingKeyBytes []byte) error {
	message, _ := e.createCommitMessage() // we have an error return for annotations, always nil here

	_, err := gitinterface.CommitUsingSpecificKey(repo, gitinterface.EmptyTree(), Ref, message, signingKeyBytes)
	return err
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

func (e *ReferenceEntry) createCommitMessage() (string, error) {
	lines := []string{
		ReferenceEntryHeader,
		"",
		fmt.Sprintf("%s: %s", RefKey, e.RefName),
		fmt.Sprintf("%s: %s", TargetIDKey, e.TargetID.String()),
	}
	return strings.Join(lines, "\n"), nil
}

// AnnotationEntry is a type of RSL record that references prior items in the
// RSL. It can be used to add extra information for the referenced items.
// Annotations can also be used to "skip", i.e. revoke, the referenced items. It
// implements the Entry interface.
type AnnotationEntry struct {
	// ID contains the Git hash for the commit corresponding to the annotation.
	ID plumbing.Hash

	// RSLEntryIDs contains one or more Git hashes for the RSL entries the annotation applies to.
	RSLEntryIDs []plumbing.Hash

	// Skip indicates if the RSLEntryIDs must be skipped during gittuf workflows.
	Skip bool

	// Message contains any messages or notes added by a user for the annotation.
	Message string
}

// NewAnnotationEntry returns an Annotation object that applies to one or more
// prior RSL entries.
func NewAnnotationEntry(rslEntryIDs []plumbing.Hash, skip bool, message string) *AnnotationEntry {
	return &AnnotationEntry{RSLEntryIDs: rslEntryIDs, Skip: skip, Message: message}
}

func (a *AnnotationEntry) GetID() plumbing.Hash {
	return a.ID
}

// Commit creates a commit object in the RSL for the Annotation.
func (a *AnnotationEntry) Commit(repo *git.Repository, sign bool) error {
	// Check if referred entries exist in the RSL namespace.
	for _, id := range a.RSLEntryIDs {
		if _, err := GetEntry(repo, id); err != nil {
			return err
		}
	}

	message, err := a.createCommitMessage()
	if err != nil {
		return err
	}

	_, err = gitinterface.Commit(repo, gitinterface.EmptyTree(), Ref, message, sign)
	return err
}

// RefersTo returns true if the specified entryID is referred to by the
// annotation.
func (a *AnnotationEntry) RefersTo(entryID plumbing.Hash) bool {
	for _, id := range a.RSLEntryIDs {
		if id == entryID {
			return true
		}
	}

	return false
}

func (a *AnnotationEntry) createCommitMessage() (string, error) {
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

// GetEntry returns the entry corresponding to entryID.
func GetEntry(repo *git.Repository, entryID plumbing.Hash) (Entry, error) {
	commitObj, err := gitinterface.GetCommit(repo, entryID)
	if err != nil {
		return nil, ErrRSLEntryNotFound
	}

	return parseRSLEntryText(entryID, commitObj.Message)
}

// GetParentForEntry returns the entry's parent RSL entry.
func GetParentForEntry(repo *git.Repository, entry Entry) (Entry, error) {
	commitObj, err := gitinterface.GetCommit(repo, entry.GetID())
	if err != nil {
		return nil, err
	}

	if len(commitObj.ParentHashes) == 0 {
		return nil, ErrRSLEntryNotFound
	}

	if len(commitObj.ParentHashes) > 1 {
		return nil, ErrRSLBranchDetected
	}

	return GetEntry(repo, commitObj.ParentHashes[0])
}

// GetNonGittufParentReferenceEntryForEntry returns the first RSL reference
// entry starting from the specified entry's parent that is not for the gittuf
// namespace.
func GetNonGittufParentReferenceEntryForEntry(repo *git.Repository, entry Entry) (*ReferenceEntry, []*AnnotationEntry, error) {
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

		if it.GetID() == parentEntry.GetID() {
			break
		}
	}

	var targetEntry *ReferenceEntry
	for {
		switch iterator := it.(type) {
		case *ReferenceEntry:
			if !strings.HasPrefix(iterator.RefName, gittufNamespacePrefix) {
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

	annotations := filterAnnotationsForRelevantAnnotations(allAnnotations, targetEntry.ID)

	return targetEntry, annotations, nil
}

// GetLatestEntry returns the latest entry available locally in the RSL.
func GetLatestEntry(repo *git.Repository) (Entry, error) {
	ref, err := repo.Reference(plumbing.ReferenceName(Ref), true)
	if err != nil {
		return nil, err
	}

	commitObj, err := gitinterface.GetCommit(repo, ref.Hash())
	if err != nil {
		return nil, ErrRSLEntryNotFound
	}

	return parseRSLEntryText(commitObj.Hash, commitObj.Message)
}

// GetLatestNonGittufReferenceEntry returns the first reference entry that is
// not for the gittuf namespace.
func GetLatestNonGittufReferenceEntry(repo *git.Repository) (*ReferenceEntry, []*AnnotationEntry, error) {
	it, err := GetLatestEntry(repo)
	if err != nil {
		return nil, nil, err
	}

	allAnnotations := []*AnnotationEntry{}
	var targetEntry *ReferenceEntry

	for {
		switch iterator := it.(type) {
		case *ReferenceEntry:
			if !strings.HasPrefix(iterator.RefName, gittufNamespacePrefix) {
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

	annotations := filterAnnotationsForRelevantAnnotations(allAnnotations, targetEntry.ID)

	return targetEntry, annotations, nil
}

// GetLatestReferenceEntryForRef returns the latest reference entry available
// locally in the RSL for the specified refName.
func GetLatestReferenceEntryForRef(repo *git.Repository, refName string) (*ReferenceEntry, []*AnnotationEntry, error) {
	return GetLatestReferenceEntryForRefBefore(repo, refName, plumbing.ZeroHash)
}

// GetLatestReferenceEntryForRefBefore returns the latest reference entry
// available locally in the RSL for the specified refName before the specified
// anchor.
func GetLatestReferenceEntryForRefBefore(repo *git.Repository, refName string, anchor plumbing.Hash) (*ReferenceEntry, []*AnnotationEntry, error) {
	allAnnotations := []*AnnotationEntry{}

	iteratorT, err := GetLatestEntry(repo)
	if err != nil {
		return nil, nil, err
	}

	if !anchor.IsZero() {
		for iteratorT.GetID() != anchor {
			if annotation, isAnnotation := iteratorT.(*AnnotationEntry); isAnnotation {
				allAnnotations = append(allAnnotations, annotation)
			}

			iteratorT, err = GetParentForEntry(repo, iteratorT)
			if err != nil {
				return nil, nil, err
			}
		}

		// If the anchor is an annotation, track that
		if annotation, isAnnotation := iteratorT.(*AnnotationEntry); isAnnotation {
			allAnnotations = append(allAnnotations, annotation)
		}

		// We have to set the iterator to the parent. The other option is to
		// swap the refName check and parent in the loop below but that breaks
		// GetLatestReferenceEntryForRef's behavior. By adding this one extra
		// GetParent here, we avoid repetition.
		iteratorT, err = GetParentForEntry(repo, iteratorT)
		if err != nil {
			return nil, nil, err
		}
	}

	var targetEntry *ReferenceEntry
	for {
		switch iterator := iteratorT.(type) {
		case *ReferenceEntry:
			if iterator.RefName == refName {
				targetEntry = iterator
			}
		case *AnnotationEntry:
			allAnnotations = append(allAnnotations, iterator)
		}

		if targetEntry != nil {
			// we've found the target entry, stop walking the RSL
			break
		}

		iteratorT, err = GetParentForEntry(repo, iteratorT)
		if err != nil {
			return nil, nil, err
		}
	}

	annotations := filterAnnotationsForRelevantAnnotations(allAnnotations, targetEntry.ID)

	return targetEntry, annotations, nil
}

// GetLatestUnskippedReferenceEntryForRef returns the latest reference entry for
// the ref that does not have an annotation marking it as to-be-skipped. Entries
// are searched from the latest entry in the RSL to include new annotations for
// each reference entry tested for the ref.
func GetLatestUnskippedReferenceEntryForRef(repo *git.Repository, refName string) (*ReferenceEntry, []*AnnotationEntry, error) {
	return GetLatestUnskippedReferenceEntryForRefBefore(repo, refName, plumbing.ZeroHash)
}

// GetLatestUnskippedReferenceEntryForRefBefore returns the first reference
// entry for the ref before the anchor that does not have an annotation marking
// it as to-be-skipped. Entries are searched from the latest entry in the RSL to
// include new annotations for each reference entry tested for the ref. The only
// reference entries for the ref considered are those that occur strictly before
// the anchor entry in the RSL. Of these, the latest reference entry that is not
// skipped by an annotation (before or after the anchor) is returned.
func GetLatestUnskippedReferenceEntryForRefBefore(repo *git.Repository, refName string, anchor plumbing.Hash) (*ReferenceEntry, []*AnnotationEntry, error) {
	for {
		latestEntry, annotations, err := GetLatestReferenceEntryForRefBefore(repo, refName, anchor)
		if err != nil {
			return nil, nil, err
		}

		if !latestEntry.SkippedBy(annotations) {
			return latestEntry, annotations, nil
		}

		anchor = latestEntry.ID
	}
}

// GetFirstEntry returns the very first entry in the RSL. It is expected to be
// a reference entry as the first entry in the RSL cannot be an annotation.
func GetFirstEntry(repo *git.Repository) (*ReferenceEntry, []*AnnotationEntry, error) {
	iteratorT, err := GetLatestEntry(repo)
	if err != nil {
		return nil, nil, err
	}

	allAnnotations := []*AnnotationEntry{}
	var firstEntry *ReferenceEntry

	if iterator, ok := iteratorT.(*AnnotationEntry); ok {
		allAnnotations = append(allAnnotations, iterator)
	}

	for {
		parentT, err := GetParentForEntry(repo, iteratorT)
		if err != nil {
			if errors.Is(err, ErrRSLEntryNotFound) {
				entry, ok := iteratorT.(*ReferenceEntry)
				if !ok {
					// The first entry cannot be an annotation
					return nil, nil, ErrInvalidRSLEntry
				}
				firstEntry = entry
				break
			}

			return nil, nil, err
		}

		if annotation, ok := parentT.(*AnnotationEntry); ok {
			allAnnotations = append(allAnnotations, annotation)
		}

		iteratorT = parentT
	}

	annotations := filterAnnotationsForRelevantAnnotations(allAnnotations, firstEntry.ID)

	return firstEntry, annotations, nil
}

// GetFirstReferenceEntryForCommit returns the first reference entry in the RSL
// that either records the commit itself or a descendent of the commit. This
// establishes the first time a commit was seen in the repository, irrespective
// of the ref it was associated with, and we can infer things like the active
// developers who could have signed the commit.
func GetFirstReferenceEntryForCommit(repo *git.Repository, commit *object.Commit) (*ReferenceEntry, []*AnnotationEntry, error) {
	// We check entries in pairs. In the initial case, we have the latest entry
	// and its parent. At all times, the parent in the pair is being tested.
	// If the latest entry is a descendant of the target commit, we start
	// checking the parent. The first pair where the parent entry is not
	// descended from the target commit, we return the other entry in the pair.

	firstEntry, firstAnnotations, err := GetLatestNonGittufReferenceEntry(repo)
	if err != nil {
		if errors.Is(err, ErrRSLEntryNotFound) {
			return nil, nil, ErrNoRecordOfCommit
		}
		return nil, nil, err
	}

	knowsCommit, err := gitinterface.KnowsCommit(repo, firstEntry.TargetID, commit)
	if err != nil {
		return nil, nil, err
	}
	if !knowsCommit {
		return nil, nil, ErrNoRecordOfCommit
	}

	for {
		iteratorEntry, iteratorAnnotations, err := GetNonGittufParentReferenceEntryForEntry(repo, firstEntry)
		if err != nil {
			if errors.Is(err, ErrRSLEntryNotFound) {
				return firstEntry, firstAnnotations, nil
			}
			return nil, nil, err
		}

		knowsCommit, err := gitinterface.KnowsCommit(repo, iteratorEntry.TargetID, commit)
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

// GetReferenceEntriesInRange returns a list of reference entries between the
// specified range and a map of annotations that refer to each reference entry
// in the range. The annotations map is keyed by the ID of the reference entry,
// with the value being a list of annotations that apply to that reference
// entry.
func GetReferenceEntriesInRange(repo *git.Repository, firstID, lastID plumbing.Hash) ([]*ReferenceEntry, map[plumbing.Hash][]*AnnotationEntry, error) {
	return GetReferenceEntriesInRangeForRef(repo, firstID, lastID, "")
}

// GetReferenceEntriesInRangeForRef returns a list of reference entries for the
// ref between the specified range and a map of annotations that refer to each
// reference entry in the range. The annotations map is keyed by the ID of the
// reference entry, with the value being a list of annotations that apply to
// that reference entry.
func GetReferenceEntriesInRangeForRef(repo *git.Repository, firstID, lastID plumbing.Hash, refName string) ([]*ReferenceEntry, map[plumbing.Hash][]*AnnotationEntry, error) {
	// We have to iterate from latest to get the annotations that refer to the
	// last requested entry
	iterator, err := GetLatestEntry(repo)
	if err != nil {
		return nil, nil, err
	}

	allAnnotations := []*AnnotationEntry{}
	for iterator.GetID() != lastID {
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

	entryStack := []*ReferenceEntry{}
	inRange := map[plumbing.Hash]bool{}
	for iterator.GetID() != firstID {
		// Here, all items are relevant until the one corresponding to first is
		// found
		switch it := iterator.(type) {
		case *ReferenceEntry:
			if len(refName) == 0 || it.RefName == refName || isRelevantGittufRef(it.RefName) {
				// It's a relevant entry if:
				// a) there's no refName set, or
				// b) the entry's refName matches the set refName, or
				// c) the entry is for a gittuf namespace
				entryStack = append(entryStack, it)
				inRange[it.ID] = true
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
	if entry, isEntry := iterator.(*ReferenceEntry); isEntry {
		if len(refName) == 0 || entry.RefName == refName || isRelevantGittufRef(entry.RefName) {
			// It's a relevant entry if:
			// a) there's no refName set, or
			// b) the entry's refName matches the set refName, or
			// c) the entry is for a gittuf namespace
			entryStack = append(entryStack, entry)
			inRange[entry.ID] = true
		}
	}

	// For each annotation, add the entry to each relevant entry it refers to
	// Process annotations in reverse order so that annotations are listed in
	// order of occurrence in the map
	annotationMap := map[plumbing.Hash][]*AnnotationEntry{}
	for i := len(allAnnotations) - 1; i >= 0; i-- {
		annotation := allAnnotations[i]
		for _, entryID := range annotation.RSLEntryIDs {
			if _, relevant := inRange[entryID]; relevant {
				// Annotation is relevant because the entry it refers to was in
				// the specified range
				if _, exists := annotationMap[entryID]; !exists {
					annotationMap[entryID] = []*AnnotationEntry{}
				}

				annotationMap[entryID] = append(annotationMap[entryID], annotation)
			}
		}
	}

	// Reverse entryStack so that it's in order of occurrence rather than in
	// order of walking back the RSL
	allEntries := make([]*ReferenceEntry, 0, len(entryStack))
	for i := len(entryStack) - 1; i >= 0; i-- {
		allEntries = append(allEntries, entryStack[i])
	}

	return allEntries, annotationMap, nil
}

func parseRSLEntryText(id plumbing.Hash, text string) (Entry, error) {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, AnnotationEntryHeader) {
		return parseAnnotationEntryText(id, text)
	}
	return parseReferenceEntryText(id, text)
}

func parseReferenceEntryText(id plumbing.Hash, text string) (*ReferenceEntry, error) {
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
			entry.TargetID = plumbing.NewHash(strings.TrimSpace(ls[1]))
		}
	}

	return entry, nil
}

func parseAnnotationEntryText(id plumbing.Hash, text string) (*AnnotationEntry, error) {
	annotation := &AnnotationEntry{
		ID:          id,
		RSLEntryIDs: []plumbing.Hash{},
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
			annotation.RSLEntryIDs = append(annotation.RSLEntryIDs, plumbing.NewHash(strings.TrimSpace(ls[1])))
		case SkipKey:
			if strings.TrimSpace(ls[1]) == "true" {
				annotation.Skip = true
			} else {
				annotation.Skip = false
			}
		}
	}

	return annotation, nil
}

func filterAnnotationsForRelevantAnnotations(allAnnotations []*AnnotationEntry, entryID plumbing.Hash) []*AnnotationEntry {
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
