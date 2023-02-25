package rsl

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/adityasaky/gittuf/internal/common"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/assert"
)

func TestInitializeNamespace(t *testing.T) {
	testDir, err := common.CreateTestRepository()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(testDir); err != nil {
		t.Fatal(err)
	}

	if err := InitializeNamespace(); err != nil {
		t.Error(err)
	}

	refContents, err := os.ReadFile(filepath.Join(".git", RSLRef))
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, plumbing.ZeroHash, plumbing.NewHash(string(refContents)))
}

func TestNewEntry(t *testing.T) {
	testDir, err := common.CreateTestRepository()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(testDir); err != nil {
		t.Fatal(err)
	}

	if err := InitializeNamespace(); err != nil {
		t.Fatal(err)
	}

	if err := NewEntry("main", plumbing.ZeroHash).Commit(false); err != nil {
		t.Error(err)
	}

	refContents, err := os.ReadFile(filepath.Join(".git", RSLRef))
	if err != nil {
		t.Error(err)
	}
	refHash := plumbing.NewHash(string(refContents))
	assert.NotEqual(t, plumbing.ZeroHash, refHash)

	repo, err := common.GetRepositoryHandler()
	if err != nil {
		t.Error(err)
	}

	commitObj, err := repo.CommitObject(refHash)
	if err != nil {
		t.Error(err)
	}

	expectedMessage := fmt.Sprintf("%s\n\n%s: %s\n%s: %s", EntryHeader, RefKey, "main", CommitIDKey, plumbing.ZeroHash.String())
	assert.Equal(t, expectedMessage, commitObj.Message)

	assert.Empty(t, commitObj.ParentHashes)

	if err := NewEntry("main", plumbing.NewHash("abcdef1234567890")).Commit(false); err != nil {
		t.Error(err)
	}
	refContents, err = os.ReadFile(filepath.Join(".git", RSLRef))
	if err != nil {
		t.Error(err)
	}
	newRefHash := plumbing.NewHash(string(refContents))

	commitObj, err = repo.CommitObject(newRefHash)
	if err != nil {
		t.Error(err)
	}

	expectedMessage = fmt.Sprintf("%s\n\n%s: %s\n%s: %s", EntryHeader, RefKey, "main", CommitIDKey, plumbing.NewHash("abcdef1234567890"))
	assert.Equal(t, expectedMessage, commitObj.Message)

	assert.Contains(t, commitObj.ParentHashes, refHash)
}

func TestGetLatestEntry(t *testing.T) {
	testDir, err := common.CreateTestRepository()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(testDir); err != nil {
		t.Fatal(err)
	}

	if err := InitializeNamespace(); err != nil {
		t.Error(err)
	}

	if err := NewEntry("main", plumbing.ZeroHash).Commit(false); err != nil {
		t.Error(err)
	}

	if entry, err := GetLatestEntry(); err != nil {
		t.Error(err)
	} else {
		e := entry.(*Entry)
		assert.Equal(t, "main", e.RefName)
		assert.Equal(t, plumbing.ZeroHash, e.CommitID)
	}

	if err := NewEntry("feature", plumbing.NewHash("abcdef1234567890")).Commit(false); err != nil {
		t.Error(err)
	}
	if entry, err := GetLatestEntry(); err != nil {
		t.Error(err)
	} else {
		e := entry.(*Entry)
		assert.NotEqual(t, "main", e.RefName)
		assert.NotEqual(t, plumbing.ZeroHash, e.CommitID)
	}

	repo, err := common.GetRepositoryHandler()
	if err != nil {
		t.Fatal(err)
	}

	ref, err := repo.Reference(plumbing.ReferenceName(RSLRef), true)
	if err != nil {
		t.Fatal(err)
	}
	entryID := ref.Hash()

	if err := NewAnnotation([]plumbing.Hash{entryID}, true, "This was a mistaken push!").Commit(false); err != nil {
		t.Error(err)
	}

	if entry, err := GetLatestEntry(); err != nil {
		t.Error(err)
	} else {
		a := entry.(*Annotation)
		assert.True(t, a.Skip)
		assert.Equal(t, []plumbing.Hash{entryID}, a.RSLEntryIDs)
		assert.Equal(t, "This was a mistaken push!", a.Message)
	}
}

