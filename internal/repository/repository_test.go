package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/adityasaky/gittuf/internal/common"
	"github.com/adityasaky/gittuf/internal/policy"
	"github.com/adityasaky/gittuf/internal/signerverifier"
	"github.com/adityasaky/gittuf/internal/signerverifier/dsse"
	"github.com/adityasaky/gittuf/internal/tuf"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/memory"
	d "github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/stretchr/testify/assert"
)

func TestLoadRepository(t *testing.T) {
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	testDir, err := common.CreateTestRepository()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(testDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(currentDir) //nolint:errcheck

	repository, err := LoadRepository()
	assert.Nil(t, err)
	assert.NotNil(t, repository.r)
}

func TestInitializeNamespaces(t *testing.T) {
	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	r := &Repository{r: repo}
	err = r.InitializeNamespaces()
	assert.Nil(t, err)
}

func TestInitializeRoot(t *testing.T) {
	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	r := &Repository{r: repo}

	rootKeyBytes, err := os.ReadFile(filepath.Join("test-data", "root"))
	if err != nil {
		t.Fatal(err)
	}
	key, err := tuf.LoadKeyFromBytes(rootKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	sv, err := signerverifier.NewSignerVerifierFromTUFKey(key)
	if err != nil {
		t.Fatal(err)
	}

	err = r.InitializeRoot(context.Background(), rootKeyBytes, false)
	assert.Nil(t, err)

	state, err := policy.LoadCurrentState(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata()
	assert.Nil(t, err)
	assert.Equal(t, key.ID(), rootMetadata.Roles[policy.RootRoleName].KeyIDs[0])
	assert.Equal(t, key.ID(), state.RootEnvelope.Signatures[0].KeyID)

	err = dsse.VerifyEnvelope(context.Background(), state.RootEnvelope, []d.Verifier{sv}, 1)
	assert.Nil(t, err)
}

func TestAddTopLevelTargetsKey(t *testing.T) {
	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	r := &Repository{r: repo}
	keyBytes, err := os.ReadFile(filepath.Join("test-data", "root"))
	if err != nil {
		t.Fatal(err)
	}
	key, err := tuf.LoadKeyFromBytes(keyBytes)
	if err != nil {
		t.Fatal(err)
	}
	sv, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(keyBytes)
	if err != nil {
		t.Fatal(err)
	}

	if err := r.InitializeRoot(context.Background(), keyBytes, false); err != nil {
		t.Fatal(err)
	}

	err = r.AddTopLevelTargetsKey(context.Background(), keyBytes, keyBytes, false)
	assert.Nil(t, err)

	state, err := policy.LoadCurrentState(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata()
	assert.Nil(t, err)
	assert.Equal(t, 2, rootMetadata.Version)
	assert.Equal(t, key.ID(), rootMetadata.Roles[policy.RootRoleName].KeyIDs[0])
	assert.Equal(t, key.ID(), rootMetadata.Roles[policy.TargetsRoleName].KeyIDs[0])
	assert.Equal(t, key.ID(), state.RootEnvelope.Signatures[0].KeyID)

	err = dsse.VerifyEnvelope(context.Background(), state.RootEnvelope, []d.Verifier{sv}, 1)
	assert.Nil(t, err)
}

func TestRemoveTopLevelTargetsKey(t *testing.T) {
	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	r := &Repository{r: repo}
	keyBytes, err := os.ReadFile(filepath.Join("test-data", "root"))
	if err != nil {
		t.Fatal(err)
	}
	rootKey, err := tuf.LoadKeyFromBytes(keyBytes)
	if err != nil {
		t.Fatal(err)
	}
	sv, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(keyBytes)
	if err != nil {
		t.Fatal(err)
	}

	if err := r.InitializeRoot(context.Background(), keyBytes, false); err != nil {
		t.Fatal(err)
	}

	err = r.AddTopLevelTargetsKey(context.Background(), keyBytes, keyBytes, false)
	if err != nil {
		t.Fatal(err)
	}

	targetsKeyBytes, err := os.ReadFile(filepath.Join("test-data", "targets.pub"))
	if err != nil {
		t.Fatal(err)
	}

	targetsKey, err := tuf.LoadKeyFromBytes(targetsKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	err = r.AddTopLevelTargetsKey(context.Background(), keyBytes, targetsKeyBytes, false)
	if err != nil {
		t.Fatal(err)
	}

	state, err := policy.LoadCurrentState(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata()
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 3, rootMetadata.Version)
	assert.Equal(t, rootKey.ID(), rootMetadata.Roles[policy.TargetsRoleName].KeyIDs[0])
	assert.Contains(t, rootMetadata.Roles[policy.TargetsRoleName].KeyIDs, rootKey.ID())
	assert.Contains(t, rootMetadata.Roles[policy.TargetsRoleName].KeyIDs, targetsKey.ID())
	err = dsse.VerifyEnvelope(context.Background(), state.RootEnvelope, []d.Verifier{sv}, 1)
	assert.Nil(t, err)

	err = r.RemoveTopLevelTargetsKey(context.Background(), keyBytes, rootKey.ID(), false)
	assert.Nil(t, err)

	state, err = policy.LoadCurrentState(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err = state.GetRootMetadata()
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 4, rootMetadata.Version)
	assert.Contains(t, rootMetadata.Roles[policy.TargetsRoleName].KeyIDs, targetsKey.ID())
	err = dsse.VerifyEnvelope(context.Background(), state.RootEnvelope, []d.Verifier{sv}, 1)
	assert.Nil(t, err)
}

func TestUnauthorizedRoot(t *testing.T) {
	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	r := &Repository{r: repo}
	keyBytes, err := os.ReadFile(filepath.Join("test-data", "root"))
	if err != nil {
		t.Fatal(err)
	}

	if err := r.InitializeRoot(context.Background(), keyBytes, false); err != nil {
		t.Fatal(err)
	}

	targetsKeyBytes, err := os.ReadFile(filepath.Join("test-data", "targets"))
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test add targets key", func(t *testing.T) {
		err := r.AddTopLevelTargetsKey(context.Background(), targetsKeyBytes, targetsKeyBytes, false)
		assert.ErrorIs(t, err, ErrUnauthorizedRootKey)
	})

	t.Run("test remove targets key", func(t *testing.T) {
		err := r.RemoveTopLevelTargetsKey(context.Background(), targetsKeyBytes, "some key ID", false)
		assert.ErrorIs(t, err, ErrUnauthorizedRootKey)
	})
}
