// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"context"
	"fmt"
	"testing"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	"github.com/stretchr/testify/assert"
)

func TestLoadState(t *testing.T) {
	t.Run("loading while verifying multiple states", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithPolicy)
		signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
		key := tufv01.NewKeyFromSSLibKey(signer.MetadataKey())

		entry, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		loadedState, err := LoadState(context.Background(), repo, entry.(*rsl.ReferenceEntry), nil)
		if err != nil {
			t.Error(err)
		}

		assertStatesEqual(t, state, loadedState)

		targetsMetadata, err := state.GetTargetsMetadata(TargetsRoleName, false)
		if err != nil {
			t.Fatal(err)
		}

		if err := targetsMetadata.AddPrincipal(key); err != nil {
			t.Fatal(err)
		}

		if err := targetsMetadata.AddRule("test-rule-1", []string{key.KeyID}, []string{"test-rule-1"}, 1); err != nil {
			t.Fatal(err)
		}
		state.ruleNames.Add("test-rule-1")

		env, err := dsse.CreateEnvelope(targetsMetadata)
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

		if err := targetsMetadata.AddRule("test-rule-2", []string{key.KeyID}, []string{"test-rule-2"}, 1); err != nil {
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

		loadedState, err = LoadState(context.Background(), repo, entry.(*rsl.ReferenceEntry), nil)
		if err != nil {
			t.Error(err)
		}

		assertStatesEqual(t, state, loadedState)
	})

	t.Run("fail loading while verifying multiple states, bad sig", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithPolicy)
		signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
		key := tufv01.NewKeyFromSSLibKey(signer.MetadataKey())

		entry, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		loadedState, err := LoadState(context.Background(), repo, entry.(*rsl.ReferenceEntry), nil)
		if err != nil {
			t.Error(err)
		}

		assertStatesEqual(t, state, loadedState)

		targetsMetadata, err := state.GetTargetsMetadata(TargetsRoleName, false)
		if err != nil {
			t.Fatal(err)
		}

		if err := targetsMetadata.AddPrincipal(key); err != nil {
			t.Fatal(err)
		}

		if err := targetsMetadata.AddRule("test-rule-1", []string{key.KeyID}, []string{"test-rule-1"}, 1); err != nil {
			t.Fatal(err)
		}
		state.ruleNames.Add("test-rule-1")

		env, err := dsse.CreateEnvelope(targetsMetadata)
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

		if err := targetsMetadata.AddRule("test-rule-2", []string{key.KeyID}, []string{"test-rule-2"}, 1); err != nil {
			t.Fatal(err)
		}
		state.ruleNames.Add("test-rule-2")

		env, err = dsse.CreateEnvelope(targetsMetadata)
		if err != nil {
			t.Fatal(err)
		}

		badSigner := setupSSHKeysForSigning(t, targets1KeyBytes, targets1PubKeyBytes)

		env, err = dsse.SignEnvelope(context.Background(), env, badSigner)
		if err != nil {
			t.Fatal(err)
		}

		state.TargetsEnvelope = env

		if err := state.Commit(repo, "", false); err != nil {
			t.Fatal(err)
		}

		policyStagingRefTip, err := repo.GetReference(PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		if err := repo.SetReference(PolicyRef, policyStagingRefTip); err != nil {
			t.Fatal(err)
		}

		if err := rsl.NewReferenceEntry(PolicyRef, policyStagingRefTip).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		entry, err = rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		_, err = LoadState(context.Background(), repo, entry.(*rsl.ReferenceEntry), nil)
		assert.ErrorIs(t, err, ErrVerifierConditionsUnmet)
	})
}

func TestLoadCurrentState(t *testing.T) {
	repo, state := createTestRepository(t, createTestStateWithOnlyRoot)

	loadedState, err := LoadCurrentState(context.Background(), repo, PolicyRef)
	if err != nil {
		t.Error(err)
	}
	assertStatesEqual(t, state, loadedState)
}

