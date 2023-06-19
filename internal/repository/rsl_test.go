package repository

import (
	"fmt"
	"testing"

	"github.com/adityasaky/gittuf/internal/rsl"
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
