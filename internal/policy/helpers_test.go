package policy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adityasaky/gittuf/internal/common"
	"github.com/adityasaky/gittuf/internal/rsl"
	"github.com/adityasaky/gittuf/internal/signerverifier"
	"github.com/adityasaky/gittuf/internal/signerverifier/dsse"
	"github.com/adityasaky/gittuf/internal/tuf"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/memory"
	sslibdsse "github.com/secure-systems-lab/go-securesystemslib/dsse"
	sslibsv "github.com/secure-systems-lab/go-securesystemslib/signerverifier"
)

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

	signingKeyBytes, err := os.ReadFile(filepath.Join("test-data", "root"))
	if err != nil {
		t.Fatal(err)
	}
	signer, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(signingKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	keyBytes, err := os.ReadFile(filepath.Join("test-data", "root.pub"))
	if err != nil {
		t.Fatal(err)
	}
	key, err := tuf.LoadKeyFromBytes(keyBytes)
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
		RootPublicKeys:      []*tuf.Key{key},
		RootEnvelope:        rootEnv,
		DelegationEnvelopes: map[string]*sslibdsse.Envelope{},
	}
}

func createTestStateWithPolicy(t *testing.T) *State {
	t.Helper()

	signingKeyBytes, err := os.ReadFile(filepath.Join("test-data", "root"))
	if err != nil {
		t.Fatal(err)
	}
	signer, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(signingKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	keyBytes, err := os.ReadFile(filepath.Join("test-data", "root.pub"))
	if err != nil {
		t.Fatal(err)
	}
	key, err := tuf.LoadKeyFromBytes(keyBytes)
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

	gpgKeyBytes, err := os.ReadFile(filepath.Join("test-data", "gpg-pubkey.asc"))
	if err != nil {
		t.Fatal(err)
	}
	gpgKey := &sslibsv.SSLibKey{
		KeyID:   common.TestGPGKeyFingerprint, // FIXME: create helper for GPG keys that populates fingerprint
		KeyType: signerverifier.GPGKeyType,
		Scheme:  signerverifier.GPGKeyType,
		KeyVal: sslibsv.KeyVal{
			Public: strings.TrimSpace(string(gpgKeyBytes)),
		},
	}

	targetsMetadata := InitializeTargetsMetadata()
	targetsMetadata, err = AddOrUpdateDelegation(targetsMetadata, "protect-main", []*tuf.Key{gpgKey}, []string{"git:refs/heads/main"})
	if err != nil {
		t.Fatal(err)
	}

	targetsMetadata, err = AddOrUpdateDelegation(targetsMetadata, "protect-file-2", []*tuf.Key{gpgKey}, []string{"git:refs/heads/main&&file:2"})
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
