// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"testing"

	"github.com/gittuf/gittuf/internal/attestations"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/signerverifier"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/stretchr/testify/assert"
)

func TestAddReferenceAuthorization(t *testing.T) {
	t.Setenv(dev.DevModeKey, "1")

	refName := "main"
	absRefName := "refs/heads/main"
	commitID := plumbing.ZeroHash.String()

	r, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(absRefName), plumbing.ZeroHash)); err != nil {
		t.Fatal(err)
	}

	repo := &Repository{r: r}

	rootSigner, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(rootKeyBytes) //nolint:staticcheck
	if err != nil {
		t.Fatal(err)
	}

	targetsSigner, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(targetsKeyBytes) //nolint:staticcheck
	if err != nil {
		t.Fatal(err)
	}

	if err := repo.InitializeNamespaces(); err != nil {
		t.Fatal(err)
	}

	if err := repo.RecordRSLEntryForReference(absRefName, false); err != nil {
		t.Fatal(err)
	}

	err = repo.AddReferenceAuthorization(context.Background(), rootSigner, absRefName, false)
	assert.Nil(t, err)

	allAttestations, err := attestations.LoadCurrentAttestations(repo.r)
	if err != nil {
		t.Fatal(err)
	}

	env, err := allAttestations.GetReferenceAuthorizationFor(repo.r, absRefName, commitID, commitID)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(env.Signatures))

	// Add authorization using the short ref name and different signer
	err = repo.AddReferenceAuthorization(context.Background(), targetsSigner, refName, false)
	assert.Nil(t, err)

	allAttestations, err = attestations.LoadCurrentAttestations(repo.r)
	if err != nil {
		t.Fatal(err)
	}

	env, err = allAttestations.GetReferenceAuthorizationFor(repo.r, absRefName, commitID, commitID)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 2, len(env.Signatures))
}

func TestRemoveReferenceAuthorization(t *testing.T) {
	t.Setenv(dev.DevModeKey, "1")

	absRefName := "refs/heads/main"
	commitID := plumbing.ZeroHash.String()

	rootSigner, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(rootKeyBytes) //nolint:staticcheck
	if err != nil {
		t.Fatal(err)
	}

	r, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(absRefName), plumbing.ZeroHash)); err != nil {
		t.Fatal(err)
	}

	repo := &Repository{r: r}

	if err := repo.InitializeNamespaces(); err != nil {
		t.Fatal(err)
	}

	if err := repo.RecordRSLEntryForReference(absRefName, false); err != nil {
		t.Fatal(err)
	}

	err = repo.AddReferenceAuthorization(context.Background(), rootSigner, absRefName, false)
	assert.Nil(t, err)

	allAttestations, err := attestations.LoadCurrentAttestations(repo.r)
	if err != nil {
		t.Fatal(err)
	}

	env, err := allAttestations.GetReferenceAuthorizationFor(repo.r, absRefName, commitID, commitID)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(env.Signatures))

	err = repo.RemoveReferenceAuthorization(context.Background(), rootSigner, absRefName, commitID, commitID, false)
	assert.Nil(t, err)

	allAttestations, err = attestations.LoadCurrentAttestations(repo.r)
	if err != nil {
		t.Fatal(err)
	}

	env, err = allAttestations.GetReferenceAuthorizationFor(repo.r, absRefName, commitID, commitID)
	assert.ErrorIs(t, err, attestations.ErrAuthorizationNotFound)
	assert.Nil(t, env)
}
