package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/stretchr/testify/assert"
)

func TestLoadRepository(t *testing.T) {
	repository, err := LoadRepository()
	assert.Nil(t, err)
	assert.NotNil(t, repository.r)
}

func TestInitializeNamespaces(t *testing.T) {
	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	r := &Repository{r: repo}
	err = r.InitializeNamespaces()
	assert.Nil(t, err)
}

func TestUnauthorizedKey(t *testing.T) {
	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	r := &Repository{r: repo}
	keyBytes, err := os.ReadFile(filepath.Join("test-data", "root"))
	if err != nil {
		t.Fatal(err)
	}

	if err := r.InitializeRoot(context.Background(), keyBytes, false); err != nil {
		t.Fatal(err)
	}

	targetsKeyBytes, err := os.ReadFile(filepath.Join("test-data", "targets"))
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test add targets key", func(t *testing.T) {
		err := r.AddTopLevelTargetsKey(context.Background(), targetsKeyBytes, targetsKeyBytes, false)
		assert.ErrorIs(t, err, ErrUnauthorizedKey)
	})

	t.Run("test remove targets key", func(t *testing.T) {
		err := r.RemoveTopLevelTargetsKey(context.Background(), targetsKeyBytes, "some key ID", false)
		assert.ErrorIs(t, err, ErrUnauthorizedKey)
	})
}

func createTestRepositoryWithRoot(t *testing.T) (*Repository, []byte) {
	t.Helper()

	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	r := &Repository{r: repo}
	keyBytes, err := os.ReadFile(filepath.Join("test-data", "root"))
	if err != nil {
		t.Fatal(err)
	}

	if err := r.InitializeRoot(context.Background(), keyBytes, false); err != nil {
		t.Fatal(err)
	}

	return r, keyBytes
}
