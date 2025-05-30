// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"context"
	"encoding/base64"
	"fmt"
	"path"
	"testing"

	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/gitinterface"
	policyopts "github.com/gittuf/gittuf/internal/policy/options/policy"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

		loadedState, err := LoadState(context.Background(), repo, entry.(*rsl.ReferenceEntry))
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

		state.Metadata.TargetsEnvelope = env

		if err := state.Commit(repo, "", true, false); err != nil {
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

		state.Metadata.TargetsEnvelope = env

		if err := state.Commit(repo, "", true, false); err != nil {
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

		loadedState, err := LoadState(context.Background(), repo, entry.(*rsl.ReferenceEntry))
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

		state.Metadata.TargetsEnvelope = env

		if err := state.Commit(repo, "", true, false); err != nil {
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

		state.Metadata.TargetsEnvelope = env

		if err := state.Commit(repo, "", true, false); err != nil {
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

		_, err = LoadState(context.Background(), repo, entry.(*rsl.ReferenceEntry))
		assert.ErrorIs(t, err, ErrVerifierConditionsUnmet)
	})

	t.Run("successful load with initial root principals", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithPolicy)

		entry, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		initialRootPrincipals := []tuf.Principal{tufv01.NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, rootPubKeyBytes))}
		loadedState, err := LoadState(context.Background(), repo, entry.(*rsl.ReferenceEntry), policyopts.WithInitialRootPrincipals(initialRootPrincipals))
		assert.Nil(t, err)

		assertStatesEqual(t, state, loadedState)
	})

	t.Run("expected failure with initial root principals", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicy)

		entry, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		initialRootPrincipals := []tuf.Principal{tufv01.NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets1PubKeyBytes))}
		_, err = LoadState(context.Background(), repo, entry.(*rsl.ReferenceEntry), policyopts.WithInitialRootPrincipals(initialRootPrincipals))
		assert.ErrorIs(t, err, ErrVerifierConditionsUnmet)
	})
}

func TestLoadCurrentState(t *testing.T) {
	t.Run("without initial keys", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithOnlyRoot)

		loadedState, err := LoadCurrentState(context.Background(), repo, PolicyRef)
		assert.Nil(t, err)
		assertStatesEqual(t, state, loadedState)
	})

	t.Run("with initial keys", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithOnlyRoot)

		initialRootPrincipals := []tuf.Principal{tufv01.NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, rootPubKeyBytes))}
		loadedState, err := LoadCurrentState(context.Background(), repo, PolicyRef, policyopts.WithInitialRootPrincipals(initialRootPrincipals))
		assert.Nil(t, err)
		assertStatesEqual(t, state, loadedState)
	})

	t.Run("with wrong initial keys", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithOnlyRoot)

		initialRootPrincipals := []tuf.Principal{tufv01.NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets1PubKeyBytes))}
		_, err := LoadCurrentState(context.Background(), repo, PolicyRef, policyopts.WithInitialRootPrincipals(initialRootPrincipals))
		assert.ErrorIs(t, err, ErrVerifierConditionsUnmet)
	})
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
	secondState.Metadata.TargetsEnvelope = targetsEnv
	if err := secondState.Commit(repo, "Second state", true, false); err != nil {
		t.Fatal(err)
	}

	loadedState, err := LoadFirstState(context.Background(), repo)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, firstState, loadedState)
}

func TestLoadStateForEntry(t *testing.T) {
	t.Run("regular state", func(t *testing.T) {
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
	})

	t.Run("with single controller metadata", func(t *testing.T) {
		// Create a state for controller repo, let's get the metadata from this
		// state and embed into another
		controllerState := createTestStateWithOnlyRoot(t)
		controllerName := "controller"

		state := createTestStateWithPolicy(t)
		state.ControllerMetadata = map[string]*StateMetadata{controllerName: controllerState.Metadata}

		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, true)
		state.repository = repo

		err := state.Commit(repo, "Create test state", true, false)
		assert.Nil(t, err)

		entry, err := rsl.GetLatestEntry(repo)
		require.Nil(t, err)

		loadedState, err := loadStateForEntry(repo, entry.(*rsl.ReferenceEntry))
		assert.Nil(t, err)

		assertStatesEqual(t, state, loadedState)
	})

	t.Run("with multiple controller metadata", func(t *testing.T) {
		// Create states for controller repos, let's get the metadata from these
		// states and embed into another
		controller1State := createTestStateWithOnlyRoot(t)
		controller1Name := "controller-1"

		controller2State := createTestStateWithDelegatedPolicies(t)
		controller2Name := "controller-2"

		state := createTestStateWithPolicy(t)
		state.ControllerMetadata = map[string]*StateMetadata{
			controller1Name: controller1State.Metadata,
			controller2Name: controller2State.Metadata,
		}

		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, true)
		state.repository = repo

		err := state.Commit(repo, "Create test state", true, false)
		assert.Nil(t, err)

		entry, err := rsl.GetLatestEntry(repo)
		require.Nil(t, err)

		loadedState, err := loadStateForEntry(repo, entry.(*rsl.ReferenceEntry))
		assert.Nil(t, err)

		assertStatesEqual(t, state, loadedState)
	})

	t.Run("with nested controller metadata", func(t *testing.T) {
		// Create states for controller repos, let's get the metadata from these
		// states and embed into another
		controller1State := createTestStateWithOnlyRoot(t)
		controller1Name := "controller-1"

		controller2State := createTestStateWithDelegatedPolicies(t)
		controller2Name := "controller-2"

		state := createTestStateWithPolicy(t)
		state.ControllerMetadata = map[string]*StateMetadata{
			controller1Name: controller1State.Metadata,
			controller2Name: controller2State.Metadata,
		}

		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, true)
		state.repository = repo

		err := state.Commit(repo, "Create test state", true, false)
		assert.Nil(t, err)

		entry, err := rsl.GetLatestEntry(repo)
		require.Nil(t, err)

		loadedState, err := loadStateForEntry(repo, entry.(*rsl.ReferenceEntry))
		assert.Nil(t, err)

		assertStatesEqual(t, state, loadedState)
	})
}

