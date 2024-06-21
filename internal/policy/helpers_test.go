// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"context"
	"testing"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/signerverifier"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/internal/tuf"
	sslibdsse "github.com/secure-systems-lab/go-securesystemslib/dsse"
)

var (
	testCtx                 = context.Background()
	rootKeyBytes            = artifacts.SSLibKey1Private
	rootPubKeyBytes         = artifacts.SSLibKey1Public
	targets1KeyBytes        = artifacts.SSLibKey2Private
	targets1PubKeyBytes     = artifacts.SSLibKey2Public
	targets2KeyBytes        = artifacts.SSLibKey3Private
	targets2PubKeyBytes     = artifacts.SSLibKey3Public
	gpgKeyBytes             = artifacts.GPGKey1Private
	gpgPubKeyBytes          = artifacts.GPGKey1Public
	gpgUnauthorizedKeyBytes = artifacts.GPGKey2Private
)

func createTestRepository(t *testing.T, stateCreator func(*testing.T) *State) (*gitinterface.Repository, *State) {
	t.Helper()

	state := stateCreator(t)

	tempDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
	state.repository = repo

	if err := state.Commit(repo, "Create test state", false); err != nil {
		t.Fatal(err)
	}
	if err := Apply(testCtx, repo, false); err != nil {
		t.Fatal(err)
	}

	return repo, state
}

