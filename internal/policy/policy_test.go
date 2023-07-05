package policy

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/memory"

	"github.com/adityasaky/gittuf/internal/rsl"
	"github.com/adityasaky/gittuf/internal/tuf"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
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
	repo, state := createTestRepository(t)

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
	repo, state := createTestRepository(t)

	loadedState, err := LoadCurrentState(context.Background(), repo)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, state, loadedState)
}

func TestLoadStateForEntry(t *testing.T) {
	repo, state := createTestRepository(t)

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
	state := createTestState(t)

	if err := state.Verify(context.Background()); err != nil {
		t.Error(err)
	}

	rootKeys := []*tuf.Key{}
	copy(rootKeys, state.RootPublicKeys)
	state.RootPublicKeys = []*tuf.Key{}

	err := state.Verify(context.Background())
	assert.NotNil(t, err)

	state.RootPublicKeys = rootKeys
	state.RootEnvelope.Signatures = []dsse.Signature{}
	err = state.Verify(context.Background())
	assert.NotNil(t, err)
}

func TestStateCommit(t *testing.T) {
	repo, _ := createTestRepository(t)

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
	state := createTestState(t)

	rootMetadata, err := state.GetRootMetadata()
	assert.Nil(t, err)
	assert.Equal(t, 1, rootMetadata.Version)
	assert.Equal(t, "437cdafde81f715cf81e75920d7d4a9ce4cab83aac5a8a5984c3902da6bf2ab7", rootMetadata.Roles[RootRoleName].KeyIDs[0])
}

func createTestRepository(t *testing.T) (*git.Repository, *State) {
	t.Helper()

	state := createTestState(t)

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

func createTestState(t *testing.T) *State {
	t.Helper()

	rootEnvBytes, err := os.ReadFile(filepath.Join("test-data", metadataTreeEntryName, "root.json"))
	if err != nil {
		t.Fatal(err)
	}

	rootEnv := &dsse.Envelope{}
	if err := json.Unmarshal(rootEnvBytes, rootEnv); err != nil {
		t.Fatal(err)
	}

	keyBytes, err := os.ReadFile(filepath.Join("test-data", rootPublicKeysTreeEntryName, "437cdafde81f715cf81e75920d7d4a9ce4cab83aac5a8a5984c3902da6bf2ab7"))
	if err != nil {
		t.Fatal(err)
	}

	key, err := tuf.LoadKeyFromBytes(keyBytes)
	if err != nil {
		t.Fatal(err)
	}

	return &State{
		RootPublicKeys:      []*tuf.Key{key},
		RootEnvelope:        rootEnv,
		DelegationEnvelopes: map[string]*dsse.Envelope{},
	}
}
