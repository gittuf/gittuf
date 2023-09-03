package rsl

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/stretchr/testify/assert"
)

func TestInitializeNamespace(t *testing.T) {
	t.Run("clean repository", func(t *testing.T) {
		repo, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}

		if err := InitializeNamespace(repo); err != nil {
			t.Error(err)
		}

		ref, err := repo.Reference(plumbing.ReferenceName(RSLRef), true)
		assert.Nil(t, err)
		assert.Equal(t, plumbing.ZeroHash, ref.Hash())
	})

	t.Run("existing RSL namespace", func(t *testing.T) {
		repo, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}

		if err := InitializeNamespace(repo); err != nil {
			t.Fatal(err)
		}

		err = InitializeNamespace(repo)
		assert.ErrorIs(t, err, ErrRSLExists)
	})
}

func TestCheckRemoteRSLForUpdates(t *testing.T) {
	remoteName := "origin"

	t.Run("remote has updates for local", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "gittuf")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir) //nolint:errcheck

		localRepoDir := filepath.Join(tmpDir, "local")

		remoteRepoDir := filepath.Join(tmpDir, "remote")
		if err := os.Mkdir(remoteRepoDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Simulate remote actions
		remoteRepo, err := git.PlainInit(remoteRepoDir, false)
		if err != nil {
			t.Fatal(err)
		}

		if err := InitializeNamespace(remoteRepo); err != nil {
			t.Fatal(err)
		}

		if err := NewEntry("refs/heads/main", plumbing.ZeroHash).Commit(remoteRepo, false); err != nil {
			t.Fatal(err)
		}

		// Simulate clone
		copyDir(t, remoteRepoDir, localRepoDir)
		localRepo, err := git.PlainOpen(localRepoDir)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := localRepo.CreateRemote(&config.RemoteConfig{Name: remoteName, URLs: []string{remoteRepoDir}}); err != nil {
			t.Fatal(err)
		}

		// Simulate more remote actions
		if err := NewEntry("refs/heads/main", plumbing.ZeroHash).Commit(remoteRepo, false); err != nil {
			t.Fatal(err)
		}

		// Local should be notified that remote has updates
		hasUpdates, err := CheckRemoteRSLForUpdates(localRepo, remoteName)
		assert.Nil(t, err)
		assert.True(t, hasUpdates)
	})

	t.Run("remote has no updates for local", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "gittuf")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir) //nolint:errcheck

		localRepoDir := filepath.Join(tmpDir, "local")

		remoteRepoDir := filepath.Join(tmpDir, "remote")
		if err := os.Mkdir(remoteRepoDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Simulate remote actions
		remoteRepo, err := git.PlainInit(remoteRepoDir, false)
		if err != nil {
			t.Fatal(err)
		}

		if err := InitializeNamespace(remoteRepo); err != nil {
			t.Fatal(err)
		}

		if err := NewEntry("refs/heads/main", plumbing.ZeroHash).Commit(remoteRepo, false); err != nil {
			t.Fatal(err)
		}

		// Simulate clone
		copyDir(t, remoteRepoDir, localRepoDir)
		localRepo, err := git.PlainOpen(localRepoDir)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := localRepo.CreateRemote(&config.RemoteConfig{Name: remoteName, URLs: []string{remoteRepoDir}}); err != nil {
			t.Fatal(err)
		}

		// Local should be notified that remote has no updates
		hasUpdates, err := CheckRemoteRSLForUpdates(localRepo, remoteName)
		assert.Nil(t, err)
		assert.False(t, hasUpdates)
	})

	t.Run("local is ahead of remote", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "gittuf")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir) //nolint:errcheck

		localRepoDir := filepath.Join(tmpDir, "local")

		remoteRepoDir := filepath.Join(tmpDir, "remote")
		if err := os.Mkdir(remoteRepoDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Simulate remote actions
		remoteRepo, err := git.PlainInit(remoteRepoDir, false)
		if err != nil {
			t.Fatal(err)
		}

		if err := InitializeNamespace(remoteRepo); err != nil {
			t.Fatal(err)
		}

		if err := NewEntry("refs/heads/main", plumbing.ZeroHash).Commit(remoteRepo, false); err != nil {
			t.Fatal(err)
		}

		// Simulate clone
		copyDir(t, remoteRepoDir, localRepoDir)
		localRepo, err := git.PlainOpen(localRepoDir)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := localRepo.CreateRemote(&config.RemoteConfig{Name: remoteName, URLs: []string{remoteRepoDir}}); err != nil {
			t.Fatal(err)
		}

		// Simulate local actions
		if err := NewEntry("refs/heads/main", plumbing.ZeroHash).Commit(localRepo, false); err != nil {
			t.Fatal(err)
		}

		// Local should be notified that remote has no updates
		hasUpdates, err := CheckRemoteRSLForUpdates(localRepo, remoteName)
		assert.Nil(t, err)
		assert.False(t, hasUpdates)
	})

	t.Run("both have no entries", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "gittuf")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir) //nolint:errcheck

		localRepoDir := filepath.Join(tmpDir, "local")

		remoteRepoDir := filepath.Join(tmpDir, "remote")
		if err := os.Mkdir(remoteRepoDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Simulate remote actions
		remoteRepo, err := git.PlainInit(remoteRepoDir, false)
		if err != nil {
			t.Fatal(err)
		}

		if err := InitializeNamespace(remoteRepo); err != nil {
			t.Fatal(err)
		}

		// Simulate clone
		copyDir(t, remoteRepoDir, localRepoDir)
		localRepo, err := git.PlainOpen(localRepoDir)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := localRepo.CreateRemote(&config.RemoteConfig{Name: remoteName, URLs: []string{remoteRepoDir}}); err != nil {
			t.Fatal(err)
		}

		// Local should be notified that remote has no updates
		hasUpdates, err := CheckRemoteRSLForUpdates(localRepo, remoteName)
		assert.Nil(t, err)
		assert.False(t, hasUpdates)
	})

	t.Run("local has no entries", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "gittuf")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir) //nolint:errcheck

		localRepoDir := filepath.Join(tmpDir, "local")

		remoteRepoDir := filepath.Join(tmpDir, "remote")
		if err := os.Mkdir(remoteRepoDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Simulate remote actions
		remoteRepo, err := git.PlainInit(remoteRepoDir, false)
		if err != nil {
			t.Fatal(err)
		}

		if err := InitializeNamespace(remoteRepo); err != nil {
			t.Fatal(err)
		}

		// Simulate clone
		copyDir(t, remoteRepoDir, localRepoDir)
		localRepo, err := git.PlainOpen(localRepoDir)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := localRepo.CreateRemote(&config.RemoteConfig{Name: remoteName, URLs: []string{remoteRepoDir}}); err != nil {
			t.Fatal(err)
		}

		// Simulate remote actions
		if err := NewEntry("refs/heads/main", plumbing.ZeroHash).Commit(remoteRepo, false); err != nil {
			t.Fatal(err)
		}

		// Local should be notified that remote has updates
		hasUpdates, err := CheckRemoteRSLForUpdates(localRepo, remoteName)
		assert.Nil(t, err)
		assert.True(t, hasUpdates)
	})

	t.Run("remote and local have diverged", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "gittuf")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir) //nolint:errcheck

		localRepoDir := filepath.Join(tmpDir, "local")

		remoteRepoDir := filepath.Join(tmpDir, "remote")
		if err := os.Mkdir(remoteRepoDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Simulate remote actions
		remoteRepo, err := git.PlainInit(remoteRepoDir, false)
		if err != nil {
			t.Fatal(err)
		}

		if err := InitializeNamespace(remoteRepo); err != nil {
			t.Fatal(err)
		}

		if err := NewEntry("refs/heads/main", plumbing.ZeroHash).Commit(remoteRepo, false); err != nil {
			t.Fatal(err)
		}

		// Simulate clone
		copyDir(t, remoteRepoDir, localRepoDir)
		localRepo, err := git.PlainOpen(localRepoDir)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := localRepo.CreateRemote(&config.RemoteConfig{Name: remoteName, URLs: []string{remoteRepoDir}}); err != nil {
			t.Fatal(err)
		}

		// Simulate remote actions
		if err := NewEntry("refs/heads/main", plumbing.ZeroHash).Commit(remoteRepo, false); err != nil {
			t.Fatal(err)
		}

		// Simulate local actions
		if err := NewEntry("refs/heads/feature", plumbing.ZeroHash).Commit(localRepo, false); err != nil {
			t.Fatal(err)
		}

		// Local should be notified that remote has updates that needs to be
		// reconciled
		hasUpdates, err := CheckRemoteRSLForUpdates(localRepo, remoteName)
		assert.Nil(t, err)
		assert.True(t, hasUpdates)
	})
}

