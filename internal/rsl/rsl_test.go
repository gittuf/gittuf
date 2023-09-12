package rsl

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
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

func TestGetLatestNonGittufEntry(t *testing.T) {
	t.Run("mix of gittuf and non gittuf entries", func(t *testing.T) {
		repo, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}

		if err := InitializeNamespace(repo); err != nil {
			t.Fatal(err)
		}

		// Add the first gittuf entry
		if err := NewEntry("refs/gittuf/policy", plumbing.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		// Add non gittuf entries
		if err := NewEntry("refs/heads/main", plumbing.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		// At this point, latest entry should be returned
		expectedLatestEntry, err := GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}
		latestEntry, err := GetLatestNonGittufEntry(repo)
		assert.Nil(t, err)
		assert.Equal(t, expectedLatestEntry, latestEntry)

		// Add another gittuf entry
		if err := NewEntry("refs/gittuf/not-policy", plumbing.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		// At this point, the expected entry is the same as before
		latestEntry, err = GetLatestNonGittufEntry(repo)
		assert.Nil(t, err)
		assert.Equal(t, expectedLatestEntry, latestEntry)
	})

	t.Run("only gittuf entries", func(t *testing.T) {
		repo, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}

		if err := InitializeNamespace(repo); err != nil {
			t.Fatal(err)
		}

		// Add the first gittuf entry
		if err := NewEntry("refs/gittuf/policy", plumbing.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		_, err = GetLatestNonGittufEntry(repo)
		assert.ErrorIs(t, err, ErrRSLEntryNotFound)

		// Add another gittuf entry
		if err := NewEntry("refs/gittuf/not-policy", plumbing.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		_, err = GetLatestNonGittufEntry(repo)
		assert.ErrorIs(t, err, ErrRSLEntryNotFound)
	})
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

func TestGetNonGittufParentForEntry(t *testing.T) {
	t.Run("mix of gittuf and non gittuf entries", func(t *testing.T) {
		repo, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}

		if err := InitializeNamespace(repo); err != nil {
			t.Fatal(err)
		}

		// Add the first gittuf entry
		if err := NewEntry("refs/gittuf/policy", plumbing.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		// Add non gittuf entry
		if err := NewEntry("refs/heads/main", plumbing.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		expectedEntry, err := GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		// Add non gittuf entry
		if err := NewEntry("refs/heads/main", plumbing.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		latestEntry, err := GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		parentEntry, err := GetNonGittufParentForEntry(repo, latestEntry)
		assert.Nil(t, err)
		assert.Equal(t, expectedEntry, parentEntry)

		// Add another gittuf entry and then a non gittuf entry
		expectedEntry = latestEntry

		if err := NewEntry("refs/gittuf/not-policy", plumbing.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}
		if err := NewEntry("refs/gittuf/main", plumbing.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		latestEntry, err = GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		// The expected entry should be from before this latest gittuf addition
		parentEntry, err = GetNonGittufParentForEntry(repo, latestEntry)
		assert.Nil(t, err)
		assert.Equal(t, expectedEntry, parentEntry)
	})

	t.Run("only gittuf entries", func(t *testing.T) {
		repo, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}

		if err := InitializeNamespace(repo); err != nil {
			t.Fatal(err)
		}

		// Add the first gittuf entry
		if err := NewEntry("refs/gittuf/policy", plumbing.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		latestEntry, err := GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}
		_, err = GetNonGittufParentForEntry(repo, latestEntry)
		assert.ErrorIs(t, err, ErrRSLEntryNotFound)

		// Add another gittuf entry
		if err := NewEntry("refs/gittuf/not-policy", plumbing.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		latestEntry, err = GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		_, err = GetNonGittufParentForEntry(repo, latestEntry)
		assert.ErrorIs(t, err, ErrRSLEntryNotFound)
	})
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

func TestGetFirstEntryForCommit(t *testing.T) {
	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	if err := InitializeNamespace(repo); err != nil {
		t.Fatal(err)
	}

	emptyTreeHash, err := gitinterface.WriteTree(repo, nil)
	if err != nil {
		t.Fatal(err)
	}

	mainRef := "refs/heads/main"
	if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(mainRef), plumbing.ZeroHash)); err != nil {
		t.Fatal(err)
	}

	initialCommitIDs := []plumbing.Hash{}
	for i := 0; i < 3; i++ {
		if err := gitinterface.Commit(repo, emptyTreeHash, mainRef, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		ref, err := repo.Reference(plumbing.ReferenceName(mainRef), true)
		if err != nil {
			t.Fatal(err)
		}

		initialCommitIDs = append(initialCommitIDs, ref.Hash())
	}

	// Right now, the RSL has no entries.
	for _, commitID := range initialCommitIDs {
		commit, err := repo.CommitObject(commitID)
		if err != nil {
			t.Fatal(err)
		}
		_, err = GetFirstEntryForCommit(repo, commit)
		assert.ErrorIs(t, err, ErrNoRecordOfCommit)
	}

	if err := NewEntry(mainRef, initialCommitIDs[len(initialCommitIDs)-1]).Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	// At this point, searching for any commit's entry should return the
	// solitary RSL entry.
	latestEntryT, err := GetLatestEntry(repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, commitID := range initialCommitIDs {
		commit, err := repo.CommitObject(commitID)
		if err != nil {
			t.Fatal(err)
		}
		entry, err := GetFirstEntryForCommit(repo, commit)
		assert.Nil(t, err)
		assert.Equal(t, latestEntryT, entry)
	}

	// Now, let's branch off from this ref and add more commits.
	featureRef := "refs/heads/feature"
	// First, "checkout" the feature branch.
	if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(featureRef), initialCommitIDs[len(initialCommitIDs)-1])); err != nil {
		t.Fatal(err)
	}
	// Next, add some new commits to this branch.
	featureCommitIDs := []plumbing.Hash{}
	for i := 0; i < 3; i++ {
		if err := gitinterface.Commit(repo, emptyTreeHash, featureRef, "Feature commit", false); err != nil {
			t.Fatal(err)
		}
		ref, err := repo.Reference(plumbing.ReferenceName(featureRef), true)
		if err != nil {
			t.Fatal(err)
		}

		featureCommitIDs = append(featureCommitIDs, ref.Hash())
	}

	// The RSL hasn't seen these new commits, however.
	for _, commitID := range featureCommitIDs {
		commit, err := repo.CommitObject(commitID)
		if err != nil {
			t.Fatal(err)
		}
		_, err = GetFirstEntryForCommit(repo, commit)
		assert.ErrorIs(t, err, ErrNoRecordOfCommit)
	}

	if err := NewEntry(featureRef, featureCommitIDs[len(featureCommitIDs)-1]).Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	// At this point, searching for any of the original commits' entry should
	// return the first RSL entry.
	for _, commitID := range initialCommitIDs {
		commit, err := repo.CommitObject(commitID)
		if err != nil {
			t.Fatal(err)
		}
		entry, err := GetFirstEntryForCommit(repo, commit)
		assert.Nil(t, err)
		assert.Equal(t, latestEntryT, entry)
	}
	// Searching for the feature commits should return the second entry.
	latestEntryT, err = GetLatestEntry(repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, commitID := range featureCommitIDs {
		commit, err := repo.CommitObject(commitID)
		if err != nil {
			t.Fatal(err)
		}
		entry, err := GetFirstEntryForCommit(repo, commit)
		assert.Nil(t, err)
		assert.Equal(t, latestEntryT, entry)
	}

	// Now, fast forward main branch to the latest feature branch commit.
	oldRef, err := repo.Reference(plumbing.ReferenceName(mainRef), true)
	if err != nil {
		t.Fatal(err)
	}
	newRef := plumbing.NewHashReference(plumbing.ReferenceName(mainRef), featureCommitIDs[len(featureCommitIDs)-1])
	if err := repo.Storer.CheckAndSetReference(newRef, oldRef); err != nil {
		t.Fatal(err)
	}

	if err := NewEntry(mainRef, featureCommitIDs[len(featureCommitIDs)-1]).Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	// Testing for any of the feature commits should return the feature branch
	// entry, not the main branch entry.
	for _, commitID := range featureCommitIDs {
		commit, err := repo.CommitObject(commitID)
		if err != nil {
			t.Fatal(err)
		}
		entry, err := GetFirstEntryForCommit(repo, commit)
		assert.Nil(t, err)
		assert.Equal(t, latestEntryT, entry)
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