func TestLoadFirstState(t *testing.T) {
	repo, firstState := createTestRepository(t, createTestStateWithPolicy)

	// Update policy, record in RSL
	secondState, err := LoadCurrentState(context.Background(), repo, PolicyRef) // secondState := state will modify state as well
	if err != nil {
		t.Fatal(err)
	}
	signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	key := tufv01.NewKeyFromSSLibKey(signer.MetadataKey())

	targetsMetadata, err := secondState.GetTargetsMetadata(TargetsRoleName, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := targetsMetadata.AddPrincipal(key); err != nil {
		t.Fatal(err)
	}
	if err := targetsMetadata.AddRule("new-rule", []string{key.KeyID}, []string{"*"}, 1); err != nil { // just a dummy rule
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

	entry, _, err := rsl.GetLatestReferenceUpdaterEntry(repo, rsl.ForReference(PolicyRef))
	if err != nil {
		t.Fatal(err)
	}

	loadedState, err := loadStateForEntry(repo, entry)
	if err != nil {
		t.Error(err)
	}

	assertStatesEqual(t, state, loadedState)
}

func TestStateVerify(t *testing.T) {
	t.Parallel()
	t.Run("only root", func(t *testing.T) {
		t.Parallel()
		state := createTestStateWithOnlyRoot(t)

		err := state.Verify(testCtx)
		assert.Nil(t, err)
	})

	t.Run("only root, remove root keys", func(t *testing.T) {
		t.Parallel()
		state := createTestStateWithOnlyRoot(t)

		state.RootPublicKeys = nil
		err := state.Verify(testCtx)
		assert.ErrorIs(t, err, ErrUnableToMatchRootKeys)
	})

	t.Run("with policy", func(t *testing.T) {
		t.Parallel()
		state := createTestStateWithPolicy(t)

		err := state.Verify(testCtx)
		assert.Nil(t, err)
	})

	t.Run("with delegated policy", func(t *testing.T) {
		t.Parallel()
		state := createTestStateWithDelegatedPolicies(t)

		err := state.Verify(testCtx)
		assert.Nil(t, err)
	})
}

func TestStateCommit(t *testing.T) {
	repo, _ := createTestRepository(t, createTestStateWithOnlyRoot)
	// Commit and Apply are called by the helper

	policyTip, err := repo.GetReference(PolicyRef)
	if err != nil {
		t.Fatal(err)
	}

	tmpEntry, err := rsl.GetLatestEntry(repo)
	if err != nil {
		t.Fatal(err)
	}
	entry := tmpEntry.(*rsl.ReferenceEntry)

	assert.Equal(t, entry.TargetID, policyTip)
}

func TestStateGetRootMetadata(t *testing.T) {
	t.Parallel()
	state := createTestStateWithOnlyRoot(t)

	rootMetadata, err := state.GetRootMetadata(true)
	assert.Nil(t, err)

	rootPrincipals, err := rootMetadata.GetRootPrincipals()
	assert.Nil(t, err)
	assert.Equal(t, "SHA256:ESJezAOo+BsiEpddzRXS6+wtF16FID4NCd+3gj96rFo", rootPrincipals[0].ID())
}

func TestStateFindVerifiersForPath(t *testing.T) {
	t.Parallel()
	t.Run("with delegated policy", func(t *testing.T) {
		t.Parallel()
		state := createTestStateWithDelegatedPolicies(t) // changed from createTestStateWithPolicies to increase test
		// coverage to cover s.DelegationEnvelopes in PublicKeys()

		keyR := ssh.NewKeyFromBytes(t, rootPubKeyBytes)
		key := tufv01.NewKeyFromSSLibKey(keyR)

		tests := map[string]struct {
			path      string
			verifiers []*SignatureVerifier
		}{
			"verifiers for files 1": {
				path: "file:1/*",
				verifiers: []*SignatureVerifier{{
					name:       "1",
					principals: []tuf.Principal{key},
					threshold:  1,
				}},
			},
			"verifiers for files": {
				path: "file:2/*",
				verifiers: []*SignatureVerifier{{
					name:       "2",
					principals: []tuf.Principal{key},
					threshold:  1,
				}},
			},
			"verifiers for unprotected branch": {
				path:      "git:refs/heads/unprotected",
				verifiers: []*SignatureVerifier{},
			},
			"verifiers for unprotected files": {
				path:      "file:unprotected",
				verifiers: []*SignatureVerifier{},
			},
		}

		for name, test := range tests {
			verifiers, err := state.FindVerifiersForPath(test.path)
			assert.Nil(t, err, fmt.Sprintf("unexpected error in test '%s'", name))
			assert.Equal(t, test.verifiers, verifiers, fmt.Sprintf("policy verifiers for path '%s' don't match expected verifiers in test '%s'", test.path, name))
		}
	})

	t.Run("without policy", func(t *testing.T) {
		t.Parallel()
		state := createTestStateWithOnlyRoot(t)

		verifiers, err := state.FindVerifiersForPath("test-path")
		assert.Nil(t, verifiers)
		assert.ErrorIs(t, err, ErrMetadataNotFound)
	})
}

func TestGetStateForCommit(t *testing.T) {
	t.Parallel()
	repo, firstState := createTestRepository(t, createTestStateWithPolicy)

	// Create some commits
	refName := "refs/heads/main"
	treeBuilder := gitinterface.NewTreeBuilder(repo)
	emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
	if err != nil {
		t.Fatal(err)
	}
	commitID, err := repo.Commit(emptyTreeHash, refName, "Initial commit", false)
	if err != nil {
		t.Fatal(err)
	}

	// No RSL entry for commit => no state yet
	state, err := GetStateForCommit(context.Background(), repo, commitID)
	assert.Nil(t, err)
	assert.Nil(t, state)

	// Record RSL entry for commit
	if err := rsl.NewReferenceEntry(refName, commitID).Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	state, err = GetStateForCommit(context.Background(), repo, commitID)
	assert.Nil(t, err)
	assertStatesEqual(t, firstState, state)

	// Create new branch, record new commit there
	anotherRefName := "refs/heads/feature"
	if err := repo.SetReference(anotherRefName, commitID); err != nil {
		t.Fatal(err)
	}
	newCommitID, err := repo.Commit(emptyTreeHash, anotherRefName, "Second commit", false)
	if err != nil {
		t.Fatal(err)
	}

	if err := rsl.NewReferenceEntry(anotherRefName, newCommitID).Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	state, err = GetStateForCommit(context.Background(), repo, newCommitID)
	assert.Nil(t, err)
	assertStatesEqual(t, firstState, state)

	// Update policy, record in RSL
	secondState, err := LoadCurrentState(context.Background(), repo, PolicyRef) // secondState := firstState will modify firstState as well
	if err != nil {
		t.Fatal(err)
	}
	targetsMetadata, err := secondState.GetTargetsMetadata(TargetsRoleName, false)
	if err != nil {
		t.Fatal(err)
	}
	keyR, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	key := tufv01.NewKeyFromSSLibKey(keyR)
	if err := targetsMetadata.AddRule("new-rule", []string{key.KeyID}, []string{"*"}, 1); err != nil { // just a dummy rule
		t.Fatal(err)
	}

	signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

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
	if err := repo.CheckAndSetReference(refName, newCommitID, commitID); err != nil {
		t.Fatal(err)
	}

	// Record in RSL
	if err := rsl.NewReferenceEntry(refName, newCommitID).Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	// Check that for this commit ID, the first state is returned and not the
	// second
	state, err = GetStateForCommit(context.Background(), repo, newCommitID)
	assert.Nil(t, err)
	assertStatesEqual(t, firstState, state)
}

func TestStateHasFileRule(t *testing.T) {
	t.Parallel()
	t.Run("with file rules", func(t *testing.T) {
		state := createTestStateWithDelegatedPolicies(t)

		hasFileRule := state.hasFileRule
		assert.True(t, hasFileRule)
	})

	t.Run("with no file rules", func(t *testing.T) {
		t.Parallel()
		state := createTestStateWithOnlyRoot(t)

		hasFileRule := state.hasFileRule
		assert.False(t, hasFileRule)
	})
}

func TestApply(t *testing.T) {
	repo, state := createTestRepository(t, createTestStateWithOnlyRoot)

	key := tufv01.NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, rootPubKeyBytes))

	signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

	rootMetadata, err := state.GetRootMetadata(false)
	if err != nil {
		t.Fatal(err)
	}

	if err := rootMetadata.AddPrimaryRuleFilePrincipal(key); err != nil {
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
	assertStatesNotEqual(t, staging, policy)

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
	assertStatesEqual(t, staging, policy)
}

func TestDiscard(t *testing.T) {
	t.Parallel()

	t.Run("discard changes when policy ref exists", func(t *testing.T) {
		t.Parallel()
		repo, state := createTestRepository(t, createTestStateWithPolicy)

		signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
		key := tufv01.NewKeyFromSSLibKey(signer.MetadataKey())

		targetsMetadata, err := state.GetTargetsMetadata(TargetsRoleName, false)
		if err != nil {
			t.Fatal(err)
		}

		if err := targetsMetadata.AddPrincipal(key); err != nil {
			t.Fatal(err)
		}

		if err := targetsMetadata.AddRule("test-rule", []string{key.KeyID}, []string{"test-rule"}, 1); err != nil {
			t.Fatal(err)
		}

		env, err := dsse.CreateEnvelope(targetsMetadata)
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

		policyTip, err := repo.GetReference(PolicyRef)
		if err != nil {
			t.Fatal(err)
		}

		stagingTip, err := repo.GetReference(PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		assert.NotEqual(t, policyTip, stagingTip)

		err = Discard(repo)
		assert.Nil(t, err)

		policyTip, err = repo.GetReference(PolicyRef)
		if err != nil {
			t.Fatal(err)
		}

		stagingTip, err = repo.GetReference(PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, policyTip, stagingTip)
	})

	t.Run("discard changes when policy ref does not exist", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		treeBuilder := gitinterface.NewTreeBuilder(repo)
		emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
		if err != nil {
			t.Fatal(err)
		}

		commitID, err := repo.Commit(emptyTreeHash, PolicyStagingRef, "test commit", false)
		if err != nil {
			t.Fatal(err)
		}

		stagingTip, err := repo.GetReference(PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, commitID, stagingTip)

		err = Discard(repo)
		assert.Nil(t, err)

		_, err = repo.GetReference(PolicyStagingRef)
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)
	})
}

func assertStatesEqual(t *testing.T, stateA, stateB *State) {
	t.Helper()

	assert.Equal(t, stateA.RootEnvelope, stateB.RootEnvelope)
	assert.Equal(t, stateA.TargetsEnvelope, stateB.TargetsEnvelope)
	assert.Equal(t, stateA.DelegationEnvelopes, stateB.DelegationEnvelopes)
	assert.Equal(t, stateA.RootPublicKeys, stateB.RootPublicKeys)
}

func assertStatesNotEqual(t *testing.T, stateA, stateB *State) {
	t.Helper()

	// at least one of these has to be different
	assert.True(t, assert.NotEqual(t, stateA.RootEnvelope, stateB.RootEnvelope) || assert.NotEqual(t, stateA.TargetsEnvelope, stateB.TargetsEnvelope) || assert.NotEqual(t, stateA.DelegationEnvelopes, stateB.DelegationEnvelopes) || assert.NotEqual(t, stateA.RootPublicKeys, stateB.RootPublicKeys))
}
