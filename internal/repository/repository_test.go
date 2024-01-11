// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"testing"

	"github.com/gittuf/gittuf/internal/tuf"
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
	if err := r.InitializeRoot(context.Background(), rootKeyBytes, false); err != nil {
		t.Fatal(err)
	}

	t.Run("test add targets key", func(t *testing.T) {
		key, err := tuf.LoadKeyFromBytes(targetsPubKeyBytes)
		if err != nil {
			t.Fatal(err)
		}

		err = r.AddTopLevelTargetsKey(context.Background(), targetsKeyBytes, key, false)
		assert.ErrorIs(t, err, ErrUnauthorizedKey)
	})

	t.Run("test remove targets key", func(t *testing.T) {
		err := r.RemoveTopLevelTargetsKey(context.Background(), targetsKeyBytes, "some key ID", false)
		assert.ErrorIs(t, err, ErrUnauthorizedKey)
	})
}
