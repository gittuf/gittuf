// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/gittuf/gittuf/internal/third_party/go-git"
	"github.com/gittuf/gittuf/internal/third_party/go-git/storage/memory"
	"github.com/go-git/go-billy/v5/memfs"
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

func createTestRepositoryWithRoot(t *testing.T, location string) (*Repository, []byte) {
	t.Helper()

	var (
		repo *git.Repository
		err  error
	)
	if location == "" {
		repo, err = git.Init(memory.NewStorage(), memfs.New())
	} else {
		repo, err = git.PlainInit(location, true)
	}
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

func createTestRepositoryWithPolicy(t *testing.T, location string) *Repository {
	t.Helper()

	r, keyBytes := createTestRepositoryWithRoot(t, location)

	targetsPrivKeyBytes, err := os.ReadFile(filepath.Join("test-data", "targets"))
	if err != nil {
		t.Fatal(err)
	}
	targetsPubKeyBytes, err := os.ReadFile(filepath.Join("test-data", "targets.pub"))
	if err != nil {
		t.Fatal(err)
	}

	if err := r.AddTopLevelTargetsKey(context.Background(), keyBytes, targetsPubKeyBytes, false); err != nil {
		t.Fatal(err)
	}

	if err := r.InitializeTargets(context.Background(), targetsPrivKeyBytes, policy.TargetsRoleName, false); err != nil {
		t.Fatal(err)
	}

	gpgKeyBytes, err := os.ReadFile(filepath.Join("test-data", "gpg-pubkey.asc"))
	if err != nil {
		t.Fatal(err)
	}
	gpgKey, err := gpg.LoadGPGKeyFromBytes(gpgKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	kb, err := json.Marshal(gpgKey)
	if err != nil {
		t.Fatal(err)
	}
	authorizedKeys := [][]byte{kb}

	if err := r.AddDelegation(context.Background(), targetsPrivKeyBytes, policy.TargetsRoleName, "protect-main", authorizedKeys, []string{"git:refs/heads/main"}, false); err != nil {
		t.Fatal(err)
	}

	return r
}
