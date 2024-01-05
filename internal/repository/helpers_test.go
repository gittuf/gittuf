// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	_ "embed"
	"encoding/json"
	"testing"

	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/memory"
)

//go:embed test-data/gpg-pubkey.asc
var gpgPubKeyBytes []byte

//go:embed test-data/gpg-privkey.asc
var gpgKeyBytes []byte

//go:embed test-data/root
var rootKeyBytes []byte

//go:embed test-data/targets
var targetsKeyBytes []byte

//go:embed test-data/targets.pub
var targetsPubKeyBytes []byte

//go:embed test-data/rsa-ssh-key
var rsaKeyBytes []byte

//go:embed test-data/ecdsa-ssh-key
var ecdsaKeyBytes []byte

var testCtx = context.Background()

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

	if err := r.InitializeRoot(testCtx, rootKeyBytes, false); err != nil {
		t.Fatal(err)
	}

	return r, rootKeyBytes
}

func createTestRepositoryWithPolicy(t *testing.T, location string) *Repository {
	t.Helper()

	r, keyBytes := createTestRepositoryWithRoot(t, location)

	if err := r.AddTopLevelTargetsKey(testCtx, keyBytes, targetsPubKeyBytes, false); err != nil {
		t.Fatal(err)
	}

	if err := r.InitializeTargets(testCtx, targetsKeyBytes, policy.TargetsRoleName, false); err != nil {
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

	if err := r.AddDelegation(testCtx, targetsKeyBytes, policy.TargetsRoleName, "protect-main", authorizedKeys, []string{"git:refs/heads/main"}, false); err != nil {
		t.Fatal(err)
	}

	return r
}
