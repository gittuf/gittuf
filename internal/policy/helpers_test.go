// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
)

var (
	testCtx                    = context.Background()
	rootKeyBytes               = artifacts.SSHRSAPrivate
	rootPubKeyBytes            = artifacts.SSHRSAPublicSSH
	targets1KeyBytes           = artifacts.SSHECDSAPrivate
	targets1PubKeyBytes        = artifacts.SSHECDSAPublicSSH
	targets2KeyBytes           = artifacts.SSHED25519Private
	targets2PubKeyBytes        = artifacts.SSHED25519PublicSSH
	gpgKeyBytes                = artifacts.GPGKey1Private
	gpgPubKeyBytes             = artifacts.GPGKey1Public
	gpgUnauthorizedKeyBytes    = artifacts.GPGKey2Private
	gpgUnauthorizedPubKeyBytes = artifacts.GPGKey2Public
)

func createTestRepository(t *testing.T, stateCreator func(*testing.T) *State) (*gitinterface.Repository, *State) {
	t.Helper()

	state := stateCreator(t)

	tempDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
	state.repository = repo

	if err := state.Commit(repo, "Create test state", true, false); err != nil {
		t.Fatal(err)
	}
	if err := Apply(testCtx, repo, false); err != nil {
		t.Fatal(err)
	}

	latestEntry, err := rsl.GetLatestEntry(repo)
	if err != nil {
		t.Fatal(err)
	}
	state.loadedEntry = latestEntry.(rsl.ReferenceUpdaterEntry)

	return repo, state
}

func createControllerAndNetworkRepositories(t *testing.T) (*gitinterface.Repository, *gitinterface.Repository) {
	t.Helper()

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
	if err != nil {
		t.Fatal(err)
	}
	controllerRootMetadata.SetRepositoryLocation(controllerRepositoryLocation)
	err = controllerRootMetadata.EnableController()
	if err != nil {
		t.Fatal(err)
	}
	err = controllerRootMetadata.AddNetworkRepository("test", networkRepositoryLocation, []tuf.Principal{tufv01.NewKeyFromSSLibKey(signer.MetadataKey())})
	if err != nil {
		t.Fatal(err)
	}
	controllerRootEnv, err := dsse.CreateEnvelope(controllerRootMetadata)
	if err != nil {
		t.Fatal(err)
	}
	controllerRootEnv, err = dsse.SignEnvelope(testCtx, controllerRootEnv, signer)
	if err != nil {
		t.Fatal(err)
	}
	controllerState.Metadata.RootEnvelope = controllerRootEnv
	err = controllerState.preprocess()
	if err != nil {
		t.Fatal(err)
	}
	err = controllerState.Commit(controllerRepository, "Initial policy\n", true, false)
	if err != nil {
		t.Fatal(err)
	}
	err = Apply(testCtx, controllerRepository, false)
	if err != nil {
		t.Fatal(err)
	}

	networkRootMetadata, err := networkState.GetRootMetadata(false)
	if err != nil {
		t.Fatal(err)
	}
	err = networkRootMetadata.AddControllerRepository("controller", controllerRepositoryLocation, []tuf.Principal{tufv01.NewKeyFromSSLibKey(signer.MetadataKey())})
	if err != nil {
		t.Fatal(err)
	}
	networkRootEnv, err := dsse.CreateEnvelope(networkRootMetadata)
	if err != nil {
		t.Fatal(err)
	}
	networkRootEnv, err = dsse.SignEnvelope(testCtx, networkRootEnv, signer)
	if err != nil {
		t.Fatal(err)
	}
	networkState.Metadata.RootEnvelope = networkRootEnv
	err = networkState.preprocess()
	if err != nil {
		t.Fatal(err)
	}
	err = networkState.Commit(networkRepository, "Initial policy\n", true, false)
	if err != nil {
		t.Fatal(err)
	}
	err = Apply(testCtx, networkRepository, false)
	if err != nil {
		t.Fatal(err)
	}

	return controllerRepository, networkRepository
}

