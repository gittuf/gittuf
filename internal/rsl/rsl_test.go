// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package rsl

import (
	"encoding/base64"
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/gittuf/gittuf/internal/tuf"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const annotationMessage = "test annotation"

func TestNewReferenceEntry(t *testing.T) {
	tempDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

	if err := NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	currentTip, err := repo.GetReference(Ref)
	if err != nil {
		t.Fatal(err)
	}

	commitMessage, err := repo.GetCommitMessage(currentTip)
	if err != nil {
		t.Fatal(err)
	}

	parentIDs, err := repo.GetCommitParentIDs(currentTip)
	if err != nil {
		t.Fatal(err)
	}

	expectedMessage := fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %d", ReferenceEntryHeader, RefKey, "refs/heads/main", TargetIDKey, gitinterface.ZeroHash.String(), NumberKey, 1)
	assert.Equal(t, expectedMessage, commitMessage)
	assert.Nil(t, parentIDs)

	if err := NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	newTip, err := repo.GetReference(Ref)
	if err != nil {
		t.Fatal(err)
	}

	commitMessage, err = repo.GetCommitMessage(newTip)
	if err != nil {
		t.Fatal(err)
	}

	parentIDs, err = repo.GetCommitParentIDs(newTip)
	if err != nil {
		t.Fatal(err)
	}

	expectedMessage = fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %d", ReferenceEntryHeader, RefKey, "refs/heads/main", TargetIDKey, gitinterface.ZeroHash.String(), NumberKey, 2)
	assert.Equal(t, expectedMessage, commitMessage)
	assert.Contains(t, parentIDs, currentTip)
}

func TestReferenceUpdaterEntry(t *testing.T) {
	refName := "refs/heads/main"
	gitID := gitinterface.ZeroHash
	upstreamRepository := "http://git.example.com/repository"

	t.Run("reference entry", func(t *testing.T) {
		entry := Entry(NewReferenceEntry(refName, gitID))
		updaterEntry, isReferenceUpdaterEntry := entry.(ReferenceUpdaterEntry)
		assert.True(t, isReferenceUpdaterEntry)
		assert.Equal(t, refName, updaterEntry.GetRefName())
		assert.Equal(t, gitID, updaterEntry.GetTargetID())
	})

	t.Run("propagation entry", func(t *testing.T) {
		entry := Entry(NewPropagationEntry(refName, gitID, upstreamRepository, gitID))
		updaterEntry, isReferenceUpdaterEntry := entry.(ReferenceUpdaterEntry)
		assert.True(t, isReferenceUpdaterEntry)
		assert.Equal(t, refName, updaterEntry.GetRefName())
		assert.Equal(t, gitID, updaterEntry.GetTargetID())
	})
}

func TestGetLatestEntry(t *testing.T) {
	tempDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

	if err := NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	entry, err := GetLatestEntry(repo)
	assert.Nil(t, err)
	e := entry.(*ReferenceEntry)
	assert.Equal(t, "refs/heads/main", e.RefName)
	assert.Equal(t, gitinterface.ZeroHash, e.TargetID)

	if err := NewReferenceEntry("refs/heads/feature", gitinterface.ZeroHash).Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	entry, err = GetLatestEntry(repo)
	assert.Nil(t, err)
	e = entry.(*ReferenceEntry)
	assert.Equal(t, "refs/heads/feature", e.RefName)
	assert.Equal(t, gitinterface.ZeroHash, e.TargetID)

	latestTip, err := repo.GetReference(Ref)
	if err != nil {
		t.Fatal(err)
	}

	if err := NewAnnotationEntry([]gitinterface.Hash{latestTip}, true, "This was a mistaken push!").Commit(repo, false); err != nil {
		t.Error(err)
	}

	entry, err = GetLatestEntry(repo)
	assert.Nil(t, err)
	a := entry.(*AnnotationEntry)
	assert.True(t, a.Skip)
	assert.Equal(t, []gitinterface.Hash{latestTip}, a.RSLEntryIDs)
	assert.Equal(t, "This was a mistaken push!", a.Message)
}

func TestGetLatestReferenceUpdaterEntry(t *testing.T) {
	t.Run("with ref name", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		refName := "refs/heads/main"
		otherRefName := "refs/heads/feature"

		if err := NewReferenceEntry(refName, gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		rslRef, err := repo.GetReference(Ref)
		if err != nil {
			t.Fatal(err)
		}

		entry, annotations, err := GetLatestReferenceUpdaterEntry(repo, ForReference(refName))
		assert.Nil(t, err)
		assert.Nil(t, annotations)
		assert.Equal(t, rslRef, entry.GetID())

		if err := NewReferenceEntry(otherRefName, gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		entry, annotations, err = GetLatestReferenceUpdaterEntry(repo, ForReference(refName))
		assert.Nil(t, err)
		assert.Nil(t, annotations)
		assert.Equal(t, rslRef, entry.GetID())

		// Add annotation for the target entry
		if err := NewAnnotationEntry([]gitinterface.Hash{entry.GetID()}, false, annotationMessage).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		entry, annotations, err = GetLatestReferenceUpdaterEntry(repo, ForReference(refName))
		assert.Nil(t, err)
		assert.Equal(t, rslRef, entry.GetID())
		assertAnnotationsReferToEntry(t, entry, annotations)
	})

	t.Run("with invalid conditions", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		emptyTreeID, err := repo.EmptyTree()
		if err != nil {
			t.Fatal(err)
		}

		// Both Before
		_, _, err = GetLatestReferenceUpdaterEntry(repo, BeforeEntryID(emptyTreeID), BeforeEntryNumber(3))
		assert.ErrorIs(t, err, ErrInvalidGetLatestReferenceUpdaterEntryOptions)

		// Both Until
		_, _, err = GetLatestReferenceUpdaterEntry(repo, UntilEntryID(emptyTreeID), UntilEntryNumber(3))
		assert.ErrorIs(t, err, ErrInvalidGetLatestReferenceUpdaterEntryOptions)

		// Before number is less than until number
		_, _, err = GetLatestReferenceUpdaterEntry(repo, BeforeEntryNumber(2), UntilEntryNumber(29))
		assert.ErrorIs(t, err, ErrInvalidGetLatestReferenceUpdaterEntryOptions)

		// Until number is greater than latest entry in the RSL
		if err := NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}
		_, _, err = GetLatestReferenceUpdaterEntry(repo, UntilEntryNumber(10))
		assert.ErrorIs(t, err, ErrInvalidUntilEntryNumberCondition)

		// a reference older than until entry number is being asked for (configuration mismatch)
		if err := NewReferenceEntry("feature", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}
		entry, annotations, err := GetLatestReferenceUpdaterEntry(repo, ForReference("main"), BeforeEntryID(emptyTreeID), UntilEntryNumber(2))
		assert.Nil(t, annotations)
		assert.Nil(t, entry)
		assert.ErrorIs(t, err, ErrInvalidGetLatestReferenceUpdaterEntryOptions)
	})

	t.Run("with ref name and until entry number", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		refName := "refs/heads/main"
		otherRefName := "refs/heads/feature"

		if err := NewReferenceEntry(refName, gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		// RSL: main

		rslRef, err := repo.GetReference(Ref)
		if err != nil {
			t.Fatal(err)
		}

		entry, annotations, err := GetLatestReferenceUpdaterEntry(repo, ForReference(refName), UntilEntryNumber(1))
		// until is inclusive
		assert.Nil(t, err)
		assert.Nil(t, annotations)
		assert.Equal(t, rslRef, entry.GetID())

		if err := NewReferenceEntry(otherRefName, gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		// RSL: main <- feature

		entry, annotations, err = GetLatestReferenceUpdaterEntry(repo, ForReference(refName), UntilEntryNumber(1))
		assert.Nil(t, err)
		assert.Nil(t, annotations)
		assert.Equal(t, rslRef, entry.GetID())

		// Add annotation for the target entry
		if err := NewAnnotationEntry([]gitinterface.Hash{entry.GetID()}, false, annotationMessage).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		// RSL: main <- feature <- annotation-on-main

		entry, annotations, err = GetLatestReferenceUpdaterEntry(repo, ForReference(refName), UntilEntryNumber(1))
		assert.Nil(t, err)
		assert.Equal(t, rslRef, entry.GetID())
		assertAnnotationsReferToEntry(t, entry, annotations)

		// Set higher until limit
		_, _, err = GetLatestReferenceUpdaterEntry(repo, ForReference(refName), UntilEntryNumber(2))
		assert.ErrorIs(t, err, ErrRSLEntryNotFound)
	})

	t.Run("with ref name and before entry ID", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		// RSL structure for the test
		// main <- feature <- main <- feature <- main
		testRefs := []string{"main", "feature", "main", "feature", "main"}
		entryIDs := []gitinterface.Hash{}
		for _, ref := range testRefs {
			if err := NewReferenceEntry(ref, gitinterface.ZeroHash).Commit(repo, false); err != nil {
				t.Fatal(err)
			}
			latest, err := GetLatestEntry(repo)
			if err != nil {
				t.Fatal(err)
			}
			entryIDs = append(entryIDs, latest.GetID())
		}

		entry, annotations, err := GetLatestReferenceUpdaterEntry(repo, ForReference("main"), BeforeEntryID(entryIDs[4]))
		assert.Nil(t, err)
		assert.Nil(t, annotations)
		assert.Equal(t, entryIDs[2], entry.GetID())

		entry, annotations, err = GetLatestReferenceUpdaterEntry(repo, ForReference("main"), BeforeEntryID(entryIDs[3]))
		assert.Nil(t, err)
		assert.Nil(t, annotations)
		assert.Equal(t, entryIDs[2], entry.GetID())

		entry, annotations, err = GetLatestReferenceUpdaterEntry(repo, ForReference("feature"), BeforeEntryID(entryIDs[4]))
		assert.Nil(t, err)
		assert.Nil(t, annotations)
		assert.Equal(t, entryIDs[3], entry.GetID())

		entry, annotations, err = GetLatestReferenceUpdaterEntry(repo, ForReference("feature"), BeforeEntryID(entryIDs[3]))
		assert.Nil(t, err)
		assert.Nil(t, annotations)
		assert.Equal(t, entryIDs[1], entry.GetID())

		_, _, err = GetLatestReferenceUpdaterEntry(repo, ForReference("feature"), BeforeEntryID(entryIDs[1]))
		assert.ErrorIs(t, err, ErrRSLEntryNotFound)
	})

	t.Run("with ref name, before entry ID, and annotations", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		// RSL structure for the test
		// main <- A <- feature <- A <- main <- A <- feature <- A <- main <- A
		testRefs := []string{"main", "feature", "main", "feature", "main"}
		entryIDs := []gitinterface.Hash{}
		for _, ref := range testRefs {
			if err := NewReferenceEntry(ref, gitinterface.ZeroHash).Commit(repo, false); err != nil {
				t.Fatal(err)
			}
			latest, err := GetLatestEntry(repo)
			if err != nil {
				t.Fatal(err)
			}
			entryIDs = append(entryIDs, latest.GetID())

			if err := NewAnnotationEntry([]gitinterface.Hash{latest.GetID()}, false, annotationMessage).Commit(repo, false); err != nil {
				t.Fatal(err)
			}
			latest, err = GetLatestEntry(repo)
			if err != nil {
				t.Fatal(err)
			}
			entryIDs = append(entryIDs, latest.GetID())
		}

		entry, annotations, err := GetLatestReferenceUpdaterEntry(repo, ForReference("main"), BeforeEntryID(entryIDs[4]))
		assert.Nil(t, err)
		assert.Equal(t, entryIDs[0], entry.GetID())
		assertAnnotationsReferToEntry(t, entry, annotations)
		// Add an annotation at the end for some entry and see it gets pulled in
		// even when the anchor is for its ancestor
		assert.Len(t, annotations, 1) // before adding an annotation, we have just 1
		if err := NewAnnotationEntry([]gitinterface.Hash{entryIDs[0]}, false, annotationMessage).Commit(repo, false); err != nil {
			t.Fatal(err)
		}
		entry, annotations, err = GetLatestReferenceUpdaterEntry(repo, ForReference("main"), BeforeEntryID(entryIDs[4]))
		assert.Nil(t, err)
		assert.Equal(t, entryIDs[0], entry.GetID())
		assertAnnotationsReferToEntry(t, entry, annotations)
		assert.Len(t, annotations, 2) // now we have 2

		entry, annotations, err = GetLatestReferenceUpdaterEntry(repo, ForReference("main"), BeforeEntryID(entryIDs[3]))
		assert.Nil(t, err)
		assert.Equal(t, entryIDs[0], entry.GetID())
		assertAnnotationsReferToEntry(t, entry, annotations)

		entry, annotations, err = GetLatestReferenceUpdaterEntry(repo, ForReference("feature"), BeforeEntryID(entryIDs[6]))
		assert.Nil(t, err)
		assert.Equal(t, entryIDs[2], entry.GetID())
		assertAnnotationsReferToEntry(t, entry, annotations)

		entry, annotations, err = GetLatestReferenceUpdaterEntry(repo, ForReference("feature"), BeforeEntryID(entryIDs[7]))
		assert.Nil(t, err)
		assert.Equal(t, entryIDs[6], entry.GetID())
		assertAnnotationsReferToEntry(t, entry, annotations)

		_, _, err = GetLatestReferenceUpdaterEntry(repo, ForReference("feature"), BeforeEntryID(entryIDs[1]))
		assert.ErrorIs(t, err, ErrRSLEntryNotFound)
	})

	t.Run("with ref name, before entry ID and until entry number", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		// RSL structure for the test
		// 1    <- 2       <- 3    <- 4       <- 5
		// main <- feature <- main <- feature <- main
		testRefs := []string{"main", "feature", "main", "feature", "main"}
		entryIDs := []gitinterface.Hash{}
		for _, ref := range testRefs {
			if err := NewReferenceEntry(ref, gitinterface.ZeroHash).Commit(repo, false); err != nil {
				t.Fatal(err)
			}
			latest, err := GetLatestEntry(repo)
			if err != nil {
				t.Fatal(err)
			}
			entryIDs = append(entryIDs, latest.GetID())
		}

		entry, annotations, err := GetLatestReferenceUpdaterEntry(repo, ForReference("main"), BeforeEntryID(entryIDs[4]), UntilEntryNumber(1))
		assert.Nil(t, err)
		assert.Nil(t, annotations)
		assert.Equal(t, entryIDs[2], entry.GetID())

		entry, annotations, err = GetLatestReferenceUpdaterEntry(repo, ForReference("main"), BeforeEntryID(entryIDs[3]), UntilEntryNumber(1))
		assert.Nil(t, err)
		assert.Nil(t, annotations)
		assert.Equal(t, entryIDs[2], entry.GetID())

		entry, annotations, err = GetLatestReferenceUpdaterEntry(repo, ForReference("feature"), BeforeEntryID(entryIDs[4]), UntilEntryNumber(1))
		assert.Nil(t, err)
		assert.Nil(t, annotations)
		assert.Equal(t, entryIDs[3], entry.GetID())

		entry, annotations, err = GetLatestReferenceUpdaterEntry(repo, ForReference("feature"), BeforeEntryID(entryIDs[3]), UntilEntryNumber(1))
		assert.Nil(t, err)
		assert.Nil(t, annotations)
		assert.Equal(t, entryIDs[1], entry.GetID())

		_, _, err = GetLatestReferenceUpdaterEntry(repo, ForReference("feature"), BeforeEntryID(entryIDs[1]), UntilEntryNumber(1))
		assert.ErrorIs(t, err, ErrRSLEntryNotFound)

		// Set higher limits to constrain search
		_, _, err = GetLatestReferenceUpdaterEntry(repo, ForReference("main"), BeforeEntryID(entryIDs[4]), UntilEntryNumber(5))
		assert.ErrorIs(t, err, ErrRSLEntryNotFound)

		entry, annotations, err = GetLatestReferenceUpdaterEntry(repo, ForReference("main"), BeforeEntryID(entryIDs[4]), UntilEntryNumber(3))
		// until is inclusive
		assert.Nil(t, err)
		assert.Nil(t, annotations)
		assert.Equal(t, entryIDs[2], entry.GetID())

		_, _, err = GetLatestReferenceUpdaterEntry(repo, ForReference("feature"), BeforeEntryID(entryIDs[3]), UntilEntryNumber(3))
		assert.ErrorIs(t, err, ErrRSLEntryNotFound)
	})

	t.Run("with ref name but before entry ID is not found in initial walk", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		// RSL structure for the test
		// 1    <- 2
		// main <- feature
		testRefs := []string{"main", "feature"}
		for _, ref := range testRefs {
			if err := NewReferenceEntry(ref, gitinterface.ZeroHash).Commit(repo, false); err != nil {
				t.Fatal(err)
			}
		}
		emptyTreeID, err := repo.EmptyTree()
		if err != nil {
			t.Fatal(err)
		}
		entry, annotations, err := GetLatestReferenceUpdaterEntry(repo, ForReference("main"), BeforeEntryID(emptyTreeID))
		assert.Nil(t, annotations)
		assert.Nil(t, entry)
		assert.ErrorIs(t, err, ErrRSLEntryNotFound)
	})

	t.Run("with ref name, before entry ID, until entry number and with annotations", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		// RSL structure for the test
		// main <- A <- feature <- A <- main <- A <- feature <- A <- main <- A
		testRefs := []string{"main", "feature", "main", "feature", "main"}
		entryIDs := []gitinterface.Hash{}
		for _, ref := range testRefs {
			if err := NewReferenceEntry(ref, gitinterface.ZeroHash).Commit(repo, false); err != nil {
				t.Fatal(err)
			}
			latest, err := GetLatestEntry(repo)
			if err != nil {
				t.Fatal(err)
			}
			entryIDs = append(entryIDs, latest.GetID())

			if err := NewAnnotationEntry([]gitinterface.Hash{latest.GetID()}, false, annotationMessage).Commit(repo, false); err != nil {
				t.Fatal(err)
			}
			latest, err = GetLatestEntry(repo)
			if err != nil {
				t.Fatal(err)
			}
			entryIDs = append(entryIDs, latest.GetID())
		}

		entry, annotations, err := GetLatestReferenceUpdaterEntry(repo, ForReference("main"), BeforeEntryID(entryIDs[4]), UntilEntryNumber(1))
		assert.Nil(t, err)
		assert.Equal(t, entryIDs[0], entry.GetID())
		assertAnnotationsReferToEntry(t, entry, annotations)
		// Add an annotation at the end for some entry and see it gets pulled in
		// even when the anchor is for its ancestor
		assert.Len(t, annotations, 1) // before adding an annotation, we have just 1
		if err := NewAnnotationEntry([]gitinterface.Hash{entryIDs[0]}, false, annotationMessage).Commit(repo, false); err != nil {
			t.Fatal(err)
		}
		entry, annotations, err = GetLatestReferenceUpdaterEntry(repo, ForReference("main"), BeforeEntryID(entryIDs[4]), UntilEntryNumber(1))
		assert.Nil(t, err)
		assert.Equal(t, entryIDs[0], entry.GetID())
		assertAnnotationsReferToEntry(t, entry, annotations)
		assert.Len(t, annotations, 2) // now we have 2

		entry, annotations, err = GetLatestReferenceUpdaterEntry(repo, ForReference("main"), BeforeEntryID(entryIDs[3]), UntilEntryNumber(1))
		assert.Nil(t, err)
		assert.Equal(t, entryIDs[0], entry.GetID())
		assertAnnotationsReferToEntry(t, entry, annotations)

		entry, annotations, err = GetLatestReferenceUpdaterEntry(repo, ForReference("feature"), BeforeEntryID(entryIDs[6]), UntilEntryNumber(1))
		assert.Nil(t, err)
		assert.Equal(t, entryIDs[2], entry.GetID())
		assertAnnotationsReferToEntry(t, entry, annotations)

		entry, annotations, err = GetLatestReferenceUpdaterEntry(repo, ForReference("feature"), BeforeEntryID(entryIDs[7]), UntilEntryNumber(1))
		assert.Nil(t, err)
		assert.Equal(t, entryIDs[6], entry.GetID())
		assertAnnotationsReferToEntry(t, entry, annotations)

		_, _, err = GetLatestReferenceUpdaterEntry(repo, ForReference("feature"), BeforeEntryID(entryIDs[1]), UntilEntryNumber(1))
		assert.ErrorIs(t, err, ErrRSLEntryNotFound)

		// Set higher until limits
		_, _, err = GetLatestReferenceUpdaterEntry(repo, ForReference("main"), BeforeEntryID(entryIDs[3]), UntilEntryNumber(2))
		assert.ErrorIs(t, err, ErrRSLEntryNotFound)

		entry, annotations, err = GetLatestReferenceUpdaterEntry(repo, ForReference("feature"), BeforeEntryID(entryIDs[6]), UntilEntryNumber(3))
		// until is inclusive
		assert.Nil(t, err)
		assert.Equal(t, entryIDs[2], entry.GetID())
		assertAnnotationsReferToEntry(t, entry, annotations)
	})

	t.Run("with ref name and unskipped", func(t *testing.T) {
		refName := "refs/heads/main"

		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		entryIDs := []gitinterface.Hash{}

		// Add an entry
		if err := NewReferenceEntry(refName, gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		// Latest unskipped entry is the one we just added
		e, err := GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}
		entryIDs = append(entryIDs, e.GetID())

		entry, annotations, err := GetLatestReferenceUpdaterEntry(repo, ForReference(refName), IsUnskipped())
		assert.Nil(t, err)
		assert.Empty(t, annotations)
		assert.Equal(t, entryIDs[len(entryIDs)-1], entry.GetID())

		// Add another entry
		if err := NewReferenceEntry(refName, gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		// Record latest entry's ID
		e, err = GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}
		entryIDs = append(entryIDs, e.GetID())

		// Latest unskipped entry is the newest one
		entry, annotations, err = GetLatestReferenceUpdaterEntry(repo, ForReference(refName), IsUnskipped())
		assert.Nil(t, err)
		assert.Empty(t, annotations)
		assert.Equal(t, entryIDs[len(entryIDs)-1], entry.GetID())

		// Skip the second one
		if err := NewAnnotationEntry([]gitinterface.Hash{entryIDs[1]}, true, "revoke").Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		// Now the latest unskipped entry should be the first one
		entry, annotations, err = GetLatestReferenceUpdaterEntry(repo, ForReference(refName), IsUnskipped())
		assert.Nil(t, err)
		assert.Empty(t, annotations)
		assert.Equal(t, entryIDs[0], entry.GetID())

		// Skip the first one too to trigger error
		if err := NewAnnotationEntry([]gitinterface.Hash{entryIDs[0]}, true, "revoke").Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		entry, annotations, err = GetLatestReferenceUpdaterEntry(repo, ForReference(refName), IsUnskipped())
		assert.Nil(t, entry)
		assert.Empty(t, annotations)
		assert.ErrorIs(t, err, ErrRSLEntryNotFound)
	})

	t.Run("with ref name, unskipped, and before entry ID", func(t *testing.T) {
		refName := "refs/heads/main"

		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		entryIDs := []gitinterface.Hash{}

		// Add an entry
		if err := NewReferenceEntry(refName, gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		// Latest unskipped entry is the one we just added
		e, err := GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}
		entryIDs = append(entryIDs, e.GetID())

		// Add another entry
		if err := NewReferenceEntry(refName, gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		// Record latest entry's ID
		e, err = GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}
		entryIDs = append(entryIDs, e.GetID())

		// Latest unskipped before the current entry is the first entry
		entry, annotations, err := GetLatestReferenceUpdaterEntry(repo, ForReference(refName), BeforeEntryID(entryIDs[1]), IsUnskipped())
		assert.Nil(t, err)
		assert.Empty(t, annotations)
		assert.Equal(t, entryIDs[0], entry.GetID())

		// Skip the second one
		if err := NewAnnotationEntry([]gitinterface.Hash{entryIDs[1]}, true, "revoke").Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		// Now even the latest unskipped entry with zero hash should return the first one
		entry, annotations, err = GetLatestReferenceUpdaterEntry(repo, ForReference(refName), BeforeEntryID(gitinterface.ZeroHash), IsUnskipped())
		assert.Nil(t, err)
		assert.Empty(t, annotations)
		assert.Equal(t, entryIDs[0], entry.GetID())

		// Skip the first one too to trigger error
		if err := NewAnnotationEntry([]gitinterface.Hash{entryIDs[0]}, true, "revoke").Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		entry, annotations, err = GetLatestReferenceUpdaterEntry(repo, ForReference(refName), BeforeEntryID(gitinterface.ZeroHash), IsUnskipped())
		assert.Nil(t, entry)
		assert.Empty(t, annotations)
		assert.ErrorIs(t, err, ErrRSLEntryNotFound)
	})

	t.Run("with non gittuf option, mix of gittuf and non gittuf entries", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		// Add the first gittuf entry
		if err := NewReferenceEntry("refs/gittuf/policy", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		// Add non gittuf entries
		if err := NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		// At this point, latest entry should be returned
		expectedLatestEntry, err := GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}
		latestEntry, annotations, err := GetLatestReferenceUpdaterEntry(repo, ForNonGittufReference())
		assert.Nil(t, err)
		assert.Nil(t, annotations)
		assert.Equal(t, expectedLatestEntry, latestEntry)

		// Add another gittuf entry
		if err := NewReferenceEntry("refs/gittuf/not-policy", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		// At this point, the expected entry is the same as before
		latestEntry, annotations, err = GetLatestReferenceUpdaterEntry(repo, ForNonGittufReference())
		assert.Nil(t, err)
		assert.Nil(t, annotations)
		assert.Equal(t, expectedLatestEntry, latestEntry)

		// Add an annotation for latest entry, check that it's returned
		if err := NewAnnotationEntry([]gitinterface.Hash{expectedLatestEntry.GetID()}, false, annotationMessage).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		latestEntry, annotations, err = GetLatestReferenceUpdaterEntry(repo, ForNonGittufReference())
		assert.Nil(t, err)
		assert.Equal(t, expectedLatestEntry, latestEntry)
		assertAnnotationsReferToEntry(t, latestEntry, annotations)
	})

	t.Run("with non gittuf option, only gittuf entries", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		// Add the first gittuf entry
		if err := NewReferenceEntry("refs/gittuf/policy", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		_, _, err := GetLatestReferenceUpdaterEntry(repo, ForNonGittufReference())
		assert.ErrorIs(t, err, ErrRSLEntryNotFound)

		// Add another gittuf entry
		if err := NewReferenceEntry("refs/gittuf/not-policy", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		_, _, err = GetLatestReferenceUpdaterEntry(repo, ForNonGittufReference())
		assert.ErrorIs(t, err, ErrRSLEntryNotFound)
	})

	t.Run("transitioning from no numbers to numbers", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		// Add non-numbered entries, including an annotation
		if err := NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).commitWithoutNumber(repo); err != nil {
			t.Fatal(err)
		}

		entry, err := GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		if err := NewAnnotationEntry([]gitinterface.Hash{entry.GetID()}, false, "annotation").commitWithoutNumber(repo); err != nil {
			t.Fatal(err)
		}

		_, _, err = GetLatestReferenceUpdaterEntry(repo, ForReference("refs/heads/main"), BeforeEntryNumber(1))
		assert.ErrorIs(t, err, ErrCannotUseEntryNumberFilter)

		_, _, err = GetLatestReferenceUpdaterEntry(repo, ForReference("refs/heads/main"), UntilEntryNumber(1))
		assert.ErrorIs(t, err, ErrCannotUseEntryNumberFilter)

		// Add numbered entries
		if err := NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		expectedEntry, err := GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		entry, annotations, err := GetLatestReferenceUpdaterEntry(repo, ForReference("refs/heads/main"), UntilEntryNumber(1))
		assert.Nil(t, err)
		assert.Equal(t, expectedEntry.GetID(), entry.GetID())
		assert.Nil(t, annotations)
	})

	t.Run("success with propagation entry for repository", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		if err := NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		upstreamRepository := "http://git.example.com/repository"
		if err := NewPropagationEntry("refs/heads/main", gitinterface.ZeroHash, upstreamRepository, gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		latestEntry, err := GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		entry, _, err := GetLatestReferenceUpdaterEntry(repo, IsPropagationEntryForRepository(upstreamRepository))
		assert.Nil(t, err)
		assert.Equal(t, latestEntry.GetID(), entry.GetID())
	})

	t.Run("expected failure with propagation entry for repository", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		if err := NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		upstreamRepository := "http://git.example.com/repository"
		if err := NewPropagationEntry("refs/heads/main", gitinterface.ZeroHash, upstreamRepository, gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		otherUpstreamRepository := "http://example.com/git/repository"
		entry, _, err := GetLatestReferenceUpdaterEntry(repo, IsPropagationEntryForRepository(otherUpstreamRepository))
		assert.ErrorIs(t, err, ErrRSLEntryNotFound)
		assert.Nil(t, entry)
	})

	t.Run("success with propagation entry for repository and ref", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		if err := NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		upstreamRepository := "http://git.example.com/repository"
		if err := NewPropagationEntry("refs/heads/feature", gitinterface.ZeroHash, upstreamRepository, gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		latestEntry, err := GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		if err := NewPropagationEntry("refs/heads/main", gitinterface.ZeroHash, upstreamRepository, gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		entry, _, err := GetLatestReferenceUpdaterEntry(repo, IsPropagationEntryForRepository(upstreamRepository), ForReference("refs/heads/feature"))
		assert.Nil(t, err)
		assert.Equal(t, latestEntry.GetID(), entry.GetID())
	})

	t.Run("success with isReferenceEntry", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		if err := NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		latestEntry, err := GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		upstreamRepository := "http://git.example.com/repository"
		if err := NewPropagationEntry("refs/heads/feature", gitinterface.ZeroHash, upstreamRepository, gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		if err := NewPropagationEntry("refs/heads/main", gitinterface.ZeroHash, upstreamRepository, gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		entry, _, err := GetLatestReferenceUpdaterEntry(repo, IsReferenceEntry())
		assert.Nil(t, err)
		assert.Equal(t, latestEntry.GetID(), entry.GetID())
	})

	t.Run("expected failure with isReferenceEntry", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		upstreamRepository := "http://git.example.com/repository"
		if err := NewPropagationEntry("refs/heads/feature", gitinterface.ZeroHash, upstreamRepository, gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		if err := NewPropagationEntry("refs/heads/main", gitinterface.ZeroHash, upstreamRepository, gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		entry, _, err := GetLatestReferenceUpdaterEntry(repo, IsReferenceEntry())
		assert.ErrorIs(t, err, ErrRSLEntryNotFound)
		assert.Nil(t, entry)
	})

	t.Run("invalid options, using both IsReferenceEntry and IsPropagationEntryForRepository", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		entry, _, err := GetLatestReferenceUpdaterEntry(repo, IsReferenceEntry(), IsPropagationEntryForRepository("http://example.com/git/repository"))
		assert.ErrorIs(t, err, ErrInvalidGetLatestReferenceUpdaterEntryOptions)
		assert.Nil(t, entry)
	})
}

func TestGetEntry(t *testing.T) {
	tempDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

	if err := NewReferenceEntry("main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	initialEntryID, err := repo.GetReference(Ref)
	if err != nil {
		t.Fatal(err)
	}

	if err := NewAnnotationEntry([]gitinterface.Hash{initialEntryID}, true, "This was a mistaken push!").Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	annotationID, err := repo.GetReference(Ref)
	if err != nil {
		t.Fatal(err)
	}

	if err := NewReferenceEntry("main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
		t.Error(err)
	}

	entry, err := GetEntry(repo, initialEntryID)
	assert.Nil(t, err)
	e := entry.(*ReferenceEntry)
	assert.Equal(t, "main", e.RefName)
	assert.Equal(t, gitinterface.ZeroHash, e.TargetID)

	entry, err = GetEntry(repo, annotationID)
	assert.Nil(t, err)
	a := entry.(*AnnotationEntry)
	assert.True(t, a.Skip)
	assert.Equal(t, []gitinterface.Hash{initialEntryID}, a.RSLEntryIDs)
	assert.Equal(t, "This was a mistaken push!", a.Message)
}

func TestGetParentForEntry(t *testing.T) {
	t.Run("regular test", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		// Assert no parent for first entry
		if err := NewReferenceEntry("main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
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
		if err := NewReferenceEntry("main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
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
		if err := NewAnnotationEntry([]gitinterface.Hash{entryID}, false, annotationMessage).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		entry, err = GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		parentEntry, err = GetParentForEntry(repo, entry)
		assert.Nil(t, err)
		assert.Equal(t, entryID, parentEntry.GetID())
	})

	t.Run("transition from no number to with number", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		if err := NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).commitWithoutNumber(repo); err != nil {
			t.Fatal(err)
		}

		nonNumberedEntry, err := GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		if err := NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		numberedEntry, err := GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, uint64(1), numberedEntry.GetNumber())

		parentEntry, err := GetParentForEntry(repo, numberedEntry)
		assert.Nil(t, err)
		assert.Equal(t, uint64(0), parentEntry.GetNumber())
		assert.Equal(t, nonNumberedEntry.GetID(), parentEntry.GetID())
	})
}

