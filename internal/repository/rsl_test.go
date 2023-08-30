package repository

import (
	"fmt"
	"testing"

	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/stretchr/testify/assert"
)

func TestRecordRSLEntryForReference(t *testing.T) {
	r, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	repo := &Repository{r: r}

	if err := rsl.InitializeNamespace(repo.r); err != nil {
		t.Fatal(err)
	}

	ref := plumbing.NewHashReference(plumbing.ReferenceName("refs/heads/main"), plumbing.ZeroHash)

	if err := repo.r.Storer.SetReference(ref); err != nil {
		t.Fatal(err)
	}

	if err := repo.RecordRSLEntryForReference("refs/heads/main", false); err != nil {
		t.Fatal(err)
	}

	rslRef, err := repo.r.Reference(rsl.RSLRef, true)
	if err != nil {
		t.Fatal(err)
	}

	entryType, err := rsl.GetEntry(repo.r, rslRef.Hash())
	if err != nil {
		t.Fatal(err)
	}

	entry, ok := entryType.(*rsl.Entry)
	if !ok {
		t.Fatal(fmt.Errorf("invalid entry type"))
	}
	assert.Equal(t, "refs/heads/main", entry.RefName)
	assert.Equal(t, plumbing.ZeroHash, entry.CommitID)

	testHash := plumbing.NewHash("abcdef1234567890")

	ref = plumbing.NewHashReference(plumbing.ReferenceName("refs/heads/main"), testHash)
	if err := repo.r.Storer.SetReference(ref); err != nil {
		t.Fatal(err)
	}

	if err := repo.RecordRSLEntryForReference("main", false); err != nil {
		t.Fatal(err)
	}

	rslRef, err = repo.r.Reference(rsl.RSLRef, true)
	if err != nil {
		t.Fatal(err)
	}

	entryType, err = rsl.GetEntry(repo.r, rslRef.Hash())
	if err != nil {
		t.Fatal(err)
	}

	entry, ok = entryType.(*rsl.Entry)
	if !ok {
		t.Fatal(fmt.Errorf("invalid entry type"))
	}
	assert.Equal(t, "refs/heads/main", entry.RefName)
	assert.Equal(t, testHash, entry.CommitID)
}

func TestRecordRSLAnnotation(t *testing.T) {
	r, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	repo := &Repository{r: r}

	if err := rsl.InitializeNamespace(repo.r); err != nil {
		t.Fatal(err)
	}

	ref := plumbing.NewHashReference(plumbing.ReferenceName("refs/heads/main"), plumbing.ZeroHash)

	if err := repo.r.Storer.SetReference(ref); err != nil {
		t.Fatal(err)
	}

	err = repo.RecordRSLAnnotation([]string{plumbing.ZeroHash.String()}, false, "test annotation", false)
	assert.ErrorIs(t, err, rsl.ErrRSLEntryNotFound)

	if err := repo.RecordRSLEntryForReference("refs/heads/main", false); err != nil {
		t.Fatal(err)
	}

	latestEntry, err := rsl.GetLatestEntry(repo.r)
	if err != nil {
		t.Fatal(err)
	}
	entryID := latestEntry.GetID()

	err = repo.RecordRSLAnnotation([]string{entryID.String()}, false, "test annotation", false)
	assert.Nil(t, err)

	latestEntry, err = rsl.GetLatestEntry(repo.r)
	if err != nil {
		t.Fatal(err)
	}
	assert.IsType(t, &rsl.Annotation{}, latestEntry)

	annotation := latestEntry.(*rsl.Annotation)
	assert.Equal(t, "test annotation", annotation.Message)
	assert.Equal(t, []plumbing.Hash{entryID}, annotation.RSLEntryIDs)
	assert.False(t, annotation.Skip)

	err = repo.RecordRSLAnnotation([]string{entryID.String()}, true, "skip annotation", false)
	assert.Nil(t, err)

	latestEntry, err = rsl.GetLatestEntry(repo.r)
	if err != nil {
		t.Fatal(err)
	}
	assert.IsType(t, &rsl.Annotation{}, latestEntry)

	annotation = latestEntry.(*rsl.Annotation)
	assert.Equal(t, "skip annotation", annotation.Message)
	assert.Equal(t, []plumbing.Hash{entryID}, annotation.RSLEntryIDs)
	assert.True(t, annotation.Skip)
}