func createTestStateWithOnlyRoot(t *testing.T) *State {
	t.Helper()

	signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes) //nolint:staticcheck
	key := tufv01.NewKeyFromSSLibKey(signer.MetadataKey())

	rootMetadata, err := InitializeRootMetadata(key)
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

	return &State{
		Metadata: &StateMetadata{
			RootEnvelope: rootEnv,
		},
	}
}

func createTestStateWithPolicy(t *testing.T) *State {
	t.Helper()

	signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	key := tufv01.NewKeyFromSSLibKey(signer.MetadataKey())

	rootMetadata, err := InitializeRootMetadata(key)
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

	gpgKeyR, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	gpgKey := tufv01.NewKeyFromSSLibKey(gpgKeyR)

	targetsMetadata := InitializeTargetsMetadata()
	if err := targetsMetadata.AddPrincipal(gpgKey); err != nil {
		t.Fatal(err)
	}
	if err := targetsMetadata.AddRule("protect-main", []string{gpgKey.KeyID}, []string{"git:refs/heads/main"}, 1); err != nil {
		t.Fatal(err)
	}
	// Add a file protection rule. When used with common.AddNTestCommitsToSpecifiedRef, we have files with names 1, 2, 3,...n.
	if err := targetsMetadata.AddRule("protect-files-1-and-2", []string{gpgKey.KeyID}, []string{"file:1", "file:2"}, 1); err != nil {
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
		Metadata: &StateMetadata{
			RootEnvelope:    rootEnv,
			TargetsEnvelope: targetsEnv,
		},
	}

	if err := state.preprocess(); err != nil {
		t.Fatal(err)
	}

	return state
}

