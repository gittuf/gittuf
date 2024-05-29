// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"context"
	"fmt"
	"testing"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/signerverifier"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	sslibsv "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/signerverifier"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
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
	t.Run("loading while verifying multiple states", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithPolicy)

		entry, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		loadedState, err := LoadState(context.Background(), repo, entry.(*rsl.ReferenceEntry))
		if err != nil {
			t.Error(err)
		}

		assert.Equal(t, state, loadedState)

		targetsMetadata, err := state.GetTargetsMetadata(TargetsRoleName)
		if err != nil {
			t.Fatal(err)
		}

		targetsMetadata, err = AddDelegation(targetsMetadata, "test-rule-1", []*tuf.Key{}, []string{""}, 1, []tuf.Role{{KeyIDs: []string{""}, Threshold: 1}})
		if err != nil {
			t.Fatal(err)
		}
		state.ruleNames.Add("test-rule-1")

		env, err := dsse.CreateEnvelope(targetsMetadata)
		if err != nil {
			t.Fatal(err)
		}

		signer, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(rootKeyBytes) //nolint:staticcheck
		if err != nil {
			t.Fatal(err)
		}

		env, err = dsse.SignEnvelope(context.Background(), env, signer)
		if err != nil {
			t.Fatal(err)
		}

		state.TargetsEnvelope = env

		if err := state.Commit(repo, "", false); err != nil {
			t.Fatal(err)
		}

		if err := Apply(context.Background(), repo, false); err != nil {
			t.Fatal(err)
		}

		targetsMetadata, err = AddDelegation(targetsMetadata, "test-rule-2", []*tuf.Key{}, []string{""}, 1, []tuf.Role{{KeyIDs: []string{""}, Threshold: 1}})
		if err != nil {
			t.Fatal(err)
		}
		state.ruleNames.Add("test-rule-2")

		env, err = dsse.CreateEnvelope(targetsMetadata)
		if err != nil {
			t.Fatal(err)
		}

		env, err = dsse.SignEnvelope(context.Background(), env, signer)
		if err != nil {
			t.Fatal(err)
		}

		state.TargetsEnvelope = env

		if err := state.Commit(repo, "", false); err != nil {
			t.Fatal(err)
		}

		if err := Apply(context.Background(), repo, false); err != nil {
			t.Fatal(err)
		}

		entry, err = rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		loadedState, err = LoadState(context.Background(), repo, entry.(*rsl.ReferenceEntry))
		if err != nil {
			t.Error(err)
		}

		assert.Equal(t, state, loadedState)
	})

	t.Run("fail loading while verifying multiple states, bad sig", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithPolicy)

		entry, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		loadedState, err := LoadState(context.Background(), repo, entry.(*rsl.ReferenceEntry))
		if err != nil {
			t.Error(err)
		}

		assert.Equal(t, state, loadedState)

		targetsMetadata, err := state.GetTargetsMetadata(TargetsRoleName)
		if err != nil {
			t.Fatal(err)
		}

		targetsMetadata, err = AddDelegation(targetsMetadata, "test-rule-1", []*tuf.Key{}, []string{""}, 1, []tuf.Role{{KeyIDs: []string{""}, Threshold: 1}})
		if err != nil {
			t.Fatal(err)
		}
		state.ruleNames.Add("test-rule-1")

		env, err := dsse.CreateEnvelope(targetsMetadata)
		if err != nil {
			t.Fatal(err)
		}

		signer, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(rootKeyBytes) //nolint:staticcheck
		if err != nil {
			t.Fatal(err)
		}

		env, err = dsse.SignEnvelope(context.Background(), env, signer)
		if err != nil {
			t.Fatal(err)
		}

		state.TargetsEnvelope = env

		if err := state.Commit(repo, "", false); err != nil {
			t.Fatal(err)
		}

		if err := Apply(context.Background(), repo, false); err != nil {
			t.Fatal(err)
		}

		targetsMetadata, err = AddDelegation(targetsMetadata, "test-rule-2", []*tuf.Key{}, []string{""}, 1, []tuf.Role{{KeyIDs: []string{""}, Threshold: 1}})
		if err != nil {
			t.Fatal(err)
		}
		state.ruleNames.Add("test-rule-2")

		env, err = dsse.CreateEnvelope(targetsMetadata)
		if err != nil {
			t.Fatal(err)
		}

		badSigner, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(targets1KeyBytes) //nolint:staticcheck
		if err != nil {
			t.Fatal(err)
		}

		env, err = dsse.SignEnvelope(context.Background(), env, badSigner)
		if err != nil {
			t.Fatal(err)
		}

		state.TargetsEnvelope = env

		if err := state.Commit(repo, "", false); err != nil {
			t.Fatal(err)
		}

		policyStagingRef, err := repo.Reference(plumbing.ReferenceName(PolicyStagingRef), true)
		if err != nil {
			t.Fatal(err)
		}

		newPolicyRef := plumbing.NewHashReference(PolicyRef, policyStagingRef.Hash())
		if err := repo.Storer.SetReference(newPolicyRef); err != nil {
			t.Fatal(err)
		}

		if err := rsl.NewReferenceEntry(PolicyRef, policyStagingRef.Hash()).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		entry, err = rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		_, err = LoadState(context.Background(), repo, entry.(*rsl.ReferenceEntry))
		assert.ErrorIs(t, err, ErrVerifierConditionsUnmet)
	})
}