func TestStateVerify(t *testing.T) {
	t.Parallel()
	t.Run("only root", func(t *testing.T) {
		t.Parallel()
		state := createTestStateWithOnlyRoot(t)

		err := state.Verify(testCtx)
		assert.Nil(t, err)
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

	t.Run("successful verification with multiple repositories", func(t *testing.T) {
		controllerRepositoryLocation := t.TempDir()
		networkRepositoryLocation := t.TempDir()

		controllerRepository := gitinterface.CreateTestGitRepository(t, controllerRepositoryLocation, true)
		controllerState := createTestStateWithGlobalConstraintThreshold(t)
		controllerState.repository = controllerRepository

		networkRepository := gitinterface.CreateTestGitRepository(t, networkRepositoryLocation, false)
		networkState := createTestStateWithOnlyRoot(t)
		networkState.repository = networkRepository

		signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

		controllerRootMetadata, err := controllerState.GetRootMetadata(false)
		require.Nil(t, err)
		err = controllerRootMetadata.EnableController()
		require.Nil(t, err)
		err = controllerRootMetadata.AddNetworkRepository("test", networkRepositoryLocation, []tuf.Principal{tufv01.NewKeyFromSSLibKey(signer.MetadataKey())})
		require.Nil(t, err)
		controllerRootEnv, err := dsse.CreateEnvelope(controllerRootMetadata)
		require.Nil(t, err)
		controllerRootEnv, err = dsse.SignEnvelope(testCtx, controllerRootEnv, signer)
		require.Nil(t, err)
		controllerState.Metadata.RootEnvelope = controllerRootEnv
		err = controllerState.preprocess()
		require.Nil(t, err)
		err = controllerState.Commit(controllerRepository, "Initial policy\n", true, false)
		require.Nil(t, err)
		err = Apply(testCtx, controllerRepository, false)
		require.Nil(t, err)
		latestControllerEntry, err := rsl.GetLatestEntry(controllerRepository)
		require.Nil(t, err)
		controllerState.loadedEntry = latestControllerEntry.(rsl.ReferenceUpdaterEntry)

		networkRootMetadata, err := networkState.GetRootMetadata(false)
		require.Nil(t, err)
		err = networkRootMetadata.AddControllerRepository("controller", controllerRepositoryLocation, []tuf.Principal{tufv01.NewKeyFromSSLibKey(signer.MetadataKey())})
		require.Nil(t, err)
		networkRootEnv, err := dsse.CreateEnvelope(networkRootMetadata)
		require.Nil(t, err)
		networkRootEnv, err = dsse.SignEnvelope(testCtx, networkRootEnv, signer)
		require.Nil(t, err)
		networkState.Metadata.RootEnvelope = networkRootEnv
		err = networkState.preprocess()
		require.Nil(t, err)
		err = networkState.Commit(networkRepository, "Initial policy\n", true, false)
		require.Nil(t, err)
		err = Apply(testCtx, networkRepository, false)
		require.Nil(t, err)
		latestNetworkEntry, err := rsl.GetLatestEntry(networkRepository)
		require.Nil(t, err)
		networkState.loadedEntry = latestNetworkEntry.(rsl.ReferenceUpdaterEntry)

		err = rsl.PropagateChangesFromUpstreamRepository(networkRepository, controllerRepository, networkRootMetadata.GetPropagationDirectives(), false)
		require.Nil(t, err)

		latestEntry, err := rsl.GetLatestEntry(networkRepository)
		require.Nil(t, err)
		state, err := loadStateForEntry(networkRepository, latestEntry.(rsl.ReferenceUpdaterEntry))
		require.Nil(t, err)

		err = state.Verify(testCtx)
		assert.Nil(t, err)
	})

	t.Run("unsuccessful verification with multiple repositories due to mismatched keys", func(t *testing.T) {
		controllerRepositoryLocation := t.TempDir()
		networkRepositoryLocation := t.TempDir()

		controllerRepository := gitinterface.CreateTestGitRepository(t, controllerRepositoryLocation, true)
		controllerState := createTestStateWithGlobalConstraintThreshold(t)
		controllerState.repository = controllerRepository

		networkRepository := gitinterface.CreateTestGitRepository(t, networkRepositoryLocation, false)
		networkState := createTestStateWithOnlyRoot(t)
		networkState.repository = networkRepository

		signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

		controllerRootMetadata, err := controllerState.GetRootMetadata(false)
		require.Nil(t, err)
		err = controllerRootMetadata.EnableController()
		require.Nil(t, err)
		err = controllerRootMetadata.AddNetworkRepository("test", networkRepositoryLocation, []tuf.Principal{tufv01.NewKeyFromSSLibKey(signer.MetadataKey())})
		require.Nil(t, err)
		controllerRootEnv, err := dsse.CreateEnvelope(controllerRootMetadata)
		require.Nil(t, err)
		controllerRootEnv, err = dsse.SignEnvelope(testCtx, controllerRootEnv, signer)
		require.Nil(t, err)
		controllerState.Metadata.RootEnvelope = controllerRootEnv
		err = controllerState.preprocess()
		require.Nil(t, err)
		err = controllerState.Commit(controllerRepository, "Initial policy\n", true, false)
		require.Nil(t, err)
		err = Apply(testCtx, controllerRepository, false)
		require.Nil(t, err)
		latestControllerEntry, err := rsl.GetLatestEntry(controllerRepository)
		require.Nil(t, err)
		controllerState.loadedEntry = latestControllerEntry.(rsl.ReferenceUpdaterEntry)

		networkRootMetadata, err := networkState.GetRootMetadata(false)
		require.Nil(t, err)
		err = networkRootMetadata.AddControllerRepository("controller", controllerRepositoryLocation, []tuf.Principal{tufv01.NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets1PubKeyBytes))})
		require.Nil(t, err)
		networkRootEnv, err := dsse.CreateEnvelope(networkRootMetadata)
		require.Nil(t, err)
		networkRootEnv, err = dsse.SignEnvelope(testCtx, networkRootEnv, signer)
		require.Nil(t, err)
		networkState.Metadata.RootEnvelope = networkRootEnv
		err = networkState.preprocess()
		require.Nil(t, err)
		err = networkState.Commit(networkRepository, "Initial policy\n", true, false)
		require.Nil(t, err)
		err = Apply(testCtx, networkRepository, false)
		require.Nil(t, err)
		latestNetworkEntry, err := rsl.GetLatestEntry(networkRepository)
		require.Nil(t, err)
		networkState.loadedEntry = latestNetworkEntry.(rsl.ReferenceUpdaterEntry)

		err = rsl.PropagateChangesFromUpstreamRepository(networkRepository, controllerRepository, getPropagationDirectivesForNetworkRepository(t, networkRootMetadata), false)
		require.Nil(t, err)

		latestEntry, err := rsl.GetLatestEntry(networkRepository)
		require.Nil(t, err)
		state, err := loadStateForEntry(networkRepository, latestEntry.(rsl.ReferenceUpdaterEntry))
		require.Nil(t, err)

		err = state.Verify(testCtx)
		assert.ErrorIs(t, err, ErrControllerMetadataNotVerified)
	})
}