// createTestStateWithGlobalConstraintThreshold creates a policy state with no
// explicit branch protection rules but with a two-approval constraint on
// changes to the main branch. The two keys trusted are `rootPubKeyBytes` and
// `gpgPubKeyBytes`.
func createTestStateWithGlobalConstraintThreshold(t *testing.T) *State {
	t.Helper()

	signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	key := tufv01.NewKeyFromSSLibKey(signer.MetadataKey())

	rootMetadata, err := InitializeRootMetadata(key)
	if err != nil {
		t.Fatal(err)
	}

	if err := rootMetadata.AddPrimaryRuleFilePrincipal(key); err != nil {
		t.Fatal(err)
	}

	if err := rootMetadata.AddGlobalRule(tufv01.NewGlobalRuleThreshold("threshold-2-main", []string{"git:refs/heads/main"}, 2)); err != nil {
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

	gpgKeyR, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	gpgKey := tufv01.NewKeyFromSSLibKey(gpgKeyR)

	targetsMetadata := InitializeTargetsMetadata()
	if err := targetsMetadata.AddPrincipal(gpgKey); err != nil {
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
		Metadata: &StateMetadata{
			RootEnvelope:    rootEnv,
			TargetsEnvelope: targetsEnv,
		},
	}

	if err := state.preprocess(); err != nil {
		t.Fatal(err)
	}

	return state
}

// createTestStateWithGlobalConstraintBlockForcePushes creates a policy state
// with no explicit branch protection rules but with a rule that blocks force
// pushes to main.
func createTestStateWithGlobalConstraintBlockForcePushes(t *testing.T) *State {
	t.Helper()

	signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	key := tufv01.NewKeyFromSSLibKey(signer.MetadataKey())

	rootMetadata, err := InitializeRootMetadata(key)
	if err != nil {
		t.Fatal(err)
	}

	if err := rootMetadata.AddPrimaryRuleFilePrincipal(key); err != nil {
		t.Fatal(err)
	}

	forcePushesGlobalRule, err := tufv01.NewGlobalRuleBlockForcePushes("block-force-pushes-main", []string{"git:refs/heads/main"})
	if err != nil {
		t.Fatal(err)
	}
	if err := rootMetadata.AddGlobalRule(forcePushesGlobalRule); err != nil {
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

	gpgKeyR, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	gpgKey := tufv01.NewKeyFromSSLibKey(gpgKeyR)

	targetsMetadata := InitializeTargetsMetadata()
	if err := targetsMetadata.AddPrincipal(gpgKey); err != nil {
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
		Metadata: &StateMetadata{
			RootEnvelope:    rootEnv,
			TargetsEnvelope: targetsEnv,
		},
	}

	if err := state.preprocess(); err != nil {
		t.Fatal(err)
	}

	return state
}

func createTestStateWithPolicyUsingPersons(t *testing.T) *State {
	t.Helper()

	signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	key := tufv01.NewKeyFromSSLibKey(signer.MetadataKey())

	rootMetadata, err := InitializeRootMetadata(key)
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

	gpgKeyR, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	gpgKey := tufv01.NewKeyFromSSLibKey(gpgKeyR)
	person := &tufv02.Person{
		PersonID: "jane.doe@example.com",
		PublicKeys: map[string]*tufv02.Key{
			gpgKey.KeyID: gpgKey,
		},
	}

	targetsMetadata := InitializeTargetsMetadata()
	if err := targetsMetadata.AddPrincipal(person); err != nil {
		t.Fatal(err)
	}
	if err := targetsMetadata.AddRule("protect-main", []string{person.PersonID}, []string{"git:refs/heads/main"}, 1); err != nil {
		t.Fatal(err)
	}
	// Add a file protection rule. When used with common.AddNTestCommitsToSpecifiedRef, we have files with names 1, 2, 3,...n.
	if err := targetsMetadata.AddRule("protect-files-1-and-2", []string{person.PersonID}, []string{"file:1", "file:2"}, 1); err != nil {
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
		Metadata: &StateMetadata{
			RootEnvelope:    rootEnv,
			TargetsEnvelope: targetsEnv,
		},
	}

	if err := state.preprocess(); err != nil {
		t.Fatal(err)
	}

	return state
}

func createTestStateWithDelegatedPolicies(t *testing.T) *State {
	t.Helper()

	signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	key := tufv01.NewKeyFromSSLibKey(signer.MetadataKey())

	rootMetadata, err := InitializeRootMetadata(key)
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

	gpgKeyR, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	gpgKey := tufv01.NewKeyFromSSLibKey(gpgKeyR)

	// Create the root targets metadata
	targetsMetadata := InitializeTargetsMetadata()

	if err := targetsMetadata.AddPrincipal(key); err != nil {
		t.Fatal(err)
	}
	if err := targetsMetadata.AddRule("1", []string{key.KeyID}, []string{"file:1/*"}, 1); err != nil {
		t.Fatal(err)
	}

	if err := targetsMetadata.AddRule("2", []string{key.KeyID}, []string{"file:2/*"}, 1); err != nil {
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
	if err := delegation1Metadata.AddPrincipal(gpgKey); err != nil {
		t.Fatal(err)
	}
	if err := delegation1Metadata.AddRule("3", []string{gpgKey.KeyID}, []string{"file:1/subpath1/*"}, 1); err != nil {
		t.Fatal(err)
	}

	if err := delegation1Metadata.AddRule("4", []string{gpgKey.KeyID}, []string{"file:1/subpath2/*"}, 1); err != nil {
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
		Metadata: &StateMetadata{
			RootEnvelope:        rootEnv,
			TargetsEnvelope:     targetsEnv,
			DelegationEnvelopes: map[string]*sslibdsse.Envelope{},
		},
	}

	// Add the delegation envelopes to the state

	curState.Metadata.DelegationEnvelopes["1"] = delegation1Env

	// delegation structure
	//
	//   targets
	//     /\
	//    1  2
	//   /\
	//  3  4

	if err := curState.preprocess(); err != nil {
		t.Fatal(err)
	}

	return curState
}

func createTestStateWithThresholdPolicy(t *testing.T) *State {
	t.Helper()

	state := createTestStateWithPolicy(t)

	gpgKeyR, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	gpgKey := tufv01.NewKeyFromSSLibKey(gpgKeyR)
	approverKey := tufv01.NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets1PubKeyBytes))

	targetsMetadata, err := state.GetTargetsMetadata(TargetsRoleName, false)
	if err != nil {
		t.Fatal(err)
	}

	if err := targetsMetadata.AddPrincipal(gpgKey); err != nil {
		t.Fatal(err)
	}
	if err := targetsMetadata.AddPrincipal(approverKey); err != nil {
		t.Fatal(err)
	}

	// Set threshold = 2 for existing rule with the added key
	if err := targetsMetadata.UpdateRule("protect-main", []string{gpgKey.KeyID, approverKey.KeyID}, []string{"git:refs/heads/main"}, 2); err != nil {
		t.Fatal(err)
	}

	targetsEnv, err := dsse.CreateEnvelope(targetsMetadata)
	if err != nil {
		t.Fatal(err)
	}

	signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

	targetsEnv, err = dsse.SignEnvelope(context.Background(), targetsEnv, signer)
	if err != nil {
		t.Fatal(err)
	}
	state.Metadata.TargetsEnvelope = targetsEnv

	return state
}

// createTestStateWithThresholdPolicyAndGitHubAppTrust sets up a test policy
// with threshold rules. It uses v0.2 (and higher) policy metadata to support
// GitHub apps.
//
// Usage notes:
//   - The app key is targets1PubKeyBytes
//   - The two authorized persons are "jane.doe" and "john.doe"
//   - jane.doe's signing key is gpgPubKeyBytes
//   - john.doe's signing key is targets2PubKeyBytes
//   - The protected namespace is the main branch
//   - Use either of them as the approver for the app, with the app's signing key
//     ID set as the app name
func createTestStateWithThresholdPolicyAndGitHubAppTrust(t *testing.T) *State {
	t.Helper()

	state := createTestStateWithPolicyUsingPersons(t)

	signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

	appName := tuf.GitHubAppRoleName

	rootMetadata, err := state.GetRootMetadata(false)
	if err != nil {
		t.Fatal(err)
	}

	appKey := tufv01.NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets1PubKeyBytes))
	if err := rootMetadata.AddGitHubAppPrincipal(tuf.GitHubAppRoleName, appKey); err != nil {
		t.Fatal(err)
	}
	rootMetadata.EnableGitHubAppApprovals(tuf.GitHubAppRoleName)

	rootEnv, err := dsse.CreateEnvelope(rootMetadata)
	if err != nil {
		t.Fatal(err)
	}
	rootEnv, err = dsse.SignEnvelope(context.Background(), rootEnv, signer)
	if err != nil {
		t.Fatal(err)
	}
	state.Metadata.RootEnvelope = rootEnv

	targetsMetadata, err := state.GetTargetsMetadata(TargetsRoleName, false)
	if err != nil {
		t.Fatal(err)
	}

	gpgKeyR, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	gpgKey := tufv01.NewKeyFromSSLibKey(gpgKeyR)
	person := &tufv02.Person{
		PersonID:             "jane.doe",
		PublicKeys:           map[string]*tufv02.Key{gpgKey.KeyID: gpgKey},
		AssociatedIdentities: map[string]string{appName: "jane.doe"},
	}

	if err := targetsMetadata.AddPrincipal(person); err != nil {
		t.Fatal(err)
	}

	approverKey := tufv01.NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets2PubKeyBytes))
	approver := &tufv02.Person{
		PersonID:             "john.doe",
		PublicKeys:           map[string]*tufv02.Key{approverKey.KeyID: approverKey},
		AssociatedIdentities: map[string]string{appName: "john.doe"},
	}
	if err := targetsMetadata.AddPrincipal(approver); err != nil {
		t.Fatal(err)
	}

	// Set threshold = 2 for existing rule with the added key
	if err := targetsMetadata.UpdateRule("protect-main", []string{person.ID(), approver.ID()}, []string{"git:refs/heads/main"}, 2); err != nil {
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
	state.Metadata.TargetsEnvelope = targetsEnv

	if err := state.preprocess(); err != nil {
		t.Fatal(err)
	}

	return state
}