func TestLoadCurrentState(t *testing.T) {
	repo, state := createTestRepository(t, createTestStateWithOnlyRoot)

	loadedState, err := LoadCurrentState(context.Background(), repo, PolicyRef)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, state, loadedState)
}

func TestLoadFirstState(t *testing.T) {
	repo, firstState := createTestRepository(t, createTestStateWithPolicy)

	// Update policy, record in RSL
	secondState, err := LoadCurrentState(context.Background(), repo, PolicyRef) // secondState := state will modify state as well
	if err != nil {
		t.Fatal(err)
	}

	targetsMetadata, err := secondState.GetTargetsMetadata(TargetsRoleName)
	if err != nil {
		t.Fatal(err)
	}
	targetsMetadata, err = AddDelegation(targetsMetadata, "new-rule", []*tuf.Key{}, []string{"*"}, 1, []tuf.Role{{KeyIDs: []string{""}, Threshold: 1}}) // just a dummy rule
	if err != nil {
		t.Fatal(err)
	}
	signer, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(rootKeyBytes) //nolint:staticcheck
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
	if err := secondState.Commit(repo, "Second state", false); err != nil {
		t.Fatal(err)
	}

	loadedState, err := LoadFirstState(context.Background(), repo)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, firstState, loadedState)
}

func TestLoadStateForEntry(t *testing.T) {
	repo, state := createTestRepository(t, createTestStateWithOnlyRoot)

	entry, _, err := rsl.GetLatestReferenceEntryForRef(repo, PolicyRef)
	if err != nil {
		t.Fatal(err)
	}

	loadedState, err := loadStateForEntry(repo, entry)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, state, loadedState)
}

