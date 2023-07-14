package policy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/memory"

	"github.com/adityasaky/gittuf/internal/rsl"
	"github.com/adityasaky/gittuf/internal/signerverifier"
	"github.com/adityasaky/gittuf/internal/tuf"
	"github.com/go-git/go-git/v5/plumbing"
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

		ref, err = repo.Reference(plumbing.ReferenceName(PolicyStagingRef), true)
		assert.Nil(t, err)
		assert.Equal(t, plumbing.ZeroHash, ref.Hash())
	})

	t.Run("existing Policy namespace", func(t *testing.T) {
		repo, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}

		if err := InitializeNamespace(repo); err != nil {
			t.Fatal(err)
		}

		err = InitializeNamespace(repo)
		assert.ErrorIs(t, err, ErrPolicyExists)
	})
}

func TestLoadState(t *testing.T) {
	repo, state := createTestRepository(t, createTestStateWithOnlyRoot)

	rslRef, err := repo.Reference(plumbing.ReferenceName(rsl.RSLRef), true)
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

	entry, err := rsl.GetLatestEntryForRef(repo, PolicyRef)
	if err != nil {
		t.Fatal(err)
	}

	loadedState, err := LoadStateForEntry(context.Background(), repo, entry)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, state, loadedState)
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

	rslRef, err := repo.Reference(plumbing.ReferenceName(rsl.RSLRef), true)
	if err != nil {
		t.Error(err)
	}
	assert.NotEqual(t, plumbing.ZeroHash, rslRef.Hash())

	tmpEntry, err := rsl.GetEntry(repo, rslRef.Hash())
	if err != nil {
		t.Error(err)
	}
	entry := tmpEntry.(*rsl.Entry)
	assert.Equal(t, entry.CommitID, policyRef.Hash())
}

func TestStateGetRootMetadata(t *testing.T) {
	state := createTestStateWithOnlyRoot(t)

	rootMetadata, err := state.GetRootMetadata()
	assert.Nil(t, err)
	assert.Equal(t, 1, rootMetadata.Version)
	assert.Equal(t, "52e3b8e73279d6ebdd62a5016e2725ff284f569665eb92ccb145d83817a02997", rootMetadata.Roles[RootRoleName].KeyIDs[0])
}

func TestStateFindPublicKeysForPath(t *testing.T) {
	state := createTestStateWithPolicy(t)

	gpgKeyBytes, err := os.ReadFile(filepath.Join("test-data", "gpg-pubkey.asc"))
	if err != nil {
		t.Fatal(err)
	}
	gpgKey := &sslibsv.SSLibKey{
		KeyType: signerverifier.GPGKeyType,
		Scheme:  signerverifier.GPGKeyType,
		KeyVal: sslibsv.KeyVal{
			Public: strings.TrimSpace(string(gpgKeyBytes)),
		},
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