func TestNewEntry(t *testing.T) {
	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	if err := InitializeNamespace(repo); err != nil {
		t.Fatal(err)
	}

	if err := NewEntry("main", plumbing.ZeroHash).Commit(repo, false); err != nil {
		t.Error(err)
	}

	ref, err := repo.Reference(plumbing.ReferenceName(RSLRef), true)
	assert.Nil(t, err)
	assert.NotEqual(t, plumbing.ZeroHash, ref.Hash())

	commitObj, err := repo.CommitObject(ref.Hash())
	if err != nil {
		t.Error(err)
	}
	expectedMessage := fmt.Sprintf("%s\n\n%s: %s\n%s: %s", EntryHeader, RefKey, "main", CommitIDKey, plumbing.ZeroHash.String())
	assert.Equal(t, expectedMessage, commitObj.Message)
	assert.Empty(t, commitObj.ParentHashes)

	if err := NewEntry("main", plumbing.NewHash("abcdef1234567890")).Commit(repo, false); err != nil {
		t.Error(err)
	}

	originalRefHash := ref.Hash()

	ref, err = repo.Reference(plumbing.ReferenceName(RSLRef), true)
	if err != nil {
		t.Error(err)
	}

	commitObj, err = repo.CommitObject(ref.Hash())
	if err != nil {
		t.Error(err)
	}

	expectedMessage = fmt.Sprintf("%s\n\n%s: %s\n%s: %s", EntryHeader, RefKey, "main", CommitIDKey, plumbing.NewHash("abcdef1234567890"))
	assert.Equal(t, expectedMessage, commitObj.Message)
	assert.Contains(t, commitObj.ParentHashes, originalRefHash)
}

