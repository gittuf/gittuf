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
	GittufNamespacePrefix      = "refs/gittuf/"
	RSLRef                     = "refs/gittuf/reference-state-log"
	EntryHeader                = "RSL Entry"
	RefKey                     = "ref"
	CommitIDKey                = "commitID"
	AnnotationHeader           = "RSL Annotation"
	AnnotationMessageBlockType = "MESSAGE"
	BeginMessage               = "-----BEGIN MESSAGE-----"
	EndMessage                 = "-----END MESSAGE-----"
	EntryIDKey                 = "entryID"
	SkipKey                    = "skip"
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
	if _, err := repo.Reference(plumbing.ReferenceName(RSLRef), true); err != nil {
		if !errors.Is(err, plumbing.ErrReferenceNotFound) {
			return err
		}
	} else {
		return ErrRSLExists
	}

	return repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(RSLRef), plumbing.ZeroHash))
}

type EntryType interface {
	GetID() plumbing.Hash
	Commit(*git.Repository, bool) error
	createCommitMessage() (string, error)
}

type Entry struct {
	// ID contains the Git hash for the commit corresponding to the entry.
	ID plumbing.Hash

	// RefName contains the Git reference the entry is for.
	RefName string

	// CommitID contains the Git hash for the object expected at RefName.
	CommitID plumbing.Hash
}

// NewEntry returns an Entry object for a normal RSL entry.
func NewEntry(refName string, commitID plumbing.Hash) *Entry {
	return &Entry{RefName: refName, CommitID: commitID}
}

func (e Entry) GetID() plumbing.Hash {
	return e.ID
}

// Commit creates a commit object in the RSL for the Entry.
func (e Entry) Commit(repo *git.Repository, sign bool) error {
	message, _ := e.createCommitMessage() // we have an error return for annotations, always nil here

	return gitinterface.Commit(repo, gitinterface.EmptyTree(), RSLRef, message, sign)
}

func (e Entry) createCommitMessage() (string, error) {
	lines := []string{
		EntryHeader,
		"",
		fmt.Sprintf("%s: %s", RefKey, e.RefName),
		fmt.Sprintf("%s: %s", CommitIDKey, e.CommitID.String()),
	}
	return strings.Join(lines, "\n"), nil
}

type Annotation struct {
	// ID contains the Git hash for the commit corresponding to the annotation.
	ID plumbing.Hash

	// RSLEntryIDs contains one or more Git hashes for the RSL entries the annotation applies to.
	RSLEntryIDs []plumbing.Hash

	// Skip indicates if the RSLEntryIDs must be skipped during gittuf workflows.
	Skip bool

	// Message contains any messages or notes added by a user for the annotation.
	Message string
}

// NewAnnotation returns an Annotation object that applies to one or more prior
// RSL entries.
func NewAnnotation(rslEntryIDs []plumbing.Hash, skip bool, message string) *Annotation {
	return &Annotation{RSLEntryIDs: rslEntryIDs, Skip: skip, Message: message}
}

func (a Annotation) GetID() plumbing.Hash {
	return a.ID
}

// Commit creates a commit object in the RSL for the Annotation.
func (a Annotation) Commit(repo *git.Repository, sign bool) error {
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

	return gitinterface.Commit(repo, gitinterface.EmptyTree(), RSLRef, message, sign)
}