func TestGetEntry(t *testing.T) {
	testDir, err := common.CreateTestRepository()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(testDir); err != nil {
		t.Fatal(err)
	}

	if err := InitializeNamespace(); err != nil {
		t.Fatal(err)
	}

	repo, err := common.GetRepositoryHandler()
	if err != nil {
		t.Fatal(err)
	}

	if err := NewEntry("main", plumbing.ZeroHash).Commit(false); err != nil {
		t.Error(err)
	}

	ref, err := repo.Reference(plumbing.ReferenceName(RSLRef), true)
	if err != nil {
		t.Fatal(err)
	}

	initialEntryID := ref.Hash()

	if err := NewAnnotation([]plumbing.Hash{initialEntryID}, true, "This was a mistaken push!").Commit(false); err != nil {
		t.Error(err)
	}

	ref, err = repo.Reference(plumbing.ReferenceName(RSLRef), true)
	if err != nil {
		t.Fatal(err)
	}

	annotationID := ref.Hash()

	if err := NewEntry("main", plumbing.ZeroHash).Commit(false); err != nil {
		t.Error(err)
	}

	if entry, err := GetEntry(initialEntryID); err != nil {
		t.Error(err)
	} else {
		e := entry.(*Entry)
		assert.Equal(t, "main", e.RefName)
		assert.Equal(t, plumbing.ZeroHash, e.CommitID)
	}

	if entry, err := GetEntry(annotationID); err != nil {
		t.Error(err)
	} else {
		a := entry.(*Annotation)
		assert.True(t, a.Skip)
		assert.Equal(t, []plumbing.Hash{initialEntryID}, a.RSLEntryIDs)
		assert.Equal(t, "This was a mistaken push!", a.Message)
	}
}