func TestGetLatestEntry(t *testing.T) {
	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	if err := InitializeNamespace(repo); err != nil {
		t.Error(err)
	}

	if err := NewEntry("main", plumbing.ZeroHash).Commit(repo, false); err != nil {
		t.Error(err)
	}

	if entry, err := GetLatestEntry(repo); err != nil {
		t.Error(err)
	} else {
		e := entry.(*Entry)
		assert.Equal(t, "main", e.RefName)
		assert.Equal(t, plumbing.ZeroHash, e.CommitID)
	}

	if err := NewEntry("feature", plumbing.NewHash("abcdef1234567890")).Commit(repo, false); err != nil {
		t.Error(err)
	}
	if entry, err := GetLatestEntry(repo); err != nil {
		t.Error(err)
	} else {
		e := entry.(*Entry)
		assert.NotEqual(t, "main", e.RefName)
		assert.NotEqual(t, plumbing.ZeroHash, e.CommitID)
	}

	ref, err := repo.Reference(plumbing.ReferenceName(RSLRef), true)
	if err != nil {
		t.Fatal(err)
	}
	entryID := ref.Hash()

	if err := NewAnnotation([]plumbing.Hash{entryID}, true, "This was a mistaken push!").Commit(repo, false); err != nil {
		t.Error(err)
	}

	if entry, err := GetLatestEntry(repo); err != nil {
		t.Error(err)
	} else {
		a := entry.(*Annotation)
		assert.True(t, a.Skip)
		assert.Equal(t, []plumbing.Hash{entryID}, a.RSLEntryIDs)
		assert.Equal(t, "This was a mistaken push!", a.Message)
	}
}