func TestStateKeys(t *testing.T) {
	state := createTestStateWithPolicy(t)

	expectedKeys := map[string]*tuf.Key{}
	rootKey, err := tuf.LoadKeyFromBytes(rootKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	expectedKeys[rootKey.KeyID] = rootKey

	gpgKey, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	expectedKeys[gpgKey.KeyID] = gpgKey

	keys, err := state.PublicKeys()
	assert.Nil(t, err, keys)
	assert.Equal(t, expectedKeys, keys)
}

func TestStateVerify(t *testing.T) {
	t.Run("only root", func(t *testing.T) {
		state := createTestStateWithOnlyRoot(t)

		err := state.Verify(testCtx)
		assert.Nil(t, err)
	})

	t.Run("only root, remove root keys", func(t *testing.T) {
		state := createTestStateWithOnlyRoot(t)

		state.RootPublicKeys = nil
		err := state.Verify(testCtx)
		assert.ErrorIs(t, err, ErrUnableToMatchRootKeys)
	})

	t.Run("with policy", func(t *testing.T) {
		state := createTestStateWithPolicy(t)

		err := state.Verify(testCtx)
		assert.Nil(t, err)
	})

	t.Run("with delegated policy", func(t *testing.T) {
		state := createTestStateWithDelegatedPolicies(t)

		err := state.Verify(testCtx)
		assert.Nil(t, err)
	})
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
			path       string
			mVerifiers []*MultiVerifier
		}{
			"verifiers for refs/heads/main": {
				path: "git:refs/heads/main",
				mVerifiers: []*MultiVerifier{{
					name: "protect-main",
					verifiers: []*Verifier{{
						keys:      []*tuf.Key{gpgKey},
						threshold: 1,
					}},
					threshold: 1,
				}},
			},
			"verifiers for files": {
				path: "file:1",
				mVerifiers: []*MultiVerifier{{
					name: "protect-files-1-and-2",
					verifiers: []*Verifier{{
						keys:      []*tuf.Key{gpgKey},
						threshold: 1,
					}},
					threshold: 1,
				}},
			},
			"verifiers for unprotected branch": {
				path:       "git:refs/heads/unprotected",
				mVerifiers: []*MultiVerifier{},
			},
			"verifiers for unprotected files": {
				path:       "file:unprotected",
				mVerifiers: []*MultiVerifier{},
			},
		}

		for name, test := range tests {
			verifiers, err := state.FindVerifiersForPath(test.path)
			assert.Nil(t, err, fmt.Sprintf("unexpected error in test '%s'", name))
			assert.Equal(t, test.mVerifiers, verifiers, fmt.Sprintf("policy verifiers for path '%s' don't match expected verifiers in test '%s'", test.path, name))
		}
	})

	t.Run("without policy", func(t *testing.T) {
		state := createTestStateWithOnlyRoot(t)

		verifiers, err := state.FindVerifiersForPath("test-path")
		assert.Nil(t, verifiers)
		assert.ErrorIs(t, err, ErrMetadataNotFound)
	})
}

func TestStateFindPublicKeysForPath(t *testing.T) {
	state := createTestStateWithPolicy(t)

	gpgKey, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
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
	secondState, err := LoadCurrentState(context.Background(), repo, PolicyRef) // secondState := firstState will modify firstState as well
	if err != nil {
		t.Fatal(err)
	}
	targetsMetadata, err := secondState.GetTargetsMetadata(TargetsRoleName)
	if err != nil {
		t.Fatal(err)
	}
	key, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	targetsMetadata, err = AddDelegation(targetsMetadata, "new-rule", []*tuf.Key{key}, []string{"*"}, 1, []tuf.Role{{KeyIDs: []string{""}, Threshold: 1}}) // just a dummy rule
	if err != nil {
		t.Fatal(err)
	}
	signer, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(rootKeyBytes) //nolint:staticcheck
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
	if err := secondState.Commit(repo, "Second state", false); err != nil {
		t.Fatal(err)
	}
	if err := Apply(context.Background(), repo, false); err != nil {
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

func TestListRules(t *testing.T) {
	t.Run("no delegations", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicy)

		rules, err := ListRules(context.Background(), repo, PolicyRef)
		assert.Nil(t, err)
		expectedRules := []*DelegationWithDepth{
			{
				Delegation: tuf.Delegation{
					Name:        "protect-main",
					Paths:       []string{"git:refs/heads/main"},
					Terminating: false,
					Custom:      nil,
					MinRoles:    1,
					Roles: []tuf.Role{{
						KeyIDs:    []string{"157507bbe151e378ce8126c1dcfe043cdd2db96e"},
						Threshold: 1},
					},
				},
				Depth: 0,
			},
			{
				Delegation: tuf.Delegation{
					Name:        "protect-files-1-and-2",
					Paths:       []string{"file:1", "file:2"},
					Terminating: false,
					Custom:      nil,
					MinRoles:    1,
					Roles: []tuf.Role{{
						KeyIDs:    []string{"157507bbe151e378ce8126c1dcfe043cdd2db96e"},
						Threshold: 1},
					},
				},
				Depth: 0,
			},
		}
		assert.Equal(t, expectedRules, rules)
	})
	t.Run("with delegations", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithDelegatedPolicies)

		rules, err := ListRules(context.Background(), repo, PolicyRef)

		assert.Nil(t, err)
		expectedRules := []*DelegationWithDepth{
			{
				Delegation: tuf.Delegation{
					Name:        "1",
					Paths:       []string{"file:1/*"},
					Terminating: false,
					Custom:      nil,
					MinRoles:    1,
					Roles: []tuf.Role{{
						KeyIDs:    []string{"52e3b8e73279d6ebdd62a5016e2725ff284f569665eb92ccb145d83817a02997"},
						Threshold: 1},
					},
				},
				Depth: 0,
			},
			{
				Delegation: tuf.Delegation{
					Name:        "3",
					Paths:       []string{"file:1/subpath1/*"},
					Terminating: false,
					Custom:      nil,
					MinRoles:    1,
					Roles: []tuf.Role{{
						KeyIDs:    []string{"157507bbe151e378ce8126c1dcfe043cdd2db96e"},
						Threshold: 1},
					},
				},
				Depth: 1,
			},
			{
				Delegation: tuf.Delegation{
					Name:        "4",
					Paths:       []string{"file:1/subpath2/*"},
					Terminating: false,
					Custom:      nil,
					MinRoles:    1,
					Roles: []tuf.Role{{
						KeyIDs:    []string{"157507bbe151e378ce8126c1dcfe043cdd2db96e"},
						Threshold: 1},
					},
				},
				Depth: 1,
			},

			{
				Delegation: tuf.Delegation{
					Name:        "2",
					Paths:       []string{"file:2/*"},
					Terminating: false,
					Custom:      nil,
					MinRoles:    1,
					Roles: []tuf.Role{{
						KeyIDs:    []string{"52e3b8e73279d6ebdd62a5016e2725ff284f569665eb92ccb145d83817a02997"},
						Threshold: 1},
					},
				},
				Depth: 0,
			},
		}
		assert.Equal(t, expectedRules, rules)
	})
}

