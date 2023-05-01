package rsl

import (
	"encoding/pem"
	"errors"
	"fmt"
	"strings"

	"github.com/adityasaky/gittuf/internal/gitinterface"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

const RSLRef = "refs/gittuf/reference-state-log"

const (
	EntryHeader = "RSL Entry"
	RefKey      = "ref"
	CommitIDKey = "commitID"
)
const (
	AnnotationHeader           = "RSL Annotation"
	AnnotationMessageBlockType = "MESSAGE"
	BeginMessage               = "-----BEGIN MESSAGE-----"
	EndMessage                 = "-----END MESSAGE-----"
	EntryIDKey                 = "entryID"
	SkipKey                    = "skip"
)

var (
	ErrRSLExists         = errors.New("cannot initialize RSL namespace as it exists already")
	ErrRSLEntryNotFound  = errors.New("unable to find RSL entry")
	ErrRSLBranchDetected = errors.New("potential RSL branch detected, entry has more than one parent")
	ErrInvalidRSLEntry   = errors.New("RSL entry has invalid format")
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
	Commit(*git.Repository, bool) error
	createCommitMessage() (string, error)
}

type Entry struct {
	RefName  string
	CommitID plumbing.Hash
}

// NewEntry returns an Entry object for a normal RSL entry.
func NewEntry(refName string, commitID plumbing.Hash) *Entry {
	return &Entry{RefName: refName, CommitID: commitID}
}

// Commit creates a commit object in the RSL for the Entry.
func (e Entry) Commit(repo *git.Repository, sign bool) error {
	message, _ := e.createCommitMessage() // we have an error return for annotations, always nil here

	return gitinterface.Commit(repo, plumbing.ZeroHash, RSLRef, message, sign)
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
	RSLEntryIDs []plumbing.Hash
	Skip        bool
	Message     string
}

// NewAnnotation returns an Annotation object that applies to one or more prior
// RSL entries.
func NewAnnotation(rslEntryIDs []plumbing.Hash, skip bool, message string) *Annotation {
	return &Annotation{RSLEntryIDs: rslEntryIDs, Skip: skip, Message: message}
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

	return gitinterface.Commit(repo, plumbing.ZeroHash, RSLRef, message, sign)
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

// GetLatestEntry returns the latest entry available locally in the RSL.
// TODO: There is no information yet about the signature for the entry.
func GetLatestEntry(repo *git.Repository) (EntryType, error) {
	ref, err := repo.Reference(plumbing.ReferenceName(RSLRef), true)
	if err != nil {
		return nil, err
	}

	commitObj, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, err
	}

	return parseRSLEntryText(commitObj.Message)
}

// GetEntry returns the entry corresponding to entryID.
// TODO: There is no information yet about the signature for the entry.
func GetEntry(repo *git.Repository, entryID plumbing.Hash) (EntryType, error) {
	ref, err := repo.Reference(plumbing.ReferenceName(RSLRef), true)
	if err != nil {
		return nil, err
	}

	iteratorHash := ref.Hash()
	for {
		commitObj, err := repo.CommitObject(iteratorHash)
		if err != nil {
			return nil, err
		}

		if iteratorHash == entryID {
			return parseRSLEntryText(commitObj.Message)
		}

		if len(commitObj.ParentHashes) == 0 {
			return nil, ErrRSLEntryNotFound
		}

		if len(commitObj.ParentHashes) > 1 {
			return nil, ErrRSLBranchDetected
		}

		iteratorHash = commitObj.ParentHashes[0]
	}
}

func parseRSLEntryText(text string) (EntryType, error) {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, AnnotationHeader) {
		return parseAnnotationText(text)
	}
	return parseEntryText(text)
}

func parseEntryText(text string) (*Entry, error) {
	lines := strings.Split(text, "\n")
	if len(lines) < 4 {
		return nil, ErrInvalidRSLEntry
	}
	lines = lines[2:]

	entry := &Entry{}
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

func parseAnnotationText(text string) (*Annotation, error) {
	annotation := &Annotation{
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