func TestGetLatestEntryForRef(t *testing.T) {
	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	if err := InitializeNamespace(repo); err != nil {
		t.Fatal(err)
	}

	if err := NewEntry("main", plumbing.ZeroHash).Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	rslRef, err := repo.Reference(plumbing.ReferenceName(RSLRef), true)
	if err != nil {
		t.Fatal(err)
	}

	if entry, err := GetLatestEntryForRef(repo, "main"); err != nil {
		t.Error(err)
	} else {
		assert.Equal(t, rslRef.Hash(), entry.ID)
	}

	if err := NewEntry("feature", plumbing.ZeroHash).Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	if entry, err := GetLatestEntryForRef(repo, "main"); err != nil {
		t.Error(err)
	} else {
		assert.Equal(t, rslRef.Hash(), entry.ID)
	}
}

func TestGetLatestEntryForRefBefore(t *testing.T) {
	t.Run("no annotations", func(t *testing.T) {
		repo, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}
		if err := InitializeNamespace(repo); err != nil {
			t.Fatal(err)
		}

		// RSL structure for the test
		// main <- feature <- main <- feature <- main
		testRefs := []string{"main", "feature", "main", "feature", "main"}
		entryIDs := []plumbing.Hash{}
		for _, ref := range testRefs {
			if err := NewEntry(ref, plumbing.ZeroHash).Commit(repo, false); err != nil {
				t.Fatal(err)
			}
			latest, err := GetLatestEntry(repo)
			if err != nil {
				t.Fatal(err)
			}
			entryIDs = append(entryIDs, latest.GetID())
		}

		entry, err := GetLatestEntryForRefBefore(repo, "main", entryIDs[4])
		assert.Nil(t, err)
		assert.Equal(t, entryIDs[2], entry.ID)

		entry, err = GetLatestEntryForRefBefore(repo, "main", entryIDs[3])
		assert.Nil(t, err)
		assert.Equal(t, entryIDs[2], entry.ID)

		entry, err = GetLatestEntryForRefBefore(repo, "feature", entryIDs[4])
		assert.Nil(t, err)
		assert.Equal(t, entryIDs[3], entry.ID)

		entry, err = GetLatestEntryForRefBefore(repo, "feature", entryIDs[3])
		assert.Nil(t, err)
		assert.Equal(t, entryIDs[1], entry.ID)

		_, err = GetLatestEntryForRefBefore(repo, "feature", entryIDs[1])
		assert.ErrorIs(t, err, ErrRSLEntryNotFound)
	})

	t.Run("with annotations", func(t *testing.T) {
		repo, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}
		if err := InitializeNamespace(repo); err != nil {
			t.Fatal(err)
		}

		// RSL structure for the test
		// main <- A <- feature <- A <- main <- A <- feature <- A <- main <- A
		testRefs := []string{"main", "feature", "main", "feature", "main"}
		entryIDs := []plumbing.Hash{}
		for _, ref := range testRefs {
			if err := NewEntry(ref, plumbing.ZeroHash).Commit(repo, false); err != nil {
				t.Fatal(err)
			}
			latest, err := GetLatestEntry(repo)
			if err != nil {
				t.Fatal(err)
			}
			entryIDs = append(entryIDs, latest.GetID())

			if err := NewAnnotation([]plumbing.Hash{latest.GetID()}, false, "test annotation").Commit(repo, false); err != nil {
				t.Fatal(err)
			}
			latest, err = GetLatestEntry(repo)
			if err != nil {
				t.Fatal(err)
			}
			entryIDs = append(entryIDs, latest.GetID())
		}

		entry, err := GetLatestEntryForRefBefore(repo, "main", entryIDs[4])
		assert.Nil(t, err)
		assert.Equal(t, entryIDs[0], entry.ID)

		entry, err = GetLatestEntryForRefBefore(repo, "main", entryIDs[3])
		assert.Nil(t, err)
		assert.Equal(t, entryIDs[0], entry.ID)

		entry, err = GetLatestEntryForRefBefore(repo, "feature", entryIDs[6])
		assert.Nil(t, err)
		assert.Equal(t, entryIDs[2], entry.ID)

		entry, err = GetLatestEntryForRefBefore(repo, "feature", entryIDs[7])
		assert.Nil(t, err)
		assert.Equal(t, entryIDs[6], entry.ID)

		_, err = GetLatestEntryForRefBefore(repo, "feature", entryIDs[1])
		assert.ErrorIs(t, err, ErrRSLEntryNotFound)
	})
}
func TestGetEntry(t *testing.T) {
	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	if err := InitializeNamespace(repo); err != nil {
		t.Fatal(err)
	}

	if err := NewEntry("main", plumbing.ZeroHash).Commit(repo, false); err != nil {
		t.Error(err)
	}

	ref, err := repo.Reference(plumbing.ReferenceName(RSLRef), true)
	if err != nil {
		t.Fatal(err)
	}

	initialEntryID := ref.Hash()

	if err := NewAnnotation([]plumbing.Hash{initialEntryID}, true, "This was a mistaken push!").Commit(repo, false); err != nil {
		t.Error(err)
	}

	ref, err = repo.Reference(plumbing.ReferenceName(RSLRef), true)
	if err != nil {
		t.Fatal(err)
	}

	annotationID := ref.Hash()

	if err := NewEntry("main", plumbing.ZeroHash).Commit(repo, false); err != nil {
		t.Error(err)
	}

	if entry, err := GetEntry(repo, initialEntryID); err != nil {
		t.Error(err)
	} else {
		e := entry.(*Entry)
		assert.Equal(t, "main", e.RefName)
		assert.Equal(t, plumbing.ZeroHash, e.CommitID)
	}

	if entry, err := GetEntry(repo, annotationID); err != nil {
		t.Error(err)
	} else {
		a := entry.(*Annotation)
		assert.True(t, a.Skip)
		assert.Equal(t, []plumbing.Hash{initialEntryID}, a.RSLEntryIDs)
		assert.Equal(t, "This was a mistaken push!", a.Message)
	}
}

