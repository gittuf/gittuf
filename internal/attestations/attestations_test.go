// SPDX-License-Identifier: Apache-2.0

package attestations

import (
	"encoding/json"
	"testing"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/third_party/go-git"
	"github.com/gittuf/gittuf/internal/third_party/go-git/plumbing"
	"github.com/gittuf/gittuf/internal/third_party/go-git/storage/memory"
	"github.com/go-git/go-billy/v5/memfs"
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

		ref, err := repo.Reference(plumbing.ReferenceName(Ref), true)
		if err != nil {
			t.Fatal(err)
		}

		initialCommit, err := gitinterface.GetCommit(repo, ref.Hash())
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, initialCommitMessage, initialCommit.Message)
		assert.Equal(t, gitinterface.EmptyTree(), initialCommit.TreeHash)
	})

	t.Run("existing attestations namespace", func(t *testing.T) {
		repo, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}

		if err := InitializeNamespace(repo); err != nil {
			t.Fatal(err)
		}

		err = InitializeNamespace(repo)
		assert.ErrorIs(t, err, ErrAttestationsExist)
	})
}

func TestLoadCurrentAttestations(t *testing.T) {
	testRef := "refs/heads/main"
	testID := plumbing.ZeroHash.String()
	testAttestation, err := NewAuthorizationAttestation(testRef, testID, testID)
	if err != nil {
		t.Fatal(err)
	}
	testEnv, err := dsse.CreateEnvelope(testAttestation)
	if err != nil {
		t.Fatal(err)
	}
	testEnvBytes, err := json.Marshal(testEnv)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("no RSL entry", func(t *testing.T) {
		repo, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}

		if err := rsl.InitializeNamespace(repo); err != nil {
			t.Fatal(err)
		}

		attestations, err := LoadCurrentAttestations(repo)
		assert.Nil(t, err)
		assert.Empty(t, attestations.authorizations)
	})

	t.Run("with RSL entry but empty", func(t *testing.T) {
		repo, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}

		if err := rsl.InitializeNamespace(repo); err != nil {
			t.Fatal(err)
		}

		if err := InitializeNamespace(repo); err != nil {
			t.Fatal(err)
		}

		ref, err := repo.Reference(plumbing.ReferenceName(Ref), true)
		if err != nil {
			t.Fatal(err)
		}

		if err := rsl.NewReferenceEntry(Ref, ref.Hash()).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		attestations, err := LoadCurrentAttestations(repo)
		assert.Nil(t, err)
		assert.Empty(t, attestations.authorizations)
	})

	t.Run("with RSL entry and with an attestation", func(t *testing.T) {
		repo, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}

		if err := rsl.InitializeNamespace(repo); err != nil {
			t.Fatal(err)
		}

		if err := InitializeNamespace(repo); err != nil {
			t.Fatal(err)
		}

		blobID, err := gitinterface.WriteBlob(repo, testEnvBytes)
		if err != nil {
			t.Fatal(err)
		}

		authorizations := map[string]plumbing.Hash{AuthorizationPath(testRef, testID, testID): blobID}

		attestations := &Attestations{authorizations: authorizations}
		if err := attestations.Commit(repo, "Test commit", false); err != nil {
			t.Fatal(err)
		}

		ref, err := repo.Reference(plumbing.ReferenceName(Ref), true)
		if err != nil {
			t.Fatal(err)
		}

		if err := rsl.NewReferenceEntry(Ref, ref.Hash()).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		attestations, err = LoadCurrentAttestations(repo)
		assert.Nil(t, err)
		assert.Equal(t, authorizations, attestations.authorizations)
	})
}