func (a Annotation) createCommitMessage() (string, error) {
	lines := []string{
		AnnotationHeader,
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
// TODO: There is no information yet about the signature for the entry.
func GetEntry(repo *git.Repository, entryID plumbing.Hash) (EntryType, error) {
	commitObj, err := repo.CommitObject(entryID)
	if err != nil {
		return nil, ErrRSLEntryNotFound
	}

	return parseRSLEntryText(entryID, commitObj.Message)
}

// GetParentForEntry returns the entry's parent RSL entry.
// TODO: There is no information yet about the signature for the parent entry.
func GetParentForEntry(repo *git.Repository, entry EntryType) (EntryType, error) {
	commitObj, err := repo.CommitObject(entry.GetID())
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

// GetNonGittufParentForEntry returns the first RSL entry starting from the
// specified entry's parent that is not for the gittuf namespace.
// TODO: There is no information yet about the signature for the entry.
func GetNonGittufParentForEntry(repo *git.Repository, entry EntryType) (*Entry, error) {
	it, err := GetParentForEntry(repo, entry)
	if err != nil {
		return nil, err
	}

	for {
		if e, ok := it.(*Entry); ok {
			if !strings.HasPrefix(e.RefName, GittufNamespacePrefix) {
				return e, nil
			}
		} // TODO: handle annotations

		it, err = GetParentForEntry(repo, it)
		if err != nil {
			return nil, err
		}
	}
}

// GetLatestEntry returns the latest entry available locally in the RSL.
// TODO: There is no information yet about the signature for the entry.
func GetLatestEntry(repo *git.Repository) (EntryType, error) {
	ref, err := repo.Reference(plumbing.ReferenceName(RSLRef), true)
	if err != nil {
		return nil, err
	}

	commitObj, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, ErrRSLEntryNotFound
	}

	return parseRSLEntryText(commitObj.Hash, commitObj.Message)
}

// GetLatestNonGittufEntry returns the first RSL entry that is not for the
// gittuf namespace.
// TODO: There is no information yet about the signature for the entry.
func GetLatestNonGittufEntry(repo *git.Repository) (*Entry, error) {
	it, err := GetLatestEntry(repo)
	if err != nil {
		return nil, err
	}

	for {
		if entry, ok := it.(*Entry); ok {
			if !strings.HasPrefix(entry.RefName, GittufNamespacePrefix) {
				return entry, nil
			}
		} // TODO: handle annotations

		it, err = GetParentForEntry(repo, it)
		if err != nil {
			return nil, err
		}
	}
}

// GetLatestEntryForRef returns the latest entry available locally in the RSL
// for the specified refName.
// TODO: There is no information yet about the signature for the entry.
func GetLatestEntryForRef(repo *git.Repository, refName string) (*Entry, error) {
	return GetLatestEntryForRefBefore(repo, refName, plumbing.ZeroHash)
}

// GetLatestEntryForRefBefore returns the latest entry available locally in the
// RSL for the specified refName before the specified anchor.
// TODO: There is no information yet about the signature for the entry.
func GetLatestEntryForRefBefore(repo *git.Repository, refName string, anchor plumbing.Hash) (*Entry, error) {
	var (
		iteratorT EntryType
		err       error
	)

	if anchor.IsZero() {
		iteratorT, err = GetLatestEntry(repo)
		if err != nil {
			return nil, err
		}
	} else {
		iteratorT, err = GetEntry(repo, anchor)
		if err != nil {
			return nil, err
		}

		// We have to set the iterator to the parent. The other option is to
		// swap the refName check and parent in the loop below but that breaks
		// GetLatestEntryForRef's behavior. By adding this one extra GetParent
		// here, we avoid repetition.
		iteratorT, err = GetParentForEntry(repo, iteratorT)
		if err != nil {
			return nil, err
		}
	}

	for {
		switch iterator := iteratorT.(type) {
		case *Entry:
			if iterator.RefName == refName {
				return iterator, nil
			}
		case *Annotation:
			// TODO: discuss if annotations should be checked to see if they
			// requested entry.
		}

		iteratorT, err = GetParentForEntry(repo, iteratorT)
		if err != nil {
			return nil, err
		}

	}
}

// GetFirstEntry returns the very first entry in the RSL. It is expected to be
// *Entry as the first entry in the RSL cannot be an annotation.
// TODO: There is no information yet about the signature for the entry.
func GetFirstEntry(repo *git.Repository) (*Entry, error) {
	iteratorT, err := GetLatestEntry(repo)
	if err != nil {
		return nil, err
	}

	for {
		parentT, err := GetParentForEntry(repo, iteratorT)
		if err != nil {
			if errors.Is(err, ErrRSLEntryNotFound) {
				entry, ok := iteratorT.(*Entry)
				if !ok {
					return nil, ErrInvalidRSLEntry
				}
				return entry, nil
			}
			return nil, err
		}
		iteratorT = parentT
	}
}

// GetFirstEntryForCommit returns the first entry in the RSL that either records
// the commit itself or a descendent of the commit. This establishes the first
// time a commit was seen in the repository, irrespective of the ref it was
// associated with, and we can infer things like the active developers who could
// have signed the commit.
// TODO: There is no information yet about the signature for the entry.
// TODO: What if the first seen RSL entry is invalid? Should it be verified
// against policy here? Probably not as this is strictly the RSL library.
func GetFirstEntryForCommit(repo *git.Repository, commit *object.Commit) (*Entry, error) {
	// We check entries in pairs. In the initial case, we have the latest entry
	// and its parent. At all times, the parent in the pair is being tested.
	// If the latest entry is a descendant of the target commit, we start
	// checking the parent. The first pair where the parent entry is not
	// descended from the target commit, we return the other entry in the pair.

	firstEntry, err := GetLatestNonGittufEntry(repo)
	if err != nil {
		if errors.Is(err, ErrRSLEntryNotFound) {
			return nil, ErrNoRecordOfCommit
		}
		return nil, err
	}
	knowsCommit, err := gitinterface.KnowsCommit(repo, firstEntry.CommitID, commit)
	if err != nil {
		return nil, err
	}
	if !knowsCommit {
		return nil, ErrNoRecordOfCommit
	}

	for {
		iteratorEntry, err := GetNonGittufParentForEntry(repo, firstEntry)
		if err != nil {
			if errors.Is(err, ErrRSLEntryNotFound) {
				return firstEntry, nil
			}
			return nil, err
		}

		knowsCommit, err := gitinterface.KnowsCommit(repo, iteratorEntry.CommitID, commit)
		if err != nil {
			return nil, err
		}

		if !knowsCommit {
			return firstEntry, nil
		}

		firstEntry = iteratorEntry
	}
}

func parseRSLEntryText(id plumbing.Hash, text string) (EntryType, error) {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, AnnotationHeader) {
		return parseAnnotationText(id, text)
	}
	return parseEntryText(id, text)
}

func parseEntryText(id plumbing.Hash, text string) (*Entry, error) {
	lines := strings.Split(text, "\n")
	if len(lines) < 4 {
		return nil, ErrInvalidRSLEntry
	}
	lines = lines[2:]

	entry := &Entry{ID: id}
	for _, l := range lines {
		l = strings.TrimSpace(l)

		ls := strings.Split(l, ":")
		if len(ls) < 2 {
			return nil, ErrInvalidRSLEntry
		}

		switch strings.TrimSpace(ls[0]) {
		case RefKey:
			entry.RefName = strings.TrimSpace(ls[1])
		case CommitIDKey:
			entry.CommitID = plumbing.NewHash(strings.TrimSpace(ls[1]))
		}
	}

	return entry, nil
}

func parseAnnotationText(id plumbing.Hash, text string) (*Annotation, error) {
	annotation := &Annotation{
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