func createTestStateWithOnlyRoot(t *testing.T) *State {
	t.Helper()

	signer, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(rootKeyBytes) //nolint:staticcheck
	if err != nil {
		t.Fatal(err)
	}

	key, err := tuf.LoadKeyFromBytes(rootPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata := InitializeRootMetadata(key)

	rootEnv, err := dsse.CreateEnvelope(rootMetadata)
	if err != nil {
		t.Fatal(err)
	}
	rootEnv, err = dsse.SignEnvelope(context.Background(), rootEnv, signer)
	if err != nil {
		t.Fatal(err)
	}

	return &State{
		RootPublicKeys: []*tuf.Key{key},
		RootEnvelope:   rootEnv,
	}
}

func createTestStateWithPolicy(t *testing.T) *State {
	t.Helper()

	signer, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(rootKeyBytes) //nolint:staticcheck
	if err != nil {
		t.Fatal(err)
	}

	key, err := tuf.LoadKeyFromBytes(rootPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata := InitializeRootMetadata(key)

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

	gpgKey, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	targetsMetadata := InitializeTargetsMetadata()
	targetsMetadata, err = AddDelegation(targetsMetadata, "protect-main", []*tuf.Key{gpgKey}, []string{"git:refs/heads/main"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	// Add a file protection rule. When used with common.AddNTestCommitsToSpecifiedRef, we have files with names 1, 2, 3,...n.
	targetsMetadata, err = AddDelegation(targetsMetadata, "protect-files-1-and-2", []*tuf.Key{gpgKey}, []string{"file:1", "file:2"}, 1)
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

	state := &State{
		RootEnvelope:    rootEnv,
		TargetsEnvelope: targetsEnv,
		RootPublicKeys:  []*tuf.Key{key},
	}

	if err := state.loadRuleNames(); err != nil {
		t.Fatal(err)
	}

	return state
}

func createTestStateWithDelegatedPolicies(t *testing.T) *State {
	t.Helper()

	signer, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(rootKeyBytes) //nolint:staticcheck
	if err != nil {
		t.Fatal(err)
	}

	key, err := tuf.LoadKeyFromBytes(rootPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata := InitializeRootMetadata(key)

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

	gpgKey, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	// Create the root targets metadata
	targetsMetadata := InitializeTargetsMetadata()

	targetsMetadata, err = AddDelegation(targetsMetadata, "1", []*tuf.Key{key}, []string{"file:1/*"}, 1)
	if err != nil {
		t.Fatal(err)
	}

	targetsMetadata, err = AddDelegation(targetsMetadata, "2", []*tuf.Key{key}, []string{"file:2/*"}, 1)
	if err != nil {
		t.Fatal(err)
	}

	// Create the targets envelope
	targetsEnv, err := dsse.CreateEnvelope(targetsMetadata)
	if err != nil {
		t.Fatal(err)
	}
	targetsEnv, err = dsse.SignEnvelope(context.Background(), targetsEnv, signer)
	if err != nil {
		t.Fatal(err)
	}

	// Create the second level of delegations
	delegation1Metadata := InitializeTargetsMetadata()
	delegation1Metadata, err = AddDelegation(delegation1Metadata, "3", []*tuf.Key{gpgKey}, []string{"file:1/subpath1/*"}, 1)
	if err != nil {
		t.Fatal(err)
	}

	delegation1Metadata, err = AddDelegation(delegation1Metadata, "4", []*tuf.Key{gpgKey}, []string{"file:1/subpath2/*"}, 1)
	if err != nil {
		t.Fatal(err)
	}

	// Create the delegation envelope
	delegation1Env, err := dsse.CreateEnvelope(delegation1Metadata)
	if err != nil {
		t.Fatal(err)
	}
	delegation1Env, err = dsse.SignEnvelope(context.Background(), delegation1Env, signer)
	if err != nil {
		t.Fatal(err)
	}

	curState := &State{
		RootEnvelope:        rootEnv,
		TargetsEnvelope:     targetsEnv,
		DelegationEnvelopes: map[string]*sslibdsse.Envelope{},
		RootPublicKeys:      []*tuf.Key{key},
	}

	// Add the delegation envelopes to the state

	curState.DelegationEnvelopes["1"] = delegation1Env

	// delegation structure
	//
	//   targets
	//     /\
	//    1  2
	//   /\
	//  3  4

	if err := curState.loadRuleNames(); err != nil {
		t.Fatal(err)
	}

	return curState
}

func createTestStateWithThresholdPolicy(t *testing.T) *State {
	t.Helper()

	state := createTestStateWithPolicy(t)

	gpgKey, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	approverKey, err := tuf.LoadKeyFromBytes(targets1PubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	targetsMetadata, err := state.GetTargetsMetadata(TargetsRoleName)
	if err != nil {
		t.Fatal(err)
	}

	// Set threshold = 2 for existing rule with the added key
	targetsMetadata, err = UpdateDelegation(targetsMetadata, "protect-main", []*tuf.Key{gpgKey, approverKey}, []string{"git:refs/heads/main"}, 2)
	if err != nil {
		t.Fatal(err)
	}

	targetsEnv, err := dsse.CreateEnvelope(targetsMetadata)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(rootKeyBytes) //nolint:staticcheck
	if err != nil {
		t.Fatal(err)
	}
	targetsEnv, err = dsse.SignEnvelope(context.Background(), targetsEnv, signer)
	if err != nil {
		t.Fatal(err)
	}
	state.TargetsEnvelope = targetsEnv

	return state
}

func createTestStateWithTagPolicy(t *testing.T) *State {
	t.Helper()

	state := createTestStateWithPolicy(t)

	gpgKey, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	targetsMetadata, err := state.GetTargetsMetadata(TargetsRoleName)
	if err != nil {
		t.Fatal(err)
	}
	targetsMetadata, err = AddDelegation(targetsMetadata, "protect-tags", []*tuf.Key{gpgKey}, []string{"git:refs/tags/*"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	targetsEnv, err := dsse.CreateEnvelope(targetsMetadata)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(rootKeyBytes) //nolint:staticcheck
	if err != nil {
		t.Fatal(err)
	}
	targetsEnv, err = dsse.SignEnvelope(context.Background(), targetsEnv, signer)
	if err != nil {
		t.Fatal(err)
	}
	state.TargetsEnvelope = targetsEnv

	if err := state.loadRuleNames(); err != nil {
		t.Fatal(err)
	}

	return state
}

func createTestStateWithTagPolicyForUnauthorizedTest(t *testing.T) *State {
	t.Helper()

	state := createTestStateWithPolicy(t)

	rootKey, err := tuf.LoadKeyFromBytes(rootPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	targetsMetadata, err := state.GetTargetsMetadata(TargetsRoleName)
	if err != nil {
		t.Fatal(err)
	}
	targetsMetadata, err = AddDelegation(targetsMetadata, "protect-tags", []*tuf.Key{rootKey}, []string{"git:refs/tags/*"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	targetsEnv, err := dsse.CreateEnvelope(targetsMetadata)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(rootKeyBytes) //nolint:staticcheck
	if err != nil {
		t.Fatal(err)
	}
	targetsEnv, err = dsse.SignEnvelope(context.Background(), targetsEnv, signer)
	if err != nil {
		t.Fatal(err)
	}
	state.TargetsEnvelope = targetsEnv

	if err := state.loadRuleNames(); err != nil {
		t.Fatal(err)
	}

	return state
}