func TestLoadAttestationsForEntry(t *testing.T) {
	testRef := "refs/heads/main"
	testID := plumbing.ZeroHash.String()
	testAttestation, err := NewAuthorizationAttestation(testRef, testID, testID)
	if err != nil {
		t.Fatal(err)
	}
	testEnv, err := dsse.CreateEnvelope(testAttestation)
	if err != nil {
		t.Fatal(err)
	}
	testEnvBytes, err := json.Marshal(testEnv)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("with RSL entry but empty", func(t *testing.T) {
		repo, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}

		if err := rsl.InitializeNamespace(repo); err != nil {
			t.Fatal(err)
		}

		if err := InitializeNamespace(repo); err != nil {
			t.Fatal(err)
		}

		ref, err := repo.Reference(plumbing.ReferenceName(Ref), true)
		if err != nil {
			t.Fatal(err)
		}

		if err := rsl.NewReferenceEntry(Ref, ref.Hash()).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		entry, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		attestations, err := LoadAttestationsForEntry(repo, entry.(*rsl.ReferenceEntry))
		assert.Nil(t, err)
		assert.Empty(t, attestations.authorizations)
	})

	t.Run("with RSL entry and with an attestation", func(t *testing.T) {
		repo, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}

		if err := rsl.InitializeNamespace(repo); err != nil {
			t.Fatal(err)
		}

		if err := InitializeNamespace(repo); err != nil {
			t.Fatal(err)
		}

		blobID, err := gitinterface.WriteBlob(repo, testEnvBytes)
		if err != nil {
			t.Fatal(err)
		}

		authorizations := map[string]plumbing.Hash{AuthorizationPath(testRef, testID, testID): blobID}

		attestations := &Attestations{authorizations: authorizations}
		if err := attestations.Commit(repo, "Test commit", false); err != nil {
			t.Fatal(err)
		}

		ref, err := repo.Reference(plumbing.ReferenceName(Ref), true)
		if err != nil {
			t.Fatal(err)
		}

		if err := rsl.NewReferenceEntry(Ref, ref.Hash()).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		entry, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		attestations, err = LoadAttestationsForEntry(repo, entry.(*rsl.ReferenceEntry))
		assert.Nil(t, err)
		assert.Equal(t, authorizations, attestations.authorizations)
	})
}

func TestAttestationsCommit(t *testing.T) {
	testRef := "refs/heads/main"
	testID := plumbing.ZeroHash.String()
	testAttestation, err := NewAuthorizationAttestation(testRef, testID, testID)
	if err != nil {
		t.Fatal(err)
	}
	testEnv, err := dsse.CreateEnvelope(testAttestation)
	if err != nil {
		t.Fatal(err)
	}
	testEnvBytes, err := json.Marshal(testEnv)
	if err != nil {
		t.Fatal(err)
	}

	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	blobID, err := gitinterface.WriteBlob(repo, testEnvBytes)
	if err != nil {
		t.Fatal(err)
	}

	if err := InitializeNamespace(repo); err != nil {
		t.Fatal(err)
	}

	ref, err := repo.Reference(plumbing.ReferenceName(Ref), true)
	if err != nil {
		t.Fatal(err)
	}
	commit, err := gitinterface.GetCommit(repo, ref.Hash())
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, gitinterface.EmptyTree(), commit.TreeHash)

	authorizations := map[string]plumbing.Hash{AuthorizationPath(testRef, testID, testID): blobID}
	attestations := &Attestations{authorizations: authorizations}

	if err := attestations.Commit(repo, "Test commit", false); err != nil {
		t.Error(err)
	}

	ref, err = repo.Reference(plumbing.ReferenceName(Ref), true)
	if err != nil {
		t.Fatal(err)
	}
	commit, err = gitinterface.GetCommit(repo, ref.Hash())
	if err != nil {
		t.Fatal(err)
	}
	assert.NotEqual(t, gitinterface.EmptyTree(), commit.TreeHash)

	rootTree, err := gitinterface.GetTree(repo, commit.TreeHash)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(rootTree.Entries))
	assert.Equal(t, authorizationsTreeEntryName, rootTree.Entries[0].Name)

	// We don't need to check every level of the tree because we do it in the
	// tree builder API
	attestations, err = LoadCurrentAttestations(repo)
	assert.Nil(t, err)
	assert.Equal(t, attestations.authorizations, authorizations)
}
