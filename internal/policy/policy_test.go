// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/memory"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/signerverifier"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/go-git/go-billy/v5/memfs"
	sslibdsse "github.com/secure-systems-lab/go-securesystemslib/dsse"
	sslibsv "github.com/secure-systems-lab/go-securesystemslib/signerverifier"
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

		ref, err := repo.Reference(plumbing.ReferenceName(PolicyRef), true)
		assert.Nil(t, err)
		assert.Equal(t, plumbing.ZeroHash, ref.Hash())

		// Disable PolicyStagingRef until it is actually used
		// https://github.com/gittuf/gittuf/issues/45
		// ref, err = repo.Reference(plumbing.ReferenceName(PolicyStagingRef), true)
		// assert.Nil(t, err)
		// assert.Equal(t, plumbing.ZeroHash, ref.Hash())
	})

	t.Run("existing Policy namespace", func(t *testing.T) {
		repo, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}

		if err := InitializeNamespace(repo); err != nil {
			t.Fatal(err)
		}

		// Check if policy with zero hash is treated as uninitialized
		err = InitializeNamespace(repo)
		assert.Nil(t, err)

		if err := repo.Storer.SetReference(plumbing.NewHashReference(PolicyRef, gitinterface.EmptyBlob())); err != nil {
			t.Fatal(err)
		}

		// Now with something added, validate that we cannot initialize the policy again
		err = InitializeNamespace(repo)
		assert.ErrorIs(t, err, ErrPolicyExists)
	})
}

func TestLoadState(t *testing.T) {
	repo, state := createTestRepository(t, createTestStateWithOnlyRoot)

	rslRef, err := repo.Reference(plumbing.ReferenceName(rsl.Ref), true)
	if err != nil {
		t.Fatal(err)
	}

	loadedState, err := LoadState(context.Background(), repo, rslRef.Hash())
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, state, loadedState)
}

func TestLoadCurrentState(t *testing.T) {
	repo, state := createTestRepository(t, createTestStateWithOnlyRoot)

	loadedState, err := LoadCurrentState(context.Background(), repo)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, state, loadedState)
}

func TestLoadStateForEntry(t *testing.T) {
	repo, state := createTestRepository(t, createTestStateWithOnlyRoot)

	entry, _, err := rsl.GetLatestReferenceEntryForRef(repo, PolicyRef)
	if err != nil {
		t.Fatal(err)
	}

	loadedState, err := LoadStateForEntry(context.Background(), repo, entry)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, state, loadedState)
}