func TestStateHasFileRule(t *testing.T) {
	t.Run("with file rules", func(t *testing.T) {
		state := createTestStateWithPolicy(t)

		hasFileRule, err := state.hasFileRule()
		assert.Nil(t, err)
		assert.True(t, hasFileRule)
	})

	t.Run("with no file rules", func(t *testing.T) {
		state := createTestStateWithOnlyRoot(t)

		hasFileRule, err := state.hasFileRule()
		assert.Nil(t, err)
		assert.False(t, hasFileRule)
	})
}

func TestApply(t *testing.T) {
	t.Run("single addition", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithOnlyRoot)

		key, err := tuf.LoadKeyFromBytes(rootPubKeyBytes)
		if err != nil {
			t.Fatal(err)
		}
		signer, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(rootKeyBytes) //nolint:staticcheck
		if err != nil {
			t.Fatal(err)
		}

		rootMetadata, err := state.GetRootMetadata()
		if err != nil {
			t.Fatal(err)
		}

		rootMetadata, err = AddTargetsKey(rootMetadata, key)
		if err != nil {
			t.Fatal(err)
		}

		rootEnv, err := dsse.CreateEnvelope(rootMetadata)
		if err != nil {
			t.Fatal(err)
		}
		rootEnv, err = dsse.SignEnvelope(context.Background(), rootEnv, signer)
		if err != nil {
			t.Fatal(err)
		}

		state.RootEnvelope = rootEnv

		if err := state.Commit(repo, "Added target key to root", false); err != nil {
			t.Fatal(err)
		}

		staging, err := LoadCurrentState(testCtx, repo, PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		policy, err := LoadCurrentState(testCtx, repo, PolicyRef)
		if err != nil {
			t.Fatal(err)
		}

		// Currently the policy ref is behind the staging ref, since the staging ref currently has an extra target key
		assert.NotEqual(t, staging, policy)

		err = Apply(testCtx, repo, false)

		assert.Nil(t, err)

		staging, err = LoadCurrentState(testCtx, repo, PolicyStagingRef)

		if err != nil {
			t.Fatal(err)
		}

		policy, err = LoadCurrentState(testCtx, repo, PolicyRef)

		if err != nil {
			t.Fatal(err)
		}

		// After Apply, the policy ref was fast-forward merged with the staging ref

		assert.Equal(t, staging, policy)
	})
}