func TestGetNonGittufParentReferenceUpdaterEntryForEntry(t *testing.T) {
	t.Run("mix of gittuf and non gittuf entries", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		// Add the first gittuf entry
		if err := NewReferenceEntry("refs/gittuf/policy", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		// Add non gittuf entry
		if err := NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		expectedEntry, err := GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		// Add non gittuf entry
		if err := NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		latestEntry, err := GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		parentEntry, annotations, err := GetNonGittufParentReferenceUpdaterEntryForEntry(repo, latestEntry)
		assert.Nil(t, err)
		assert.Nil(t, annotations)
		assert.Equal(t, expectedEntry, parentEntry)

		// Add another gittuf entry and then a non gittuf entry
		expectedEntry = latestEntry

		if err := NewReferenceEntry("refs/gittuf/not-policy", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}
		if err := NewReferenceEntry("refs/gittuf/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		latestEntry, err = GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		// The expected entry should be from before this latest gittuf addition
		parentEntry, annotations, err = GetNonGittufParentReferenceUpdaterEntryForEntry(repo, latestEntry)
		assert.Nil(t, err)
		assert.Nil(t, annotations)
		assert.Equal(t, expectedEntry, parentEntry)

		// Add annotation pertaining to the expected entry
		if err := NewAnnotationEntry([]gitinterface.Hash{expectedEntry.GetID()}, false, annotationMessage).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		parentEntry, annotations, err = GetNonGittufParentReferenceUpdaterEntryForEntry(repo, latestEntry)
		assert.Nil(t, err)
		assert.Equal(t, expectedEntry, parentEntry)
		assertAnnotationsReferToEntry(t, parentEntry, annotations)
	})

	t.Run("only gittuf entries", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		// Add the first gittuf entry
		if err := NewReferenceEntry("refs/gittuf/policy", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		latestEntry, err := GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}
		_, _, err = GetNonGittufParentReferenceUpdaterEntryForEntry(repo, latestEntry)
		assert.ErrorIs(t, err, ErrRSLEntryNotFound)

		// Add another gittuf entry
		if err := NewReferenceEntry("refs/gittuf/not-policy", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		latestEntry, err = GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		_, _, err = GetNonGittufParentReferenceUpdaterEntryForEntry(repo, latestEntry)
		assert.ErrorIs(t, err, ErrRSLEntryNotFound)
	})
}

func TestGetFirstEntry(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

	if err := NewReferenceEntry("first", gitinterface.ZeroHash).Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	firstEntryT, err := GetLatestEntry(repo)
	if err != nil {
		t.Fatal(err)
	}
	firstEntry := firstEntryT.(*ReferenceEntry)

	for i := 0; i < 5; i++ {
		if err := NewReferenceEntry("main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}
	}

	testEntry, annotations, err := GetFirstEntry(repo)
	assert.Nil(t, err)
	assert.Nil(t, annotations)
	assert.Equal(t, firstEntry, testEntry)

	for i := 0; i < 5; i++ {
		if err := NewAnnotationEntry([]gitinterface.Hash{firstEntry.ID}, false, annotationMessage).Commit(repo, false); err != nil {
			t.Fatal(err)
		}
	}

	testEntry, annotations, err = GetFirstEntry(repo)
	assert.Nil(t, err)
	assert.Equal(t, firstEntry, testEntry)
	assert.Equal(t, 5, len(annotations))
	assertAnnotationsReferToEntry(t, firstEntry, annotations)
}

func TestGetFirstReferenceUpdaterEntryForRef(t *testing.T) {
	tempDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

	if err := NewReferenceEntry("first", gitinterface.ZeroHash).Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	firstEntryT, err := GetLatestEntry(repo)
	if err != nil {
		t.Fatal(err)
	}
	firstEntry := firstEntryT.(*ReferenceEntry)

	for i := 0; i < 5; i++ {
		if err := NewReferenceEntry("main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}
	}

	testEntry, annotations, err := GetFirstReferenceUpdaterEntryForRef(repo, "first")
	assert.Nil(t, err)
	assert.Nil(t, annotations)
	assert.Equal(t, firstEntry, testEntry)

	for i := 0; i < 5; i++ {
		if err := NewAnnotationEntry([]gitinterface.Hash{firstEntry.ID}, false, annotationMessage).Commit(repo, false); err != nil {
			t.Fatal(err)
		}
	}

	testEntry, annotations, err = GetFirstReferenceUpdaterEntryForRef(repo, "first")
	assert.Nil(t, err)
	assert.Equal(t, firstEntry, testEntry)
	assert.Equal(t, 5, len(annotations))
	assertAnnotationsReferToEntry(t, firstEntry, annotations)
}

func TestSkipAllInvalidReferenceEntriesForRef(t *testing.T) {
	t.Run("skip latest entry", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		treeBuilder := gitinterface.NewTreeBuilder(repo)
		emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
		require.Nil(t, err)

		initialCommitHash, err := repo.Commit(emptyTreeHash, "refs/heads/main", "Initial commit\n", false)
		require.Nil(t, err)

		if err := NewReferenceEntry("refs/heads/main", initialCommitHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		toBeSkippedEntry, err := GetLatestEntry(repo)
		require.Nil(t, err)

		// Create a different commit and override the ref
		if err := repo.SetReference("refs/heads/main", gitinterface.ZeroHash); err != nil {
			t.Fatal(err)
		}
		newCommitHash, err := repo.Commit(emptyTreeHash, "refs/heads/main", "Real initial commit\n", false)
		require.Nil(t, err)

		if err := NewReferenceEntry("refs/heads/main", newCommitHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		if err := SkipAllInvalidReferenceEntriesForRef(repo, "refs/heads/main", false); err != nil {
			t.Fatal(err)
		}

		latestEntry, err := GetLatestEntry(repo)
		require.Nil(t, err)

		annotationEntry, isAnnotation := latestEntry.(*AnnotationEntry)
		if !isAnnotation {
			t.Fatal("invalid entry type")
		}

		assert.Equal(t, []gitinterface.Hash{toBeSkippedEntry.GetID()}, annotationEntry.RSLEntryIDs)
	})

	t.Run("skip multiple entries", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		treeBuilder := gitinterface.NewTreeBuilder(repo)
		emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
		require.Nil(t, err)

		skippedEntries := []gitinterface.Hash{}

		initialCommitHash, err := repo.Commit(emptyTreeHash, "refs/heads/main", "Initial commit\n", false)
		require.Nil(t, err)

		if err := NewReferenceEntry("refs/heads/main", initialCommitHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		toBeSkippedEntry, err := GetLatestEntry(repo)
		require.Nil(t, err)
		skippedEntries = append(skippedEntries, toBeSkippedEntry.GetID())

		// Add another commit and entry that'll be skipped later
		secondCommitHash, err := repo.Commit(emptyTreeHash, "refs/heads/main", "Second commit\n", false)
		require.Nil(t, err)

		if err := NewReferenceEntry("refs/heads/main", secondCommitHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		toBeSkippedEntry, err = GetLatestEntry(repo)
		require.Nil(t, err)
		skippedEntries = append(skippedEntries, toBeSkippedEntry.GetID())

		// Create a different commit and override the ref
		if err := repo.SetReference("refs/heads/main", gitinterface.ZeroHash); err != nil {
			t.Fatal(err)
		}
		newCommitHash, err := repo.Commit(emptyTreeHash, "refs/heads/main", "Real initial commit\n", false)
		require.Nil(t, err)

		if err := NewReferenceEntry("refs/heads/main", newCommitHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		if err := SkipAllInvalidReferenceEntriesForRef(repo, "refs/heads/main", false); err != nil {
			t.Fatal(err)
		}

		latestEntry, err := GetLatestEntry(repo)
		require.Nil(t, err)

		annotationEntry, isAnnotation := latestEntry.(*AnnotationEntry)
		if !isAnnotation {
			t.Fatal("invalid entry type")
		}

		// we have to reverse the order of one of the lists
		slices.Reverse[[]gitinterface.Hash](skippedEntries)
		assert.Equal(t, skippedEntries, annotationEntry.RSLEntryIDs)
	})

	t.Run("just one entry, nothing should change", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		treeBuilder := gitinterface.NewTreeBuilder(repo)
		emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
		require.Nil(t, err)

		initialCommitHash, err := repo.Commit(emptyTreeHash, "refs/heads/main", "Initial commit\n", false)
		require.Nil(t, err)

		if err := NewReferenceEntry("refs/heads/main", initialCommitHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		originalLatestEntry, err := GetLatestEntry(repo)
		require.Nil(t, err)

		if err := SkipAllInvalidReferenceEntriesForRef(repo, "refs/heads/main", false); err != nil {
			t.Fatal(err)
		}

		newLatestEntry, err := GetLatestEntry(repo)
		require.Nil(t, err)

		// Confirm no annotation was created
		if _, isReferenceEntry := newLatestEntry.(*ReferenceEntry); !isReferenceEntry {
			t.Fatal(fmt.Errorf("invalid entry type"))
		}

		assert.Equal(t, originalLatestEntry, newLatestEntry)
	})

	t.Run("multiple entries, nothing should change", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		treeBuilder := gitinterface.NewTreeBuilder(repo)
		emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
		require.Nil(t, err)

		initialCommitHash, err := repo.Commit(emptyTreeHash, "refs/heads/main", "Initial commit\n", false)
		require.Nil(t, err)

		if err := NewReferenceEntry("refs/heads/main", initialCommitHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		anotherCommitHash, err := repo.Commit(emptyTreeHash, "refs/heads/main", "Second commit\n", false)
		require.Nil(t, err)

		if err := NewReferenceEntry("refs/heads/main", anotherCommitHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		originalLatestEntry, err := GetLatestEntry(repo)
		require.Nil(t, err)

		if err := SkipAllInvalidReferenceEntriesForRef(repo, "refs/heads/main", false); err != nil {
			t.Fatal(err)
		}

		newLatestEntry, err := GetLatestEntry(repo)
		require.Nil(t, err)

		// Confirm no annotation was created
		if _, isReferenceEntry := newLatestEntry.(*ReferenceEntry); !isReferenceEntry {
			t.Fatal(fmt.Errorf("invalid entry type"))
		}

		assert.Equal(t, originalLatestEntry, newLatestEntry)
	})
}

func TestGetFirstReferenceUpdaterEntryForCommit(t *testing.T) {
	tempDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

	treeBuilder := gitinterface.NewTreeBuilder(repo)
	emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
	if err != nil {
		t.Fatal(err)
	}

	mainRef := "refs/heads/main"

	initialTargetIDs := []gitinterface.Hash{}
	for i := 0; i < 3; i++ {
		commitID, err := repo.Commit(emptyTreeHash, mainRef, "Test commit", false)
		if err != nil {
			t.Fatal(err)
		}

		initialTargetIDs = append(initialTargetIDs, commitID)
	}

	// Right now, the RSL has no entries.
	for _, commitID := range initialTargetIDs {
		_, _, err = GetFirstReferenceUpdaterEntryForCommit(repo, commitID)
		assert.ErrorIs(t, err, ErrNoRecordOfCommit)
	}

	if err := NewReferenceEntry(mainRef, initialTargetIDs[len(initialTargetIDs)-1]).Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	// At this point, searching for any commit's entry should return the
	// solitary RSL entry.
	latestEntryT, err := GetLatestEntry(repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, commitID := range initialTargetIDs {
		entry, annotations, err := GetFirstReferenceUpdaterEntryForCommit(repo, commitID)
		assert.Nil(t, err)
		assert.Nil(t, annotations)
		assert.Equal(t, latestEntryT, entry)
	}

	// Now, let's branch off from this ref and add more commits.
	featureRef := "refs/heads/feature"
	// First, "checkout" the feature branch.
	if err := repo.SetReference(featureRef, initialTargetIDs[len(initialTargetIDs)-1]); err != nil {
		t.Fatal(err)
	}

	// Next, add some new commits to this branch.
	featureTargetIDs := []gitinterface.Hash{}
	for i := 0; i < 3; i++ {
		commitID, err := repo.Commit(emptyTreeHash, featureRef, "Feature commit", false)
		if err != nil {
			t.Fatal(err)
		}

		featureTargetIDs = append(featureTargetIDs, commitID)
	}

	// The RSL hasn't seen these new commits, however.
	for _, commitID := range featureTargetIDs {
		_, _, err = GetFirstReferenceUpdaterEntryForCommit(repo, commitID)
		assert.ErrorIs(t, err, ErrNoRecordOfCommit)
	}

	if err := NewReferenceEntry(featureRef, featureTargetIDs[len(featureTargetIDs)-1]).Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	// At this point, searching for any of the original commits' entry should
	// return the first RSL entry.
	for _, commitID := range initialTargetIDs {
		entry, annotations, err := GetFirstReferenceUpdaterEntryForCommit(repo, commitID)
		assert.Nil(t, err)
		assert.Nil(t, annotations)
		assert.Equal(t, latestEntryT, entry)
	}
	// Searching for the feature commits should return the second entry.
	latestEntryT, err = GetLatestEntry(repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, commitID := range featureTargetIDs {
		entry, annotations, err := GetFirstReferenceUpdaterEntryForCommit(repo, commitID)
		assert.Nil(t, err)
		assert.Nil(t, annotations)
		assert.Equal(t, latestEntryT, entry)
	}

	// Now, fast forward main branch to the latest feature branch commit.
	if err := repo.SetReference(mainRef, featureTargetIDs[len(featureTargetIDs)-1]); err != nil {
		t.Fatal(err)
	}

	if err := NewReferenceEntry(mainRef, featureTargetIDs[len(featureTargetIDs)-1]).Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	// Testing for any of the feature commits should return the feature branch
	// entry, not the main branch entry.
	for _, commitID := range featureTargetIDs {
		entry, annotations, err := GetFirstReferenceUpdaterEntryForCommit(repo, commitID)
		assert.Nil(t, err)
		assert.Nil(t, annotations)
		assert.Equal(t, latestEntryT, entry)
	}

	// Add annotation for feature entry
	if err := NewAnnotationEntry([]gitinterface.Hash{latestEntryT.GetID()}, false, annotationMessage).Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	latestEntry := latestEntryT.(*ReferenceEntry)
	for _, commitID := range featureTargetIDs {
		entry, annotations, err := GetFirstReferenceUpdaterEntryForCommit(repo, commitID)
		assert.Nil(t, err)
		assert.Equal(t, latestEntryT, entry)
		assertAnnotationsReferToEntry(t, latestEntry, annotations)
	}
}

func TestGetReferenceUpdaterEntriesInRange(t *testing.T) {
	refName := "refs/heads/main"
	anotherRefName := "refs/heads/feature"

	// We add a mix of reference entries and annotations, establishing expected
	// return values as we go along

	tempDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

	expectedEntries := []ReferenceUpdaterEntry{}
	expectedAnnotationMap := map[string][]*AnnotationEntry{}

	// Add some entries to main
	for i := 0; i < 3; i++ {
		if err := NewReferenceEntry(refName, gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		// We run GetLatestEntry so that the entry has its ID set as well
		entry, err := GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}
		expectedEntries = append(expectedEntries, entry.(ReferenceUpdaterEntry))
	}

	// Add some annotations
	for i := 0; i < 3; i++ {
		if err := NewAnnotationEntry([]gitinterface.Hash{expectedEntries[i].GetID()}, false, annotationMessage).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		annotation, err := GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}
		expectedAnnotationMap[expectedEntries[i].GetID().String()] = []*AnnotationEntry{annotation.(*AnnotationEntry)}
	}

	// Each entry has one annotation
	entries, annotationMap, err := GetReferenceUpdaterEntriesInRange(repo, expectedEntries[0].GetID(), expectedEntries[len(expectedEntries)-1].GetID())
	assert.Nil(t, err)
	assert.Equal(t, expectedEntries, entries)
	assert.Equal(t, expectedAnnotationMap, annotationMap)

	// Add an entry and annotation for feature branch
	if err := NewReferenceEntry(anotherRefName, gitinterface.ZeroHash).Commit(repo, false); err != nil {
		t.Fatal(err)
	}
	latestEntry, err := GetLatestEntry(repo)
	if err != nil {
		t.Fatal(err)
	}
	expectedEntries = append(expectedEntries, latestEntry.(*ReferenceEntry))
	if err := NewAnnotationEntry([]gitinterface.Hash{latestEntry.GetID()}, false, annotationMessage).Commit(repo, false); err != nil {
		t.Fatal(err)
	}
	latestEntry, err = GetLatestEntry(repo)
	if err != nil {
		t.Fatal(err)
	}
	expectedAnnotationMap[expectedEntries[len(expectedEntries)-1].GetID().String()] = []*AnnotationEntry{latestEntry.(*AnnotationEntry)}

	// Expected values include the feature branch entry and annotation
	entries, annotationMap, err = GetReferenceUpdaterEntriesInRange(repo, expectedEntries[0].GetID(), expectedEntries[len(expectedEntries)-1].GetID())
	assert.Nil(t, err)
	assert.Equal(t, expectedEntries, entries)
	assert.Equal(t, expectedAnnotationMap, annotationMap)

	// Add an annotation that refers to two valid entries
	if err := NewAnnotationEntry([]gitinterface.Hash{expectedEntries[0].GetID(), expectedEntries[1].GetID()}, false, annotationMessage).Commit(repo, false); err != nil {
		t.Fatal(err)
	}
	latestEntry, err = GetLatestEntry(repo)
	if err != nil {
		t.Fatal(err)
	}
	// This annotation is relevant to both entries
	annotation := latestEntry.(*AnnotationEntry)
	expectedAnnotationMap[expectedEntries[0].GetID().String()] = append(expectedAnnotationMap[expectedEntries[0].GetID().String()], annotation)
	expectedAnnotationMap[expectedEntries[1].GetID().String()] = append(expectedAnnotationMap[expectedEntries[1].GetID().String()], annotation)

	entries, annotationMap, err = GetReferenceUpdaterEntriesInRange(repo, expectedEntries[0].GetID(), expectedEntries[len(expectedEntries)-1].GetID())
	assert.Nil(t, err)
	assert.Equal(t, expectedEntries, entries)
	assert.Equal(t, expectedAnnotationMap, annotationMap)

	// Add a gittuf namespace entry and ensure it's returned as relevant
	if err := NewReferenceEntry("refs/gittuf/relevant", gitinterface.ZeroHash).Commit(repo, false); err != nil {
		t.Fatal(err)
	}
	latestEntry, err = GetLatestEntry(repo)
	if err != nil {
		t.Fatal(err)
	}
	expectedEntries = append(expectedEntries, latestEntry.(*ReferenceEntry))

	entries, annotationMap, err = GetReferenceUpdaterEntriesInRange(repo, expectedEntries[0].GetID(), expectedEntries[len(expectedEntries)-1].GetID())
	assert.Nil(t, err)
	assert.Equal(t, expectedEntries, entries)
	assert.Equal(t, expectedAnnotationMap, annotationMap)
}

func TestGetReferenceUpdaterEntriesInRangeForRef(t *testing.T) {
	refName := "refs/heads/main"
	anotherRefName := "refs/heads/feature"

	// We add a mix of reference entries and annotations, establishing expected
	// return values as we go along

	tempDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

	expectedEntries := []ReferenceUpdaterEntry{}
	expectedAnnotationMap := map[string][]*AnnotationEntry{}

	// Add some entries to main
	for i := 0; i < 3; i++ {
		if err := NewReferenceEntry(refName, gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		// We run GetLatestEntry so that the entry has its ID set as well
		entry, err := GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}
		expectedEntries = append(expectedEntries, entry.(*ReferenceEntry))
	}

	// Add some annotations
	for i := 0; i < 3; i++ {
		if err := NewAnnotationEntry([]gitinterface.Hash{expectedEntries[i].GetID()}, false, annotationMessage).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		annotation, err := GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}
		expectedAnnotationMap[expectedEntries[i].GetID().String()] = []*AnnotationEntry{annotation.(*AnnotationEntry)}
	}

	// Each entry has one annotation
	entries, annotationMap, err := GetReferenceUpdaterEntriesInRangeForRef(repo, expectedEntries[0].GetID(), expectedEntries[len(expectedEntries)-1].GetID(), refName)
	assert.Nil(t, err)
	assert.Equal(t, expectedEntries, entries)
	assert.Equal(t, expectedAnnotationMap, annotationMap)

	// Add an entry and annotation for feature branch
	if err := NewReferenceEntry(anotherRefName, gitinterface.ZeroHash).Commit(repo, false); err != nil {
		t.Fatal(err)
	}
	latestEntry, err := GetLatestEntry(repo)
	if err != nil {
		t.Fatal(err)
	}
	if err := NewAnnotationEntry([]gitinterface.Hash{latestEntry.GetID()}, false, annotationMessage).Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	// Expected values do not change
	entries, annotationMap, err = GetReferenceUpdaterEntriesInRangeForRef(repo, expectedEntries[0].GetID(), expectedEntries[len(expectedEntries)-1].GetID(), refName)
	assert.Nil(t, err)
	assert.Equal(t, expectedEntries, entries)
	assert.Equal(t, expectedAnnotationMap, annotationMap)

	// Add an annotation that refers to two valid entries
	if err := NewAnnotationEntry([]gitinterface.Hash{expectedEntries[0].GetID(), expectedEntries[1].GetID()}, false, annotationMessage).Commit(repo, false); err != nil {
		t.Fatal(err)
	}
	latestEntry, err = GetLatestEntry(repo)
	if err != nil {
		t.Fatal(err)
	}
	// This annotation is relevant to both entries
	annotation := latestEntry.(*AnnotationEntry)
	expectedAnnotationMap[expectedEntries[0].GetID().String()] = append(expectedAnnotationMap[expectedEntries[0].GetID().String()], annotation)
	expectedAnnotationMap[expectedEntries[1].GetID().String()] = append(expectedAnnotationMap[expectedEntries[1].GetID().String()], annotation)

	entries, annotationMap, err = GetReferenceUpdaterEntriesInRangeForRef(repo, expectedEntries[0].GetID(), expectedEntries[len(expectedEntries)-1].GetID(), refName)
	assert.Nil(t, err)
	assert.Equal(t, expectedEntries, entries)
	assert.Equal(t, expectedAnnotationMap, annotationMap)

	// Add a gittuf namespace entry and ensure it's returned as relevant
	if err := NewReferenceEntry("refs/gittuf/relevant", gitinterface.ZeroHash).Commit(repo, false); err != nil {
		t.Fatal(err)
	}
	latestEntry, err = GetLatestEntry(repo)
	if err != nil {
		t.Fatal(err)
	}
	expectedEntries = append(expectedEntries, latestEntry.(*ReferenceEntry))

	entries, annotationMap, err = GetReferenceUpdaterEntriesInRangeForRef(repo, expectedEntries[0].GetID(), expectedEntries[len(expectedEntries)-1].GetID(), refName)
	assert.Nil(t, err)
	assert.Equal(t, expectedEntries, entries)
	assert.Equal(t, expectedAnnotationMap, annotationMap)
}

func TestPropagateChangesFromUpstreamRepository(t *testing.T) {
	// Create upstreamRepo
	upstreamRepoLocation := t.TempDir()
	upstreamRepo := gitinterface.CreateTestGitRepository(t, upstreamRepoLocation, true)

	downstreamRepoLocation := t.TempDir()
	downstreamRepo := gitinterface.CreateTestGitRepository(t, downstreamRepoLocation, true)

	propagationDetails := &tufv01.PropagationDirective{
		UpstreamReference:   "refs/heads/main",
		UpstreamRepository:  upstreamRepoLocation,
		DownstreamReference: "refs/heads/main",
		DownstreamPath:      "upstream",
	}

	err := PropagateChangesFromUpstreamRepository(downstreamRepo, upstreamRepo, []tuf.PropagationDirective{propagationDetails}, false)
	assert.Nil(t, err) // propagation has nothing to do because no RSL exists in upstream

	// Add things to upstreamRepo
	blobAID, err := upstreamRepo.WriteBlob([]byte("a"))
	if err != nil {
		t.Fatal(err)
	}

	blobBID, err := upstreamRepo.WriteBlob([]byte("b"))
	if err != nil {
		t.Fatal(err)
	}

	upstreamTreeBuilder := gitinterface.NewTreeBuilder(upstreamRepo)
	upstreamRootTreeID, err := upstreamTreeBuilder.WriteTreeFromEntries([]gitinterface.TreeEntry{
		gitinterface.NewEntryBlob("a", blobAID),
		gitinterface.NewEntryBlob("b", blobBID),
	})
	if err != nil {
		t.Fatal(err)
	}
	upstreamCommitID, err := upstreamRepo.Commit(upstreamRootTreeID, "refs/heads/main", "Initial commit\n", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := NewReferenceEntry("refs/heads/main", upstreamCommitID).Commit(upstreamRepo, false); err != nil {
		t.Fatal(err)
	}
	upstreamEntry, err := GetLatestEntry(upstreamRepo)
	if err != nil {
		t.Fatal(err)
	}

	err = PropagateChangesFromUpstreamRepository(downstreamRepo, upstreamRepo, []tuf.PropagationDirective{propagationDetails}, false)
	// TODO: should propagation result in a new local ref?
	assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)

	// Add things to downstreamRepo
	blobAID, err = downstreamRepo.WriteBlob([]byte("a"))
	if err != nil {
		t.Fatal(err)
	}

	blobBID, err = downstreamRepo.WriteBlob([]byte("b"))
	if err != nil {
		t.Fatal(err)
	}

	downstreamTreeBuilder := gitinterface.NewTreeBuilder(downstreamRepo)
	downstreamRootTreeID, err := downstreamTreeBuilder.WriteTreeFromEntries([]gitinterface.TreeEntry{
		gitinterface.NewEntryBlob("a", blobAID),
		gitinterface.NewEntryBlob("foo/b", blobBID),
	})
	if err != nil {
		t.Fatal(err)
	}
	downstreamCommitID, err := downstreamRepo.Commit(downstreamRootTreeID, "refs/heads/main", "Initial commit\n", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := NewReferenceEntry("refs/heads/main", downstreamCommitID).Commit(downstreamRepo, false); err != nil {
		t.Fatal(err)
	}

	err = PropagateChangesFromUpstreamRepository(downstreamRepo, upstreamRepo, []tuf.PropagationDirective{propagationDetails}, false)
	assert.Nil(t, err)

	latestEntry, err := GetLatestEntry(downstreamRepo)
	if err != nil {
		t.Fatal(err)
	}
	propagationEntry, isPropagationEntry := latestEntry.(*PropagationEntry)
	if !isPropagationEntry {
		t.Fatal("unexpected entry type in downstream repo")
	}
	assert.Equal(t, upstreamRepoLocation, propagationEntry.UpstreamRepository)
	assert.Equal(t, upstreamEntry.GetID(), propagationEntry.UpstreamEntryID)

	downstreamRootTreeID, err = downstreamRepo.GetCommitTreeID(propagationEntry.TargetID)
	if err != nil {
		t.Fatal(err)
	}
	pathTreeID, err := downstreamRepo.GetPathIDInTree("upstream", downstreamRootTreeID)
	if err != nil {
		t.Fatal(err)
	}

	// Check the subtree ID in downstream repo matches upstream root tree ID
	assert.Equal(t, upstreamRootTreeID, pathTreeID)

	// Check the downstream tree still contains other items
	expectedRootTreeID, err := downstreamTreeBuilder.WriteTreeFromEntries([]gitinterface.TreeEntry{
		gitinterface.NewEntryBlob("a", blobAID),
		gitinterface.NewEntryBlob("foo/b", blobBID),
		gitinterface.NewEntryBlob("upstream/a", blobAID),
		gitinterface.NewEntryBlob("upstream/b", blobBID),
	})
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, expectedRootTreeID, downstreamRootTreeID)

	// Nothing to propagate, check that a new entry has not been added in the downstreamRepo
	err = PropagateChangesFromUpstreamRepository(downstreamRepo, upstreamRepo, []tuf.PropagationDirective{propagationDetails}, false)
	assert.Nil(t, err)

	latestEntry, err = GetLatestEntry(downstreamRepo)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, propagationEntry.GetID(), latestEntry.GetID())
}

func TestAnnotationEntryRefersTo(t *testing.T) {
	// We use these as stand-ins for actual RSL IDs that have the same data type
	tempDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

	treeBuilder := gitinterface.NewTreeBuilder(repo)
	emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
	if err != nil {
		t.Fatal(err)
	}

	emptyBlobID, err := repo.WriteBlob(nil)
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]struct {
		annotation     *AnnotationEntry
		entryID        gitinterface.Hash
		expectedResult bool
	}{
		"annotation refers to single entry, returns true": {
			annotation:     NewAnnotationEntry([]gitinterface.Hash{emptyBlobID}, false, annotationMessage),
			entryID:        emptyBlobID,
			expectedResult: true,
		},
		"annotation refers to multiple entries, returns true": {
			annotation:     NewAnnotationEntry([]gitinterface.Hash{emptyTreeID, emptyBlobID}, false, annotationMessage),
			entryID:        emptyBlobID,
			expectedResult: true,
		},
		"annotation refers to single entry, returns false": {
			annotation:     NewAnnotationEntry([]gitinterface.Hash{emptyBlobID}, false, annotationMessage),
			entryID:        gitinterface.ZeroHash,
			expectedResult: false,
		},
		"annotation refers to multiple entries, returns false": {
			annotation:     NewAnnotationEntry([]gitinterface.Hash{emptyTreeID, emptyBlobID}, false, annotationMessage),
			entryID:        gitinterface.ZeroHash,
			expectedResult: false,
		},
	}

	for name, test := range tests {
		result := test.annotation.RefersTo(test.entryID)
		assert.Equal(t, test.expectedResult, result, fmt.Sprintf("unexpected result in test '%s'", name))
	}
}

func TestReferenceEntryCreateCommitMessage(t *testing.T) {
	nonZeroHash, err := gitinterface.NewHash("abcdef12345678900987654321fedcbaabcdef12")
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]struct {
		entry           *ReferenceEntry
		expectedMessage string
	}{
		"entry, fully resolved ref": {
			entry: &ReferenceEntry{
				RefName:  "refs/heads/main",
				TargetID: gitinterface.ZeroHash,
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s", ReferenceEntryHeader, RefKey, "refs/heads/main", TargetIDKey, plumbing.ZeroHash.String()),
		},
		"entry, non-zero commit": {
			entry: &ReferenceEntry{
				RefName:  "refs/heads/main",
				TargetID: nonZeroHash,
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s", ReferenceEntryHeader, RefKey, "refs/heads/main", TargetIDKey, "abcdef12345678900987654321fedcbaabcdef12"),
		},
		"entry, fully resolved ref, small number": {
			entry: &ReferenceEntry{
				RefName:  "refs/heads/main",
				TargetID: gitinterface.ZeroHash,
				Number:   1,
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %d", ReferenceEntryHeader, RefKey, "refs/heads/main", TargetIDKey, plumbing.ZeroHash.String(), NumberKey, 1),
		},
		"entry, fully resolved ref, large number": {
			entry: &ReferenceEntry{
				RefName:  "refs/heads/main",
				TargetID: gitinterface.ZeroHash,
				Number:   math.MaxUint64,
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %d", ReferenceEntryHeader, RefKey, "refs/heads/main", TargetIDKey, plumbing.ZeroHash.String(), NumberKey, uint64(math.MaxUint64)),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			message, _ := test.entry.createCommitMessage(true)
			if !assert.Equal(t, test.expectedMessage, message) {
				t.Errorf("expected\n%s\n\ngot\n%s", test.expectedMessage, message)
			}
		})
	}
}

func TestAnnotationEntryCreateCommitMessage(t *testing.T) {
	tests := map[string]struct {
		entry           *AnnotationEntry
		expectedMessage string
	}{
		"annotation, no message": {
			entry: &AnnotationEntry{
				RSLEntryIDs: []gitinterface.Hash{gitinterface.ZeroHash},
				Skip:        true,
				Message:     "",
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s", AnnotationEntryHeader, EntryIDKey, gitinterface.ZeroHash.String(), SkipKey, "true"),
		},
		"annotation, with message": {
			entry: &AnnotationEntry{
				RSLEntryIDs: []gitinterface.Hash{gitinterface.ZeroHash},
				Skip:        true,
				Message:     "message",
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s\n%s\n%s", AnnotationEntryHeader, EntryIDKey, gitinterface.ZeroHash.String(), SkipKey, "true", BeginMessage, base64.StdEncoding.EncodeToString([]byte("message")), EndMessage),
		},
		"annotation, with multi-line message": {
			entry: &AnnotationEntry{
				RSLEntryIDs: []gitinterface.Hash{gitinterface.ZeroHash},
				Skip:        true,
				Message:     "message1\nmessage2",
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s\n%s\n%s", AnnotationEntryHeader, EntryIDKey, gitinterface.ZeroHash.String(), SkipKey, "true", BeginMessage, base64.StdEncoding.EncodeToString([]byte("message1\nmessage2")), EndMessage),
		},
		"annotation, no message, skip false": {
			entry: &AnnotationEntry{
				RSLEntryIDs: []gitinterface.Hash{gitinterface.ZeroHash},
				Skip:        false,
				Message:     "",
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s", AnnotationEntryHeader, EntryIDKey, gitinterface.ZeroHash.String(), SkipKey, "false"),
		},
		"annotation, no message, skip false, multiple entry IDs": {
			entry: &AnnotationEntry{
				RSLEntryIDs: []gitinterface.Hash{gitinterface.ZeroHash, gitinterface.ZeroHash},
				Skip:        false,
				Message:     "",
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s", AnnotationEntryHeader, EntryIDKey, gitinterface.ZeroHash.String(), EntryIDKey, gitinterface.ZeroHash.String(), SkipKey, "false"),
		},
		"annotation, no message, small number": {
			entry: &AnnotationEntry{
				RSLEntryIDs: []gitinterface.Hash{gitinterface.ZeroHash},
				Skip:        true,
				Message:     "",
				Number:      1,
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %d", AnnotationEntryHeader, EntryIDKey, gitinterface.ZeroHash.String(), SkipKey, "true", NumberKey, 1),
		},
		"annotation, no message, large number": {
			entry: &AnnotationEntry{
				RSLEntryIDs: []gitinterface.Hash{gitinterface.ZeroHash},
				Skip:        true,
				Message:     "",
				Number:      math.MaxUint64,
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %d", AnnotationEntryHeader, EntryIDKey, gitinterface.ZeroHash.String(), SkipKey, "true", NumberKey, uint64(math.MaxUint64)),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			message, err := test.entry.createCommitMessage(true)
			if err != nil {
				t.Fatal(err)
			}
			if !assert.Equal(t, test.expectedMessage, message) {
				t.Errorf("expected\n%s\n\ngot\n%s", test.expectedMessage, message)
			}
		})
	}
}

func TestPropagationEntryCreateCommitMessage(t *testing.T) {
	nonZeroHash, err := gitinterface.NewHash("abcdef12345678900987654321fedcbaabcdef12")
	if err != nil {
		t.Fatal(err)
	}

	upstreamRepository := "https://git.example.com/example/repository"

	tests := map[string]struct {
		entry           *PropagationEntry
		expectedMessage string
	}{
		"entry, fully resolved ref": {
			entry: &PropagationEntry{
				RefName:            "refs/heads/main",
				TargetID:           gitinterface.ZeroHash,
				UpstreamRepository: upstreamRepository,
				UpstreamEntryID:    gitinterface.ZeroHash,
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s\n%s: %s", PropagationEntryHeader, RefKey, "refs/heads/main", TargetIDKey, gitinterface.ZeroHash.String(), UpstreamRepositoryKey, upstreamRepository, UpstreamEntryIDKey, gitinterface.ZeroHash.String()),
		},
		"entry, non-zero commit": {
			entry: &PropagationEntry{
				RefName:            "refs/heads/main",
				TargetID:           nonZeroHash,
				UpstreamRepository: upstreamRepository,
				UpstreamEntryID:    nonZeroHash,
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s\n%s: %s", PropagationEntryHeader, RefKey, "refs/heads/main", TargetIDKey, "abcdef12345678900987654321fedcbaabcdef12", UpstreamRepositoryKey, upstreamRepository, UpstreamEntryIDKey, "abcdef12345678900987654321fedcbaabcdef12"),
		},
		"entry, fully resolved ref, small number": {
			entry: &PropagationEntry{
				RefName:            "refs/heads/main",
				TargetID:           gitinterface.ZeroHash,
				UpstreamRepository: upstreamRepository,
				UpstreamEntryID:    gitinterface.ZeroHash,
				Number:             1,
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s\n%s: %s\n%s: %d", PropagationEntryHeader, RefKey, "refs/heads/main", TargetIDKey, gitinterface.ZeroHash.String(), UpstreamRepositoryKey, upstreamRepository, UpstreamEntryIDKey, gitinterface.ZeroHash.String(), NumberKey, 1),
		},
		"entry, fully resolved ref, large number": {
			entry: &PropagationEntry{
				RefName:            "refs/heads/main",
				TargetID:           gitinterface.ZeroHash,
				UpstreamRepository: upstreamRepository,
				UpstreamEntryID:    gitinterface.ZeroHash,
				Number:             math.MaxUint64,
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s\n%s: %s\n%s: %d", PropagationEntryHeader, RefKey, "refs/heads/main", TargetIDKey, gitinterface.ZeroHash.String(), UpstreamRepositoryKey, upstreamRepository, UpstreamEntryIDKey, gitinterface.ZeroHash.String(), NumberKey, uint64(math.MaxUint64)),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			message, _ := test.entry.createCommitMessage(true)
			if !assert.Equal(t, test.expectedMessage, message) {
				t.Errorf("expected\n%s\n\ngot\n%s", test.expectedMessage, message)
			}
		})
	}
}

func TestParseRSLEntryText(t *testing.T) {
	nonZeroHash, err := gitinterface.NewHash("abcdef12345678900987654321fedcbaabcdef12")
	if err != nil {
		t.Fatal(err)
	}

	upstreamRepository := "https://git.example.com/example/repository"

	tests := map[string]struct {
		expectedEntry Entry
		expectedError error
		message       string
	}{
		"entry, fully resolved ref": {
			expectedEntry: &ReferenceEntry{
				ID:       gitinterface.ZeroHash,
				RefName:  "refs/heads/main",
				TargetID: gitinterface.ZeroHash,
			},
			message: fmt.Sprintf("%s\n\n%s: %s\n%s: %s", ReferenceEntryHeader, RefKey, "refs/heads/main", TargetIDKey, gitinterface.ZeroHash.String()),
		},
		"entry, non-zero commit": {
			expectedEntry: &ReferenceEntry{
				ID:       gitinterface.ZeroHash,
				RefName:  "refs/heads/main",
				TargetID: nonZeroHash,
			},
			message: fmt.Sprintf("%s\n\n%s: %s\n%s: %s", ReferenceEntryHeader, RefKey, "refs/heads/main", TargetIDKey, "abcdef12345678900987654321fedcbaabcdef12"),
		},
		"entry, missing header": {
			expectedError: ErrInvalidRSLEntry,
			message:       fmt.Sprintf("%s: %s\n%s: %s", RefKey, "refs/heads/main", TargetIDKey, gitinterface.ZeroHash.String()),
		},
		"entry, missing information": {
			expectedError: ErrInvalidRSLEntry,
			message:       fmt.Sprintf("%s\n\n%s: %s", ReferenceEntryHeader, RefKey, "refs/heads/main"),
		},
		"annotation, no message": {
			expectedEntry: &AnnotationEntry{
				ID:          gitinterface.ZeroHash,
				RSLEntryIDs: []gitinterface.Hash{gitinterface.ZeroHash},
				Skip:        true,
				Message:     "",
			},
			message: fmt.Sprintf("%s\n\n%s: %s\n%s: %s", AnnotationEntryHeader, EntryIDKey, gitinterface.ZeroHash.String(), SkipKey, "true"),
		},
		"annotation, with message": {
			expectedEntry: &AnnotationEntry{
				ID:          gitinterface.ZeroHash,
				RSLEntryIDs: []gitinterface.Hash{gitinterface.ZeroHash},
				Skip:        true,
				Message:     "message",
			},
			message: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s\n%s\n%s", AnnotationEntryHeader, EntryIDKey, gitinterface.ZeroHash.String(), SkipKey, "true", BeginMessage, base64.StdEncoding.EncodeToString([]byte("message")), EndMessage),
		},
		"annotation, with multi-line message": {
			expectedEntry: &AnnotationEntry{
				ID:          gitinterface.ZeroHash,
				RSLEntryIDs: []gitinterface.Hash{gitinterface.ZeroHash},
				Skip:        true,
				Message:     "message1\nmessage2",
			},
			message: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s\n%s\n%s", AnnotationEntryHeader, EntryIDKey, gitinterface.ZeroHash.String(), SkipKey, "true", BeginMessage, base64.StdEncoding.EncodeToString([]byte("message1\nmessage2")), EndMessage),
		},
		"annotation, no message, skip false": {
			expectedEntry: &AnnotationEntry{
				ID:          gitinterface.ZeroHash,
				RSLEntryIDs: []gitinterface.Hash{gitinterface.ZeroHash},
				Skip:        false,
				Message:     "",
			},
			message: fmt.Sprintf("%s\n\n%s: %s\n%s: %s", AnnotationEntryHeader, EntryIDKey, gitinterface.ZeroHash.String(), SkipKey, "false"),
		},
		"annotation, no message, skip false, multiple entry IDs": {
			expectedEntry: &AnnotationEntry{
				ID:          gitinterface.ZeroHash,
				RSLEntryIDs: []gitinterface.Hash{gitinterface.ZeroHash, gitinterface.ZeroHash},
				Skip:        false,
				Message:     "",
			},
			message: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s", AnnotationEntryHeader, EntryIDKey, gitinterface.ZeroHash.String(), EntryIDKey, gitinterface.ZeroHash.String(), SkipKey, "false"),
		},
		"annotation, missing header": {
			expectedError: ErrInvalidRSLEntry,
			message:       fmt.Sprintf("%s: %s\n%s: %s\n%s\n%s\n%s", EntryIDKey, gitinterface.ZeroHash.String(), SkipKey, "true", BeginMessage, base64.StdEncoding.EncodeToString([]byte("message")), EndMessage),
		},
		"annotation, missing information": {
			expectedError: ErrInvalidRSLEntry,
			message:       fmt.Sprintf("%s\n\n%s: %s", AnnotationEntryHeader, EntryIDKey, gitinterface.ZeroHash.String()),
		},
		"propagation entry, fully resolved ref": {
			expectedEntry: &PropagationEntry{
				ID:                 gitinterface.ZeroHash,
				RefName:            "refs/heads/main",
				TargetID:           gitinterface.ZeroHash,
				UpstreamRepository: upstreamRepository,
				UpstreamEntryID:    gitinterface.ZeroHash,
			},
			message: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s\n%s: %s", PropagationEntryHeader, RefKey, "refs/heads/main", TargetIDKey, gitinterface.ZeroHash.String(), UpstreamRepositoryKey, upstreamRepository, UpstreamEntryIDKey, gitinterface.ZeroHash.String()),
		},
		"propagation entry, non-zero commit": {
			expectedEntry: &PropagationEntry{
				ID:                 gitinterface.ZeroHash,
				RefName:            "refs/heads/main",
				TargetID:           nonZeroHash,
				UpstreamRepository: upstreamRepository,
				UpstreamEntryID:    nonZeroHash,
			},
			message: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s\n%s: %s", PropagationEntryHeader, RefKey, "refs/heads/main", TargetIDKey, "abcdef12345678900987654321fedcbaabcdef12", UpstreamRepositoryKey, upstreamRepository, UpstreamEntryIDKey, "abcdef12345678900987654321fedcbaabcdef12"),
		},
		"propagation entry, missing information": {
			expectedError: ErrInvalidRSLEntry,
			message:       fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s", PropagationEntryHeader, RefKey, "refs/heads/main", TargetIDKey, "abcdef12345678900987654321fedcbaabcdef12", UpstreamRepositoryKey, upstreamRepository),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			entry, err := parseRSLEntryText(gitinterface.ZeroHash, test.message)
			if err != nil {
				assert.ErrorIs(t, err, test.expectedError)
			} else if !assert.Equal(t, test.expectedEntry, entry) {
				t.Errorf("expected\n%+v\n\ngot\n%+v", test.expectedEntry, entry)
			}
		})
	}
}

func assertAnnotationsReferToEntry(t *testing.T, entry ReferenceUpdaterEntry, annotations []*AnnotationEntry) {
	t.Helper()

	if entry == nil || annotations == nil {
		t.Error("expected entry and annotations, received nil")
	}

	for _, annotation := range annotations {
		assert.True(t, annotation.RefersTo(entry.GetID()))
		assert.Equal(t, annotationMessage, annotation.Message)
	}
}