func TestGetParentForEntry(t *testing.T) {
	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	if err := InitializeNamespace(repo); err != nil {
		t.Fatal(err)
	}

	// Assert no parent for first entry
	if err := NewEntry("main", plumbing.ZeroHash).Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	entry, err := GetLatestEntry(repo)
	if err != nil {
		t.Fatal(err)
	}
	entryID := entry.GetID()

	_, err = GetParentForEntry(repo, entry)
	assert.ErrorIs(t, err, ErrRSLEntryNotFound)

	// Find parent for an entry
	if err := NewEntry("main", plumbing.ZeroHash).Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	entry, err = GetLatestEntry(repo)
	if err != nil {
		t.Fatal(err)
	}

	parentEntry, err := GetParentForEntry(repo, entry)
	assert.Nil(t, err)
	assert.Equal(t, entryID, parentEntry.GetID())

	entryID = entry.GetID()

	// Find parent for an annotation
	if err := NewAnnotation([]plumbing.Hash{entryID}, false, "test annotation").Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	entry, err = GetLatestEntry(repo)
	if err != nil {
		t.Fatal(err)
	}

	parentEntry, err = GetParentForEntry(repo, entry)
	assert.Nil(t, err)
	assert.Equal(t, entryID, parentEntry.GetID())
}

func TestGetFirstEntry(t *testing.T) {
	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	if err := InitializeNamespace(repo); err != nil {
		t.Fatal(err)
	}

	if err := NewEntry("first", plumbing.ZeroHash).Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	firstEntryT, err := GetLatestEntry(repo)
	if err != nil {
		t.Fatal(err)
	}
	firstEntry := firstEntryT.(*Entry)

	for i := 0; i < 5; i++ {
		if err := NewEntry("main", plumbing.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}
	}

	testEntry, err := GetFirstEntry(repo)
	assert.Nil(t, err)
	assert.Equal(t, firstEntry, testEntry)

	for i := 0; i < 5; i++ {
		if err := NewAnnotation([]plumbing.Hash{firstEntry.ID}, false, "test annotation").Commit(repo, false); err != nil {
			t.Fatal(err)
		}
	}

	testEntry, err = GetFirstEntry(repo)
	assert.Nil(t, err)
	assert.Equal(t, firstEntry, testEntry)
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
				ID:       plumbing.ZeroHash,
				RefName:  "refs/heads/main",
				CommitID: plumbing.ZeroHash,
			},
			message: fmt.Sprintf("%s\n\n%s: %s\n%s: %s", EntryHeader, RefKey, "refs/heads/main", CommitIDKey, plumbing.ZeroHash.String()),
		},
		"entry, non-zero commit": {
			expectedEntry: &Entry{
				ID:       plumbing.ZeroHash,
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
				ID:          plumbing.ZeroHash,
				RSLEntryIDs: []plumbing.Hash{plumbing.ZeroHash},
				Skip:        true,
				Message:     "",
			},
			message: fmt.Sprintf("%s\n\n%s: %s\n%s: %s", AnnotationHeader, EntryIDKey, plumbing.ZeroHash.String(), SkipKey, "true"),
		},
		"annotation, with message": {
			expectedEntry: &Annotation{
				ID:          plumbing.ZeroHash,
				RSLEntryIDs: []plumbing.Hash{plumbing.ZeroHash},
				Skip:        true,
				Message:     "message",
			},
			message: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s\n%s\n%s", AnnotationHeader, EntryIDKey, plumbing.ZeroHash.String(), SkipKey, "true", BeginMessage, base64.StdEncoding.EncodeToString([]byte("message")), EndMessage),
		},
		"annotation, with multi-line message": {
			expectedEntry: &Annotation{
				ID:          plumbing.ZeroHash,
				RSLEntryIDs: []plumbing.Hash{plumbing.ZeroHash},
				Skip:        true,
				Message:     "message1\nmessage2",
			},
			message: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s\n%s\n%s", AnnotationHeader, EntryIDKey, plumbing.ZeroHash.String(), SkipKey, "true", BeginMessage, base64.StdEncoding.EncodeToString([]byte("message1\nmessage2")), EndMessage),
		},
		"annotation, no message, skip false": {
			expectedEntry: &Annotation{
				ID:          plumbing.ZeroHash,
				RSLEntryIDs: []plumbing.Hash{plumbing.ZeroHash},
				Skip:        false,
				Message:     "",
			},
			message: fmt.Sprintf("%s\n\n%s: %s\n%s: %s", AnnotationHeader, EntryIDKey, plumbing.ZeroHash.String(), SkipKey, "false"),
		},
		"annotation, no message, skip false, multiple entry IDs": {
			expectedEntry: &Annotation{
				ID:          plumbing.ZeroHash,
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
			entry, err := parseRSLEntryText(plumbing.ZeroHash, test.message)
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

// copyDir recursively copies a directory tree, attempting to preserve permissions.
// Source directory must exist, destination directory must *not* exist.
// Symlinks are ignored and skipped.
// Source: https://gist.github.com/r0l1/92462b38df26839a3ca324697c8cba04
// Licensed MIT, Author: Roland Singer [roland.singer@desertbit.com]
func copyDir(t *testing.T, src string, dst string) {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	si, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}
	if !si.IsDir() {
		t.Fatal(fmt.Errorf("source is not a directory"))
	}

	_, err = os.Stat(dst)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err == nil {
		t.Fatal(fmt.Errorf("destination already exists"))
	}

	err = os.MkdirAll(dst, si.Mode())
	if err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			copyDir(t, srcPath, dstPath)
		} else {
			// Skip symlinks.
			if entry.Type()&os.ModeSymlink != 0 {
				continue
			}

			copyFile(t, srcPath, dstPath)
		}
	}
}

// copyFile copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the contents
// of the source file. The file mode will be copied from the source and
// the copied data is synced/flushed to stable storage.
// Source: https://gist.github.com/r0l1/92462b38df26839a3ca324697c8cba04
// Licensed MIT, Author: Roland Singer [roland.singer@desertbit.com]
func copyFile(t *testing.T, src, dst string) {
	t.Helper()

	in, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := out.Close(); e != nil {
			t.Fatal(e)
		}
	}()

	_, err = io.Copy(out, in)
	if err != nil {
		t.Fatal(err)
	}

	err = out.Sync()
	if err != nil {
		t.Fatal(err)
	}

	si, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Chmod(dst, si.Mode())
	if err != nil {
		t.Fatal(err)
	}
}