func TestEntryCreateCommitMessage(t *testing.T) {
	tests := map[string]struct {
		entry           *Entry
		expectedMessage string
	}{
		"entry, fully resolved ref": {
			entry: &Entry{
				RefName:  "refs/heads/main",
				CommitID: plumbing.ZeroHash,
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s", EntryHeader, RefKey, "refs/heads/main", CommitIDKey, plumbing.ZeroHash.String()),
		},
		"entry, non-zero commit": {
			entry: &Entry{
				RefName:  "refs/heads/main",
				CommitID: plumbing.NewHash("abcdef12345678900987654321fedcbaabcdef12"),
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s", EntryHeader, RefKey, "refs/heads/main", CommitIDKey, "abcdef12345678900987654321fedcbaabcdef12"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			message, _ := test.entry.createCommitMessage()
			if !assert.Equal(t, test.expectedMessage, message) {
				t.Errorf("expected\n%s\n\ngot\n%s", test.expectedMessage, message)
			}
		})
	}
}

func TestAnnotationCreateCommitMessage(t *testing.T) {
	tests := map[string]struct {
		entry           *Annotation
		expectedMessage string
	}{
		"annotation, no message": {
			entry: &Annotation{
				RSLEntryIDs: []plumbing.Hash{plumbing.ZeroHash},
				Skip:        true,
				Message:     "",
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s", AnnotationHeader, EntryIDKey, plumbing.ZeroHash.String(), SkipKey, "true"),
		},
		"annotation, with message": {
			entry: &Annotation{
				RSLEntryIDs: []plumbing.Hash{plumbing.ZeroHash},
				Skip:        true,
				Message:     "message",
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s\n%s\n%s", AnnotationHeader, EntryIDKey, plumbing.ZeroHash.String(), SkipKey, "true", BeginMessage, base64.StdEncoding.EncodeToString([]byte("message")), EndMessage),
		},
		"annotation, with multi-line message": {
			entry: &Annotation{
				RSLEntryIDs: []plumbing.Hash{plumbing.ZeroHash},
				Skip:        true,
				Message:     "message1\nmessage2",
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s\n%s\n%s", AnnotationHeader, EntryIDKey, plumbing.ZeroHash.String(), SkipKey, "true", BeginMessage, base64.StdEncoding.EncodeToString([]byte("message1\nmessage2")), EndMessage),
		},
		"annotation, no message, skip false": {
			entry: &Annotation{
				RSLEntryIDs: []plumbing.Hash{plumbing.ZeroHash},
				Skip:        false,
				Message:     "",
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s", AnnotationHeader, EntryIDKey, plumbing.ZeroHash.String(), SkipKey, "false"),
		},
		"annotation, no message, skip false, multiple entry IDs": {
			entry: &Annotation{
				RSLEntryIDs: []plumbing.Hash{plumbing.ZeroHash, plumbing.ZeroHash},
				Skip:        false,
				Message:     "",
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s", AnnotationHeader, EntryIDKey, plumbing.ZeroHash.String(), EntryIDKey, plumbing.ZeroHash.String(), SkipKey, "false"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			message, err := test.entry.createCommitMessage()
			if err != nil {
				t.Fatal(err)
			}
			if !assert.Equal(t, test.expectedMessage, message) {
				t.Errorf("expected\n%s\n\ngot\n%s", test.expectedMessage, message)
			}
		})
	}
}

func TestParseRSLEntryMessage(t *testing.T) {
	tests := map[string]struct {
		expectedEntry EntryType
		expectedError error
		message       string
	}{
		"entry, fully resolved ref": {
			expectedEntry: &Entry{
				RefName:  "refs/heads/main",
				CommitID: plumbing.ZeroHash,
			},
			message: fmt.Sprintf("%s\n\n%s: %s\n%s: %s", EntryHeader, RefKey, "refs/heads/main", CommitIDKey, plumbing.ZeroHash.String()),
		},
		"entry, non-zero commit": {
			expectedEntry: &Entry{
				RefName:  "refs/heads/main",
				CommitID: plumbing.NewHash("abcdef12345678900987654321fedcbaabcdef12"),
			},
			message: fmt.Sprintf("%s\n\n%s: %s\n%s: %s", EntryHeader, RefKey, "refs/heads/main", CommitIDKey, "abcdef12345678900987654321fedcbaabcdef12"),
		},
		"entry, missing header": {
			expectedError: ErrInvalidRSLEntry,
			message:       fmt.Sprintf("%s: %s\n%s: %s", RefKey, "refs/heads/main", CommitIDKey, plumbing.ZeroHash.String()),
		},
		"entry, missing information": {
			expectedError: ErrInvalidRSLEntry,
			message:       fmt.Sprintf("%s\n\n%s: %s", EntryHeader, RefKey, "refs/heads/main"),
		},
		"annotation, no message": {
			expectedEntry: &Annotation{
				RSLEntryIDs: []plumbing.Hash{plumbing.ZeroHash},
				Skip:        true,
				Message:     "",
			},
			message: fmt.Sprintf("%s\n\n%s: %s\n%s: %s", AnnotationHeader, EntryIDKey, plumbing.ZeroHash.String(), SkipKey, "true"),
		},
		"annotation, with message": {
			expectedEntry: &Annotation{
				RSLEntryIDs: []plumbing.Hash{plumbing.ZeroHash},
				Skip:        true,
				Message:     "message",
			},
			message: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s\n%s\n%s", AnnotationHeader, EntryIDKey, plumbing.ZeroHash.String(), SkipKey, "true", BeginMessage, base64.StdEncoding.EncodeToString([]byte("message")), EndMessage),
		},
		"annotation, with multi-line message": {
			expectedEntry: &Annotation{
				RSLEntryIDs: []plumbing.Hash{plumbing.ZeroHash},
				Skip:        true,
				Message:     "message1\nmessage2",
			},
			message: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s\n%s\n%s", AnnotationHeader, EntryIDKey, plumbing.ZeroHash.String(), SkipKey, "true", BeginMessage, base64.StdEncoding.EncodeToString([]byte("message1\nmessage2")), EndMessage),
		},
		"annotation, no message, skip false": {
			expectedEntry: &Annotation{
				RSLEntryIDs: []plumbing.Hash{plumbing.ZeroHash},
				Skip:        false,
				Message:     "",
			},
			message: fmt.Sprintf("%s\n\n%s: %s\n%s: %s", AnnotationHeader, EntryIDKey, plumbing.ZeroHash.String(), SkipKey, "false"),
		},
		"annotation, no message, skip false, multiple entry IDs": {
			expectedEntry: &Annotation{
				RSLEntryIDs: []plumbing.Hash{plumbing.ZeroHash, plumbing.ZeroHash},
				Skip:        false,
				Message:     "",
			},
			message: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s", AnnotationHeader, EntryIDKey, plumbing.ZeroHash.String(), EntryIDKey, plumbing.ZeroHash.String(), SkipKey, "false"),
		},
		"annotation, missing header": {
			expectedError: ErrInvalidRSLEntry,
			message:       fmt.Sprintf("%s: %s\n%s: %s\n%s\n%s\n%s", EntryIDKey, plumbing.ZeroHash.String(), SkipKey, "true", BeginMessage, base64.StdEncoding.EncodeToString([]byte("message")), EndMessage),
		},
		"annotation, missing information": {
			expectedError: ErrInvalidRSLEntry,
			message:       fmt.Sprintf("%s\n\n%s: %s", AnnotationHeader, EntryIDKey, plumbing.ZeroHash.String()),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			entry, err := parseRSLEntryText(test.message)
			if err != nil {
				assert.ErrorIs(t, err, test.expectedError)
			} else {
				if !assert.Equal(t, test.expectedEntry, entry) {
					t.Errorf("expected\n%+v\n\ngot\n%+v", test.expectedEntry, entry)
				}
			}
		})
	}
}