// createTestStateWithThresholdPolicyAndGitHubAppTrustForMixedAttestations sets
// up a test policy with threshold rules. It uses v0.2 (and higher) policy
// metadata to support GitHub apps.
//
// Usage notes:
//   - The app key is targets1PubKeyBytes
//   - The three authorized persons are "jane.doe", "john.doe", and "jill.doe"
//   - jane.doe's signing key is gpgPubKeyBytes
//   - john.doe's signing key is targets2PubKeyBytes
//   - jill.doe's signing key is gpgUnauthorizedPubKeyBytes
//   - The protected namespace is the main branch
//   - Use any of them as the approver for the app, with the app's signing key
//     ID set as the app name
func createTestStateWithThresholdPolicyAndGitHubAppTrustForMixedAttestations(t *testing.T) *State {
	t.Helper()

	t.Setenv(dev.DevModeKey, "1")

	state := createTestStateWithPolicyUsingPersons(t)

	signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

	appName := tuf.GitHubAppRoleName // TODO: this should be generalized more

	rootMetadata, err := state.GetRootMetadata(false)
	if err != nil {
		t.Fatal(err)
	}

	appKey := tufv01.NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets1PubKeyBytes))
	if err := rootMetadata.AddGitHubAppPrincipal(tuf.GitHubAppRoleName, appKey); err != nil {
		t.Fatal(err)
	}
	rootMetadata.EnableGitHubAppApprovals(tuf.GitHubAppRoleName)

	rootEnv, err := dsse.CreateEnvelope(rootMetadata)
	if err != nil {
		t.Fatal(err)
	}
	rootEnv, err = dsse.SignEnvelope(context.Background(), rootEnv, signer)
	if err != nil {
		t.Fatal(err)
	}
	state.Metadata.RootEnvelope = rootEnv

	targetsMetadata, err := state.GetTargetsMetadata(TargetsRoleName, false)
	if err != nil {
		t.Fatal(err)
	}

	gpgKeyR, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	gpgKey := tufv01.NewKeyFromSSLibKey(gpgKeyR)
	person := &tufv02.Person{
		PersonID:             "jane.doe",
		PublicKeys:           map[string]*tufv02.Key{gpgKey.KeyID: gpgKey},
		AssociatedIdentities: map[string]string{appName: "jane.doe"},
	}
	if err := targetsMetadata.AddPrincipal(person); err != nil {
		t.Fatal(err)
	}

	approver1Key := tufv01.NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets2PubKeyBytes))
	if err := targetsMetadata.AddPrincipal(approver1Key); err != nil {
		t.Fatal(err)
	}
	approver1 := &tufv02.Person{
		PersonID:             "john.doe",
		PublicKeys:           map[string]*tufv02.Key{approver1Key.KeyID: approver1Key},
		AssociatedIdentities: map[string]string{appName: "john.doe"},
	}
	if err := targetsMetadata.AddPrincipal(approver1); err != nil {
		t.Fatal(err)
	}

	approver2KeyR, err := gpg.LoadGPGKeyFromBytes(gpgUnauthorizedPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	approver2Key := tufv01.NewKeyFromSSLibKey(approver2KeyR)
	approver2 := &tufv02.Person{
		PersonID:             "jill.doe",
		PublicKeys:           map[string]*tufv02.Key{approver2Key.KeyID: approver2Key},
		AssociatedIdentities: map[string]string{appName: "jill.doe"},
	}
	if err := targetsMetadata.AddPrincipal(approver2); err != nil {
		t.Fatal(err)
	}

	// Set threshold = 3 for existing rule with the added principals
	if err := targetsMetadata.UpdateRule("protect-main", []string{person.PersonID, approver1.PersonID, approver2.PersonID}, []string{"git:refs/heads/main"}, 3); err != nil {
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
	state.Metadata.TargetsEnvelope = targetsEnv

	if err := state.preprocess(); err != nil {
		t.Fatal(err)
	}

	return state
}

func createTestStateWithTagPolicy(t *testing.T) *State {
	t.Helper()

	state := createTestStateWithPolicy(t)

	gpgKeyR, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	gpgKey := tufv01.NewKeyFromSSLibKey(gpgKeyR)
	targetsMetadata, err := state.GetTargetsMetadata(TargetsRoleName, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := targetsMetadata.AddPrincipal(gpgKey); err != nil {
		t.Fatal(err)
	}
	if err := targetsMetadata.AddRule("protect-tags", []string{gpgKey.KeyID}, []string{"git:refs/tags/*"}, 1); err != nil {
		t.Fatal(err)
	}
	targetsEnv, err := dsse.CreateEnvelope(targetsMetadata)
	if err != nil {
		t.Fatal(err)
	}

	signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

	targetsEnv, err = dsse.SignEnvelope(context.Background(), targetsEnv, signer)
	if err != nil {
		t.Fatal(err)
	}
	state.Metadata.TargetsEnvelope = targetsEnv

	if err := state.preprocess(); err != nil {
		t.Fatal(err)
	}

	return state
}

func createTestStateWithThresholdTagPolicy(t *testing.T) *State {
	t.Helper()

	state := createTestStateWithPolicy(t)

	gpgKeyR, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	gpgKey := tufv01.NewKeyFromSSLibKey(gpgKeyR)
	approverKey := tufv01.NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets1PubKeyBytes))

	targetsMetadata, err := state.GetTargetsMetadata(TargetsRoleName, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := targetsMetadata.AddPrincipal(gpgKey); err != nil {
		t.Fatal(err)
	}
	if err := targetsMetadata.AddPrincipal(approverKey); err != nil {
		t.Fatal(err)
	}
	if err := targetsMetadata.AddRule("protect-tags", []string{gpgKey.KeyID, approverKey.KeyID}, []string{"git:refs/tags/*"}, 2); err != nil {
		t.Fatal(err)
	}
	targetsEnv, err := dsse.CreateEnvelope(targetsMetadata)
	if err != nil {
		t.Fatal(err)
	}

	signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

	targetsEnv, err = dsse.SignEnvelope(context.Background(), targetsEnv, signer)
	if err != nil {
		t.Fatal(err)
	}
	state.Metadata.TargetsEnvelope = targetsEnv

	if err := state.preprocess(); err != nil {
		t.Fatal(err)
	}

	return state
}

func createTestStateWithTagPolicyForUnauthorizedTest(t *testing.T) *State {
	t.Helper()

	state := createTestStateWithPolicy(t)

	rootKey := tufv01.NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, rootPubKeyBytes))
	targetsMetadata, err := state.GetTargetsMetadata(TargetsRoleName, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := targetsMetadata.AddPrincipal(rootKey); err != nil {
		t.Fatal(err)
	}
	if err := targetsMetadata.AddRule("protect-tags", []string{rootKey.KeyID}, []string{"git:refs/tags/*"}, 1); err != nil {
		t.Fatal(err)
	}
	targetsEnv, err := dsse.CreateEnvelope(targetsMetadata)
	if err != nil {
		t.Fatal(err)
	}

	signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

	targetsEnv, err = dsse.SignEnvelope(context.Background(), targetsEnv, signer)
	if err != nil {
		t.Fatal(err)
	}
	state.Metadata.TargetsEnvelope = targetsEnv

	if err := state.preprocess(); err != nil {
		t.Fatal(err)
	}

	return state
}

func setupSSHKeysForSigning(t *testing.T, privateBytes, publicBytes []byte) *ssh.Signer {
	t.Helper()

	keysDir := t.TempDir()
	privKeyPath := filepath.Join(keysDir, "key")
	pubKeyPath := filepath.Join(keysDir, "key.pub")

	if err := os.WriteFile(privKeyPath, privateBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pubKeyPath, publicBytes, 0o600); err != nil {
		t.Fatal(err)
	}

	signer, err := ssh.NewSignerFromFile(privKeyPath)
	if err != nil {
		t.Fatal(err)
	}

	return signer
}
