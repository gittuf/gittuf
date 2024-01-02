// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"context"
	_ "embed"
	"testing"

	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/signerverifier"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/memory"
	sslibdsse "github.com/secure-systems-lab/go-securesystemslib/dsse"
)

var testCtx = context.Background()

//go:embed test-data/root
var rootKeyBytes []byte

//go:embed test-data/root.pub
var rootPubKeyBytes []byte

//go:embed test-data/targets-1
var targets1KeyBytes []byte

//go:embed test-data/targets-1.pub
var targets1PubKeyBytes []byte

//go:embed test-data/gpg-pubkey.asc
var gpgPubKeyBytes []byte

func createTestRepository(t *testing.T, stateCreator func(*testing.T) *State) (*git.Repository, *State) {
	t.Helper()

	state := stateCreator(t)

	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	if err := InitializeNamespace(repo); err != nil {
		t.Fatal(err)
	}
	if err := rsl.InitializeNamespace(repo); err != nil {
		t.Fatal(err)
	}

	if err := state.Commit(context.Background(), repo, "Create test state", false); err != nil {
		t.Fatal(err)
	}

	return repo, state
}

func createTestStateWithOnlyRoot(t *testing.T) *State {
	t.Helper()

	signer, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(rootKeyBytes)
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

	signer, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(rootKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	key, err := tuf.LoadKeyFromBytes(rootPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata := InitializeRootMetadata(key)

	rootMetadata = AddTargetsKey(rootMetadata, key)

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
	targetsMetadata, err = AddOrUpdateDelegation(targetsMetadata, "protect-main", []*tuf.Key{gpgKey}, []string{"git:refs/heads/main"})
	if err != nil {
		t.Fatal(err)
	}
	// Add a file protection rule. When used with common.AddNTestCommitsToSpecifiedRef, we have files with names 1, 2, 3,...n.
	targetsMetadata, err = AddOrUpdateDelegation(targetsMetadata, "protect-files-1-and-2", []*tuf.Key{gpgKey}, []string{"file:1", "file:2"})
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

	return &State{
		RootEnvelope:    rootEnv,
		TargetsEnvelope: targetsEnv,
		RootPublicKeys:  []*tuf.Key{key},
	}
}

func createTestStateWithDelegatedPolicies(t *testing.T) *State {
	t.Helper()

	signer, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(rootKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	key, err := tuf.LoadKeyFromBytes(rootPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	gpgkey, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)

	if err != nil {
		t.Fatal(err)
	}

	rootMetadata := InitializeRootMetadata(key)

	rootMetadata = AddTargetsKey(rootMetadata, key)

	rootEnv, err := dsse.CreateEnvelope(rootMetadata)
	if err != nil {
		t.Fatal(err)
	}
	rootEnv, err = dsse.SignEnvelope(context.Background(), rootEnv, signer)
	if err != nil {
		t.Fatal(err)
	}

	// Create the root targets metadata
	targetsMetadata := InitializeTargetsMetadata()

	targetsMetadata, err = AddOrUpdateDelegation(targetsMetadata, "1", []*tuf.Key{key}, []string{"file:1/*"})
	if err != nil {
		t.Fatal(err)
	}

	targetsMetadata, err = AddOrUpdateDelegation(targetsMetadata, "2", []*tuf.Key{key}, []string{"file:2/*"})
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
	delegation1Metadata, err = AddOrUpdateDelegation(delegation1Metadata, "3", []*tuf.Key{gpgkey}, []string{"file:1/subpath1/*"})
	if err != nil {
		t.Fatal(err)
	}

	delegation1Metadata, err = AddOrUpdateDelegation(delegation1Metadata, "4", []*tuf.Key{gpgkey}, []string{"file:1/subpath2/*"})
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

	return curState
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
	targetsMetadata, err = AddOrUpdateDelegation(targetsMetadata, "protect-tags", []*tuf.Key{gpgKey}, []string{"git:refs/tags/*"})
	if err != nil {
		t.Fatal(err)
	}
	targetsEnv, err := dsse.CreateEnvelope(targetsMetadata)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(rootKeyBytes)
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
	targetsMetadata, err = AddOrUpdateDelegation(targetsMetadata, "protect-tags", []*tuf.Key{rootKey}, []string{"git:refs/tags/*"})
	if err != nil {
		t.Fatal(err)
	}
	targetsEnv, err := dsse.CreateEnvelope(targetsMetadata)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(rootKeyBytes)
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