func TestStateKeys(t *testing.T) {
	state := createTestStateWithPolicy(t)

	expectedKeys := map[string]*tuf.Key{}
	rootKeyBytes, err := os.ReadFile(filepath.Join("test-data", "root.pub"))
	if err != nil {
		t.Fatal(err)
	}
	rootKey, err := tuf.LoadKeyFromBytes(rootKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	expectedKeys[rootKey.KeyID] = rootKey

	gpgKeyBytes, err := os.ReadFile(filepath.Join("test-data", "gpg-pubkey.asc"))
	if err != nil {
		t.Fatal(err)
	}
	gpgKey, err := gpg.LoadGPGKeyFromBytes(gpgKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	expectedKeys[gpgKey.KeyID] = gpgKey

	keys, err := state.PublicKeys()
	assert.Nil(t, err, keys)
	assert.Equal(t, expectedKeys, keys)
}

func TestStateVerify(t *testing.T) {
	state := createTestStateWithOnlyRoot(t)

	if err := state.Verify(context.Background()); err != nil {
		t.Error(err)
	}

	rootKeys := []*tuf.Key{}
	copy(rootKeys, state.RootPublicKeys)
	state.RootPublicKeys = []*tuf.Key{}

	err := state.Verify(context.Background())
	assert.NotNil(t, err)

	state.RootPublicKeys = rootKeys
	state.RootEnvelope.Signatures = []sslibdsse.Signature{}
	err = state.Verify(context.Background())
	assert.NotNil(t, err)
}

func TestStateCommit(t *testing.T) {
	repo, _ := createTestRepository(t, createTestStateWithOnlyRoot)

	policyRef, err := repo.Reference(plumbing.ReferenceName(PolicyRef), true)
	if err != nil {
		t.Error(err)
	}
	assert.NotEqual(t, plumbing.ZeroHash, policyRef.Hash())

	rslRef, err := repo.Reference(plumbing.ReferenceName(rsl.Ref), true)
	if err != nil {
		t.Error(err)
	}
	assert.NotEqual(t, plumbing.ZeroHash, rslRef.Hash())

	tmpEntry, err := rsl.GetEntry(repo, rslRef.Hash())
	if err != nil {
		t.Error(err)
	}
	entry := tmpEntry.(*rsl.ReferenceEntry)
	assert.Equal(t, entry.TargetID, policyRef.Hash())
}

func TestStateGetRootMetadata(t *testing.T) {
	state := createTestStateWithOnlyRoot(t)

	rootMetadata, err := state.GetRootMetadata()
	assert.Nil(t, err)
	assert.Equal(t, 1, rootMetadata.Version)
	assert.Equal(t, "52e3b8e73279d6ebdd62a5016e2725ff284f569665eb92ccb145d83817a02997", rootMetadata.Roles[RootRoleName].KeyIDs[0])
}

func TestStateFindVerifiersForPath(t *testing.T) {
	t.Run("with policy", func(t *testing.T) {
		state := createTestStateWithPolicy(t)

		gpgKey, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
		if err != nil {
			t.Fatal(err)
		}

		tests := map[string]struct {
			path      string
			verifiers []*Verifier
		}{
			"verifiers for refs/heads/main": {
				path: "git:refs/heads/main",
				verifiers: []*Verifier{{
					name:      "protect-main",
					keys:      []*tuf.Key{gpgKey},
					threshold: 1,
				}},
			},
			"verifiers for files": {
				path: "file:1",
				verifiers: []*Verifier{{
					name:      "protect-files-1-and-2",
					keys:      []*tuf.Key{gpgKey},
					threshold: 1,
				}},
			},
			"verifiers for unprotected branch": {
				path:      "git:refs/heads/unprotected",
				verifiers: []*Verifier{},
			},
			"verifiers for unprotected files": {
				path:      "file:unprotected",
				verifiers: []*Verifier{},
			},
		}

		for name, test := range tests {
			verifiers, err := state.FindVerifiersForPath(context.Background(), test.path)
			assert.Nil(t, err, fmt.Sprintf("unexpected error in test '%s'", name))
			assert.Equal(t, test.verifiers, verifiers, fmt.Sprintf("policy verifiers for path '%s' don't match expected verifiers in test '%s'", test.path, name))
		}
	})

	t.Run("without policy", func(t *testing.T) {
		state := createTestStateWithOnlyRoot(t)

		verifiers, err := state.FindVerifiersForPath(context.Background(), "test-path")
		assert.Nil(t, verifiers)
		assert.ErrorIs(t, err, ErrMetadataNotFound)
	})
}

func TestStateFindPublicKeysForPath(t *testing.T) {
	state := createTestStateWithPolicy(t)

	gpgKeyBytes, err := os.ReadFile(filepath.Join("test-data", "gpg-pubkey.asc"))
	if err != nil {
		t.Fatal(err)
	}
	gpgKey, err := gpg.LoadGPGKeyFromBytes(gpgKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]struct {
		path string
		keys []*sslibsv.SSLibKey
	}{
		"public keys for refs/heads/main": {
			path: "git:refs/heads/main",
			keys: []*sslibsv.SSLibKey{gpgKey},
		},
		"public keys for unprotected branch": {
			path: "git:refs/heads/unprotected",
			keys: []*sslibsv.SSLibKey{},
		},
	}

	for name, test := range tests {
		keys, err := state.FindPublicKeysForPath(context.Background(), test.path)
		assert.Nil(t, err, fmt.Sprintf("unexpected error in test '%s'", name))
		assert.Equal(t, test.keys, keys, fmt.Sprintf("policy keys for path '%s' don't match expected keys in test '%s'", test.path, name))
	}
}

func TestGetStateForCommit(t *testing.T) {
	repo, firstState := createTestRepository(t, createTestStateWithPolicy)

	// Create some commits
	refName := "refs/heads/main"
	emptyTreeHash, err := gitinterface.WriteTree(repo, nil)
	if err != nil {
		t.Fatal(err)
	}
	commitID, err := gitinterface.Commit(repo, emptyTreeHash, refName, "Initial commit", false)
	if err != nil {
		t.Fatal(err)
	}

	// No RSL entry for commit => no state yet
	commit, err := gitinterface.GetCommit(repo, commitID)
	if err != nil {
		t.Fatal(err)
	}
	state, err := GetStateForCommit(context.Background(), repo, commit)
	assert.Nil(t, err)
	assert.Nil(t, state)

	// Record RSL entry for commit
	if err := rsl.NewReferenceEntry(refName, commitID).Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	state, err = GetStateForCommit(context.Background(), repo, commit)
	assert.Nil(t, err)
	assert.Equal(t, firstState, state)

	// Create new branch, record new commit there
	anotherRefName := "refs/heads/feature"
	if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(anotherRefName), commitID)); err != nil {
		t.Fatal(err)
	}
	newCommitID, err := gitinterface.Commit(repo, emptyTreeHash, anotherRefName, "Second commit", false)
	if err != nil {
		t.Fatal(err)
	}

	if err := rsl.NewReferenceEntry(anotherRefName, newCommitID).Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	newCommit, err := gitinterface.GetCommit(repo, newCommitID)
	if err != nil {
		t.Fatal(err)
	}

	state, err = GetStateForCommit(context.Background(), repo, newCommit)
	assert.Nil(t, err)
	assert.Equal(t, firstState, state)

	// Update policy, record in RSL
	secondState, err := LoadCurrentState(context.Background(), repo) // secondState := firstState will modify firstState as well
	if err != nil {
		t.Fatal(err)
	}
	targetsMetadata, err := secondState.GetTargetsMetadata(TargetsRoleName)
	if err != nil {
		t.Fatal(err)
	}
	targetsMetadata, err = AddOrUpdateDelegation(targetsMetadata, "new-rule", []*tuf.Key{}, []string{"*"}) // just a dummy rule
	if err != nil {
		t.Fatal(err)
	}
	signingKeyBytes, err := os.ReadFile(filepath.Join("test-data", "root"))
	if err != nil {
		t.Fatal(err)
	}
	signer, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(signingKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	targetsEnv, err := dsse.CreateEnvelope(targetsMetadata)
	if err != nil {
		t.Fatal(err)
	}
	targetsEnv, err = dsse.SignEnvelope(context.Background(), targetsEnv, signer)
	if err != nil {
		t.Fatal(err)
	}
	secondState.TargetsEnvelope = targetsEnv
	if err := secondState.Commit(context.Background(), repo, "Second state", false); err != nil {
		t.Fatal(err)
	}

	// Merge feature branch commit into main
	curRef, err := repo.Reference(plumbing.ReferenceName(refName), true)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Storer.CheckAndSetReference(plumbing.NewHashReference(plumbing.ReferenceName(refName), newCommitID), curRef); err != nil {
		t.Fatal(err)
	}

	// Record in RSL
	if err := rsl.NewReferenceEntry(refName, newCommitID).Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	// Check that for this commit ID, the first state is returned and not the
	// second
	state, err = GetStateForCommit(context.Background(), repo, newCommit)
	assert.Nil(t, err)
	assert.Equal(t, firstState, state)
}

func TestStateFindDelegationEntry(t *testing.T) {
	// Test case when no delegation is found
	t.Run("no delegation", func(t *testing.T) {
		state := createTestStateWithPolicy(t)
		entry, err := state.findDelegationEntry("random")
		assert.ErrorIs(t, err, ErrDelegationNotFound)
		assert.Nil(t, entry)
	})

	// Test case for a simple delegation
	t.Run("simple delegation", func(t *testing.T) {
		state := createTestStateWithDelegatedPolicies(t)
		entry, err := state.findDelegationEntry("1")
		assert.Nil(t, err)
		assert.Equal(t, &tuf.Delegation{Name: "1", Paths: []string{"path1/*"}, Terminating: false, Role: tuf.Role{KeyIDs: []string{"157507bbe151e378ce8126c1dcfe043cdd2db96e"}, Threshold: 1}}, entry)
	})

	// Test case for a delegation with multiple roles
	t.Run("delegation with multiple roles", func(t *testing.T) {
		state := createTestStateWithDelegatedPolicies(t)
		entry, err := state.findDelegationEntry("4")
		assert.Nil(t, err)
		assert.Equal(t, &tuf.Delegation{Name: "4", Paths: []string{"path1/subpath2/*"}, Terminating: false, Role: tuf.Role{KeyIDs: []string{"157507bbe151e378ce8126c1dcfe043cdd2db96e"}, Threshold: 1}}, entry)
	})
}