func TestStateCommit(t *testing.T) {
	t.Run("no controller metadata", func(t *testing.T) {
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
	})

	t.Run("with single controller metadata", func(t *testing.T) {
		// Create a state for controller repo, let's get the metadata from this
		// state and embed into another
		controllerState := createTestStateWithOnlyRoot(t)
		controllerName := "controller"

		state := createTestStateWithPolicy(t)
		state.ControllerMetadata = map[string]*StateMetadata{controllerName: controllerState.Metadata}

		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, true)
		state.repository = repo

		err := state.Commit(repo, "Create test state", true, false)
		assert.Nil(t, err)

		// The state commit must contain specific paths, search for them
		controllerPrefix := path.Join(tuf.GittufControllerPrefix, controllerName)
		expectedPaths := set.NewSetFromItems(
			path.Join(metadataTreeEntryName, "root.json"),
			path.Join(metadataTreeEntryName, "targets.json"),
			path.Join(controllerPrefix, "root.json"),
		)

		stagingTip, err := repo.GetReference(PolicyStagingRef)
		require.Nil(t, err)

		treeID, err := repo.GetCommitTreeID(stagingTip)
		require.Nil(t, err)

		allFiles, err := repo.GetAllFilesInTree(treeID)
		require.Nil(t, err)
		assert.Equal(t, expectedPaths.Len(), len(allFiles))

		for name := range allFiles {
			expectedPaths.Remove(name)
		}
		assert.Equal(t, 0, expectedPaths.Len())
	})

	t.Run("with multiple controller metadata", func(t *testing.T) {
		// Create states for controller repos, let's get the metadata from these
		// states and embed into another
		controller1State := createTestStateWithOnlyRoot(t)
		controller1Name := "controller-1"

		controller2State := createTestStateWithDelegatedPolicies(t)
		controller2Name := "controller-2"

		state := createTestStateWithPolicy(t)
		state.ControllerMetadata = map[string]*StateMetadata{
			controller1Name: controller1State.Metadata,
			controller2Name: controller2State.Metadata,
		}

		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, true)
		state.repository = repo

		err := state.Commit(repo, "Create test state", true, false)
		assert.Nil(t, err)

		// The state commit must contain specific paths, search for them
		controller1Prefix := path.Join(tuf.GittufControllerPrefix, controller1Name)
		controller2Prefix := path.Join(tuf.GittufControllerPrefix, controller2Name)
		expectedPaths := set.NewSetFromItems(
			path.Join(metadataTreeEntryName, "root.json"),
			path.Join(metadataTreeEntryName, "targets.json"),
			path.Join(controller1Prefix, "root.json"),
			path.Join(controller2Prefix, "root.json"),
			path.Join(controller2Prefix, "targets.json"),
			path.Join(controller2Prefix, "1.json"),
		)

		stagingTip, err := repo.GetReference(PolicyStagingRef)
		require.Nil(t, err)

		treeID, err := repo.GetCommitTreeID(stagingTip)
		require.Nil(t, err)

		allFiles, err := repo.GetAllFilesInTree(treeID)
		require.Nil(t, err)
		assert.Equal(t, expectedPaths.Len(), len(allFiles))

		for name := range allFiles {
			expectedPaths.Remove(name)
		}
		assert.Equal(t, 0, expectedPaths.Len())
	})

	t.Run("with nested controller metadata", func(t *testing.T) {
		// Create states for controller repos, let's get the metadata from these
		// states and embed into another
		controller1State := createTestStateWithOnlyRoot(t)
		controller1Name := "controller-1"

		controller2State := createTestStateWithDelegatedPolicies(t)
		controller2Name := "controller-2"

		state := createTestStateWithPolicy(t)
		state.ControllerMetadata = map[string]*StateMetadata{
			controller1Name: controller1State.Metadata,
			controller2Name: controller2State.Metadata,
		}

		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, true)
		state.repository = repo

		err := state.Commit(repo, "Create test state", true, false)
		assert.Nil(t, err)

		// The state commit must contain specific paths, search for them
		controller1Prefix := path.Join(tuf.GittufControllerPrefix, controller1Name)
		controller2Prefix := path.Join(tuf.GittufControllerPrefix, controller2Name)
		expectedPaths := set.NewSetFromItems(
			path.Join(metadataTreeEntryName, "root.json"),
			path.Join(metadataTreeEntryName, "targets.json"),
			path.Join(controller1Prefix, "root.json"),
			path.Join(controller2Prefix, "root.json"),
			path.Join(controller2Prefix, "targets.json"),
			path.Join(controller2Prefix, "1.json"),
		)

		stagingTip, err := repo.GetReference(PolicyStagingRef)
		require.Nil(t, err)

		treeID, err := repo.GetCommitTreeID(stagingTip)
		require.Nil(t, err)

		allFiles, err := repo.GetAllFilesInTree(treeID)
		require.Nil(t, err)
		assert.Equal(t, expectedPaths.Len(), len(allFiles))

		for name := range allFiles {
			expectedPaths.Remove(name)
		}
		assert.Equal(t, 0, expectedPaths.Len())
	})
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
	t.Run("regular apply", func(t *testing.T) {
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

		state.Metadata.RootEnvelope = rootEnv

		if err := state.Commit(repo, "Added target key to root", true, false); err != nil {
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
	})

	t.Run("policy out of sync with RSL, entry does not exist", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithOnlyRoot)
		latestEntry, err := rsl.GetLatestEntry(repo)
		require.Nil(t, err)
		parentEntry, err := rsl.GetParentForEntry(repo, latestEntry)
		require.Nil(t, err)

		// Undo entry for policy in RSL to force sync issue
		err = repo.SetReference(rsl.Ref, parentEntry.GetID())
		require.Nil(t, err)

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

		state.Metadata.RootEnvelope = rootEnv

		if err := state.Commit(repo, "Added target key to root", true, false); err != nil {
			t.Fatal(err)
		}

		err = Apply(testCtx, repo, false)
		assert.ErrorIs(t, err, ErrInvalidPolicy)
	})

	t.Run("policy out of sync with RSL, policy change does not match RSL", func(t *testing.T) {
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

		state.Metadata.RootEnvelope = rootEnv

		if err := state.Commit(repo, "Added target key to root", true, false); err != nil {
			t.Fatal(err)
		}

		stagingTip, err := repo.GetReference(PolicyStagingRef)
		require.Nil(t, err)
		err = repo.SetReference(PolicyRef, stagingTip)
		require.Nil(t, err)

		err = Apply(testCtx, repo, false)
		assert.ErrorIs(t, err, ErrInvalidPolicy)
	})

	t.Run("policy out of sync with RSL, policy ref does not exist", func(t *testing.T) {
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

		state.Metadata.RootEnvelope = rootEnv

		if err := state.Commit(repo, "Added target key to root", true, false); err != nil {
			t.Fatal(err)
		}

		err = repo.DeleteReference(PolicyRef)
		require.Nil(t, err)

		err = Apply(testCtx, repo, false)
		assert.ErrorIs(t, err, ErrInvalidPolicy)
	})
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

		state.Metadata.TargetsEnvelope = env

		if err := state.Commit(repo, "", true, false); err != nil {
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

func TestReconcileStaging(t *testing.T) {
	t.Parallel()

	t.Run("single repository, no controller", func(t *testing.T) {
		t.Parallel()

		repo, state := createTestRepository(t, createTestStateWithOnlyRoot)

		key := tufv01.NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, rootPubKeyBytes))

		signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

		// Reconciling staging should return no error here
		err := ReconcileStaging(repo, false)
		assert.Nil(t, err)

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

		state.Metadata.RootEnvelope = rootEnv

		if err := state.Commit(repo, "Added target key to root", true, false); err != nil {
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

		// Currently the policy ref is behind the staging ref, since the staging
		// ref currently has an extra target key
		assertStatesNotEqual(t, staging, policy)

		// Reconciling staging should return no error here
		err = ReconcileStaging(repo, false)
		assert.Nil(t, err)

		// Load states again and check that ReconcileStaging hasn't changed
		// anything
		postReconciliationStaging, err := LoadCurrentState(testCtx, repo, PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		postReconciliationPolicy, err := LoadCurrentState(testCtx, repo, PolicyRef)
		if err != nil {
			t.Fatal(err)
		}

		assertStatesNotEqual(t, postReconciliationStaging, postReconciliationPolicy)

		// Check that pre-reconciliation staging == post-reconciliation staging
		// and pre-reconciliation policy == post-reconciliation policy
		assertStatesEqual(t, staging, postReconciliationStaging)
		assertStatesEqual(t, policy, postReconciliationPolicy)

		if err := Apply(testCtx, repo, false); err != nil {
			t.Fatal(err)
		}

		staging, err = LoadCurrentState(testCtx, repo, PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		policy, err = LoadCurrentState(testCtx, repo, PolicyRef)
		if err != nil {
			t.Fatal(err)
		}

		// After Apply, the policy ref was fast-forward merged with the staging
		// ref
		assertStatesEqual(t, staging, policy)

		// Reconciling staging should return no error here
		err = ReconcileStaging(repo, false)
		assert.Nil(t, err)
	})

	controllerThresholdName := "test-controller-threshold-global-rule"
	controllerThresholdPatterns := []string{"git:refs/heads/main"}
	controllerThresholdAmount := 2

	networkThresholdName := "test-network-threshold-global-rule"
	networkThresholdPatterns := []string{"git:refs/heads/notmain"}
	networkThresholdAmount := 2

	// The next few tests stress test policy staging reconciliation with a
	// controller and network repository in different cases. Each test has a
	// timeline inside that describes what happens. Note that each test calls
	// the same helper to initialize the repositories, so each test starts with
	// the same configuration of controller and network repositories.

	t.Run("controller and network repository, basic propagation", func(t *testing.T) {
		t.Parallel()

		// The test follows the following timeline:
		// 0. Initialize both the controller and network repositories. The
		// controller will have a global rule defined at the start, while the
		// network repo will not.
		// 1. Propagate changes from the controller repository into the network
		// repository. Ensure that the reconciliation workflow is able to
		// reconcile the policy and policy-staging refs.

		// 0. Initialize both a controller and network repository
		controllerRepositoryLocation := t.TempDir()
		networkRepositoryLocation := t.TempDir()

		controllerRepository, networkRepository, _ := setupControllerAndNetworkRepositories(t, controllerRepositoryLocation, networkRepositoryLocation)
		controllerState, err := LoadCurrentState(testCtx, controllerRepository, PolicyRef)
		require.Nil(t, err)
		networkState, err := LoadCurrentState(testCtx, networkRepository, PolicyRef)
		require.Nil(t, err)
		networkRootMetadata, err := networkState.GetRootMetadata(false)
		require.Nil(t, err)
		networkRootEnv := networkState.Metadata.RootEnvelope

		// 1. Now, propagate changes from the controller into the network
		// repository
		err = rsl.PropagateChangesFromUpstreamRepository(networkRepository, controllerRepository, getPropagationDirectivesForNetworkRepository(t, networkRootMetadata), false)
		require.Nil(t, err)

		// These should not be equal, as policy has been updated, but *not*
		// policy-staging
		networkPolicyTip, err := networkRepository.GetReference(PolicyRef)
		require.Nil(t, err)
		networkStagingTip, err := networkRepository.GetReference(PolicyStagingRef)
		require.Nil(t, err)
		assert.NotEqual(t, networkPolicyTip, networkStagingTip)

		// Now, run the reconciliation workflow
		err = ReconcileStaging(networkRepository, false)
		assert.Nil(t, err)

		// Check that the policy has not changed and that staging has been
		// updated to match policy
		postReconciliationNetworkPolicyTip, err := networkRepository.GetReference(PolicyRef)
		require.Nil(t, err)
		assert.Equal(t, networkPolicyTip, postReconciliationNetworkPolicyTip)
		networkStagingTip, err = networkRepository.GetReference(PolicyStagingRef)
		require.Nil(t, err)
		assert.Equal(t, postReconciliationNetworkPolicyTip, networkStagingTip)

		// Load the updated network repository states
		networkState, err = LoadCurrentState(testCtx, networkRepository, PolicyRef)
		require.Nil(t, err)

		// Check that controller metadata has in fact been propagated...
		controllerName := fmt.Sprintf("controller-%s", base64.URLEncoding.EncodeToString([]byte(controllerRepositoryLocation)))
		assert.Equal(t, controllerState.Metadata.RootEnvelope, networkState.ControllerMetadata[controllerName].RootEnvelope)
		// ...and that other metadata has remained the same.
		assert.Equal(t, networkRootEnv, networkState.Metadata.RootEnvelope)
		assert.Nil(t, networkState.Metadata.TargetsEnvelope)
	})

	t.Run("controller and network repository, propagate with staged changes to network policy", func(t *testing.T) {
		t.Parallel()
		// 0. Initialize both the controller and network repositories. The
		// controller will have a global rule defined at the start, while the
		// network repo will not.
		// 1. Stage (but do not apply) a new global rule in both the controller
		// and network repositories.
		// 2. Apply the changes in the controller repository, and propagate them
		// into the network repository. Ensure that the reconciliation workflow
		// is able to handle this and reconcile the policy-staging ref with the
		// new changes coming from the controller with the staged local global
		// rule.
		// 3. Apply the changes in the network repository.

		// 0. Initialize both a controller and network repository
		controllerRepositoryLocation := t.TempDir()
		networkRepositoryLocation := t.TempDir()

		controllerRepository, networkRepository, signer := setupControllerAndNetworkRepositories(t, controllerRepositoryLocation, networkRepositoryLocation)
		networkState, err := LoadCurrentState(testCtx, networkRepository, PolicyRef)
		require.Nil(t, err)
		networkRootEnv := networkState.Metadata.RootEnvelope

		// 1. Now, create a new global rule in both the controller and network
		// repositories, but not staging or applying either yet
		controllerState, err := LoadCurrentState(testCtx, controllerRepository, PolicyRef)
		require.Nil(t, err)
		controllerRootMetadata, err := controllerState.GetRootMetadata(false)
		require.Nil(t, err)
		err = controllerRootMetadata.AddGlobalRule(tufv01.NewGlobalRuleThreshold(controllerThresholdName, controllerThresholdPatterns, controllerThresholdAmount))
		require.Nil(t, err)

		networkRootMetadata, err := networkState.GetRootMetadata(false)
		require.Nil(t, err)
		err = networkRootMetadata.AddGlobalRule(tufv01.NewGlobalRuleThreshold(networkThresholdName, networkThresholdPatterns, networkThresholdAmount))
		require.Nil(t, err)

		// Stage both changes
		controllerRootEnv, err := dsse.CreateEnvelope(controllerRootMetadata)
		require.Nil(t, err)
		controllerRootEnv, err = dsse.SignEnvelope(testCtx, controllerRootEnv, signer)
		require.Nil(t, err)
		controllerState.Metadata.RootEnvelope = controllerRootEnv
		err = controllerState.preprocess()
		require.Nil(t, err)
		err = controllerState.Commit(controllerRepository, "Add controller repo global rule\n", true, false)
		require.Nil(t, err)

		newNetworkRootEnv, err := dsse.CreateEnvelope(networkRootMetadata)
		require.Nil(t, err)
		newNetworkRootEnv, err = dsse.SignEnvelope(testCtx, newNetworkRootEnv, signer)
		require.Nil(t, err)
		networkState.Metadata.RootEnvelope = newNetworkRootEnv
		err = networkState.preprocess()
		require.Nil(t, err)
		err = networkState.Commit(networkRepository, "Add network repo global rule\n", true, false)
		require.Nil(t, err)
		networkStagingTip, err := networkRepository.GetReference(PolicyStagingRef)
		require.Nil(t, err)

		// 2. Apply the controller's changes and propagate
		err = Apply(testCtx, controllerRepository, false)
		require.Nil(t, err)
		err = rsl.PropagateChangesFromUpstreamRepository(networkRepository, controllerRepository, getPropagationDirectivesForNetworkRepository(t, networkRootMetadata), false)
		require.Nil(t, err)

		// The network repository's staging ref should not have changed since
		// the last time, we have not staged any new changes locally
		newNetworkStagingTip, err := networkRepository.GetReference(PolicyStagingRef)
		require.Nil(t, err)
		assert.Equal(t, networkStagingTip, newNetworkStagingTip)

		// Load the updated repository states
		controllerState, err = LoadCurrentState(testCtx, controllerRepository, PolicyRef)
		require.Nil(t, err)
		networkState, err = LoadCurrentState(testCtx, networkRepository, PolicyRef)
		require.Nil(t, err)

		// Check that controller metadata has in fact been propagated...
		controllerName := fmt.Sprintf("controller-%s", base64.URLEncoding.EncodeToString([]byte(controllerRepositoryLocation)))
		assert.Equal(t, controllerState.Metadata.RootEnvelope, networkState.ControllerMetadata[controllerName].RootEnvelope)
		// ...and that other metadata has remained the same.
		assert.Equal(t, networkRootEnv, networkState.Metadata.RootEnvelope)
		assert.Nil(t, networkState.Metadata.TargetsEnvelope)

		// These should not be equal, as policy has been updated, but *not*
		// policy-staging
		networkPolicyTip, err := networkRepository.GetReference(PolicyRef)
		require.Nil(t, err)
		networkStagingTip, err = networkRepository.GetReference(PolicyStagingRef)
		require.Nil(t, err)
		assert.NotEqual(t, networkPolicyTip, networkStagingTip)

		// Now, run the reconciliation workflow
		err = ReconcileStaging(networkRepository, false)
		assert.Nil(t, err)

		// Check that the policy-staging tip is different due to reconciliation
		postReconciliationNetworkStagingTip, err := networkRepository.GetReference(PolicyStagingRef)
		require.Nil(t, err)
		assert.NotEqual(t, networkStagingTip, postReconciliationNetworkStagingTip)

		// Now, apply policy on the network repository
		err = Apply(testCtx, networkRepository, false)
		require.Nil(t, err)

		// These should be reconciled now
		networkPolicyTip, err = networkRepository.GetReference(PolicyRef)
		require.Nil(t, err)
		networkStagingTip, err = networkRepository.GetReference(PolicyStagingRef)
		require.Nil(t, err)
		assert.Equal(t, networkPolicyTip, networkStagingTip)
	})

	t.Run("controller and network repository, propagate after policy diverges", func(t *testing.T) {
		t.Parallel()
		// 0. Initialize both the controller and network repositories. The
		// controller will have a global rule defined at the start, while the
		// network repo will not.
		// 1. Create a new global rule in the network repository. Stage and
		// apply the change.
		// 2. Create a new global rule in the controller repository. Stage and
		// apply this change.
		// 3. Propagate the changes from the controller repository into the
		// network repository. Ensure that the reconciliation workflow can
		// handle this as well.

		// 0. Initialize both a controller and network repository
		controllerRepositoryLocation := t.TempDir()
		networkRepositoryLocation := t.TempDir()

		controllerRepository, networkRepository, signer := setupControllerAndNetworkRepositories(t, controllerRepositoryLocation, networkRepositoryLocation)
		controllerState, err := LoadCurrentState(testCtx, controllerRepository, PolicyRef)
		require.Nil(t, err)
		networkState, err := LoadCurrentState(testCtx, networkRepository, PolicyRef)
		require.Nil(t, err)

		// 1. Create a new global rule in the network repository
		networkRootMetadata, err := networkState.GetRootMetadata(false)
		require.Nil(t, err)
		err = networkRootMetadata.AddGlobalRule(tufv01.NewGlobalRuleThreshold(networkThresholdName, networkThresholdPatterns, networkThresholdAmount))
		require.Nil(t, err)

		networkRootEnv, err := dsse.CreateEnvelope(networkRootMetadata)
		require.Nil(t, err)
		networkRootEnv, err = dsse.SignEnvelope(testCtx, networkRootEnv, signer)
		require.Nil(t, err)
		networkState.Metadata.RootEnvelope = networkRootEnv
		err = networkState.preprocess()
		require.Nil(t, err)
		err = networkState.Commit(networkRepository, "Add network repo global rule two\n", true, false)
		require.Nil(t, err)
		err = Apply(testCtx, networkRepository, false)
		require.Nil(t, err)

		// 2. Create a new global rule in the controller repository
		controllerRootMetadata, err := controllerState.GetRootMetadata(false)
		require.Nil(t, err)
		err = controllerRootMetadata.AddGlobalRule(tufv01.NewGlobalRuleThreshold(controllerThresholdName, controllerThresholdPatterns, controllerThresholdAmount))
		require.Nil(t, err)

		controllerRootEnv, err := dsse.CreateEnvelope(controllerRootMetadata)
		require.Nil(t, err)
		controllerRootEnv, err = dsse.SignEnvelope(testCtx, controllerRootEnv, signer)
		require.Nil(t, err)
		controllerState.Metadata.RootEnvelope = controllerRootEnv
		err = controllerState.preprocess()
		require.Nil(t, err)
		err = controllerState.Commit(controllerRepository, "Add controller repo global rule two\n", true, false)
		require.Nil(t, err)
		err = Apply(testCtx, controllerRepository, false)
		require.Nil(t, err)

		// 3. Propagate changes from the controller repository into the network
		// repository
		err = rsl.PropagateChangesFromUpstreamRepository(networkRepository, controllerRepository, getPropagationDirectivesForNetworkRepository(t, networkRootMetadata), false)
		require.Nil(t, err)

		// These should not be equal, as policy has been updated, but *not*
		// policy-staging
		networkPolicyTip, err := networkRepository.GetReference(PolicyRef)
		require.Nil(t, err)
		networkStagingTip, err := networkRepository.GetReference(PolicyStagingRef)
		require.Nil(t, err)
		assert.NotEqual(t, networkPolicyTip, networkStagingTip)

		// Reconcile the network repository's policy-staging ref with the new
		// changes in the policy ref
		err = ReconcileStaging(networkRepository, false)
		assert.Nil(t, err)
		networkPolicyTip, err = networkRepository.GetReference(PolicyRef)
		require.Nil(t, err)
		networkStagingTip, err = networkRepository.GetReference(PolicyStagingRef)
		require.Nil(t, err)
		assert.Equal(t, networkPolicyTip, networkStagingTip)

		// Load the updated network repository state
		networkState, err = LoadCurrentState(testCtx, networkRepository, PolicyRef)
		require.Nil(t, err)

		// Check that controller metadata has in fact been propagated...
		controllerName := fmt.Sprintf("controller-%s", base64.URLEncoding.EncodeToString([]byte(controllerRepositoryLocation)))
		assert.Equal(t, controllerState.Metadata.RootEnvelope, networkState.ControllerMetadata[controllerName].RootEnvelope)
		// ...and that other metadata has remained the same.
		assert.Equal(t, networkRootEnv, networkState.Metadata.RootEnvelope)
		assert.Nil(t, networkState.Metadata.TargetsEnvelope)
	})
}

func assertStatesEqual(t *testing.T, stateA, stateB *State) {
	t.Helper()

	assert.Equal(t, stateA.Metadata, stateB.Metadata)
	assert.Equal(t, stateA.ControllerMetadata, stateB.ControllerMetadata)
}

func assertStatesNotEqual(t *testing.T, stateA, stateB *State) {
	t.Helper()

	// at least one of these has to be different
	assert.True(t, assert.NotEqual(t, stateA.Metadata, stateB.Metadata) ||
		assert.NotEqual(t, stateA.ControllerMetadata, stateB.ControllerMetadata))
}

func setupControllerAndNetworkRepositories(t *testing.T, controllerRepositoryLocation, networkRepositoryLocation string) (*gitinterface.Repository, *gitinterface.Repository, *ssh.Signer) {
	controllerRepository := gitinterface.CreateTestGitRepository(t, controllerRepositoryLocation, true)
	// The controller will start with a global threshold rule...
	controllerState := createTestStateWithGlobalConstraintThreshold(t)
	controllerState.repository = controllerRepository

	// ...while the network repository will not.
	networkRepository := gitinterface.CreateTestGitRepository(t, networkRepositoryLocation, false)
	networkState := createTestStateWithOnlyRoot(t)
	networkState.repository = networkRepository

	signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

	// Set up the controller repository and add a network repository to it
	controllerRootMetadata, err := controllerState.GetRootMetadata(false)
	require.Nil(t, err)
	err = controllerRootMetadata.EnableController()
	require.Nil(t, err)
	err = controllerRootMetadata.AddNetworkRepository("test", networkRepositoryLocation, []tuf.Principal{tufv01.NewKeyFromSSLibKey(signer.MetadataKey())})
	require.Nil(t, err)
	controllerRootEnv, err := dsse.CreateEnvelope(controllerRootMetadata)
	require.Nil(t, err)
	controllerRootEnv, err = dsse.SignEnvelope(testCtx, controllerRootEnv, signer)
	require.Nil(t, err)
	controllerState.Metadata.RootEnvelope = controllerRootEnv
	err = controllerState.preprocess()
	require.Nil(t, err)
	err = controllerState.Commit(controllerRepository, "Initial policy\n", true, false)
	require.Nil(t, err)
	err = Apply(testCtx, controllerRepository, false)
	require.Nil(t, err)

	// Set up the network repository and add the controller to it
	networkRootMetadata, err := networkState.GetRootMetadata(false)
	require.Nil(t, err)
	err = networkRootMetadata.AddControllerRepository("controller", controllerRepositoryLocation, []tuf.Principal{tufv01.NewKeyFromSSLibKey(signer.MetadataKey())})
	require.Nil(t, err)
	networkRootEnv, err := dsse.CreateEnvelope(networkRootMetadata)
	require.Nil(t, err)
	networkRootEnv, err = dsse.SignEnvelope(testCtx, networkRootEnv, signer)
	require.Nil(t, err)
	networkState.Metadata.RootEnvelope = networkRootEnv
	err = networkState.preprocess()
	require.Nil(t, err)
	err = networkState.Commit(networkRepository, "Initial policy\n", true, false)
	require.Nil(t, err)
	err = Apply(testCtx, networkRepository, false)
	require.Nil(t, err)

	return controllerRepository, networkRepository, signer
}
