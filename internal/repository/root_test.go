package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/adityasaky/gittuf/internal/policy"
	"github.com/adityasaky/gittuf/internal/signerverifier"
	"github.com/adityasaky/gittuf/internal/signerverifier/dsse"
	"github.com/adityasaky/gittuf/internal/tuf"
	d "github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/stretchr/testify/assert"
)

func TestInitializeRoot(t *testing.T) {
	// The helper also runs InitializeRoot for this test
	r, rootKeyBytes := createTestRepositoryWithRoot(t)

	key, err := tuf.LoadKeyFromBytes(rootKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	keyID, err := key.ID()
	if err != nil {
		t.Fatal(err)
	}
	sv, err := signerverifier.NewSignerVerifierFromTUFKey(key)
	if err != nil {
		t.Fatal(err)
	}

	state, err := policy.LoadCurrentState(context.Background(), r.r)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata()
	assert.Nil(t, err)
	assert.Equal(t, keyID, rootMetadata.Roles[policy.RootRoleName].KeyIDs[0])
	assert.Equal(t, keyID, state.RootEnvelope.Signatures[0].KeyID)

	err = dsse.VerifyEnvelope(context.Background(), state.RootEnvelope, []d.Verifier{sv}, 1)
	assert.Nil(t, err)
}

func TestAddTopLevelTargetsKey(t *testing.T) {
	r, keyBytes := createTestRepositoryWithRoot(t)

	key, err := tuf.LoadKeyFromBytes(keyBytes)
	if err != nil {
		t.Fatal(err)
	}
	keyID, err := key.ID()
	if err != nil {
		t.Fatal(err)
	}
	sv, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(keyBytes)
	if err != nil {
		t.Fatal(err)
	}

	err = r.AddTopLevelTargetsKey(context.Background(), keyBytes, keyBytes, false)
	assert.Nil(t, err)

	state, err := policy.LoadCurrentState(context.Background(), r.r)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata()
	assert.Nil(t, err)
	assert.Equal(t, 2, rootMetadata.Version)
	assert.Equal(t, keyID, rootMetadata.Roles[policy.RootRoleName].KeyIDs[0])
	assert.Equal(t, keyID, rootMetadata.Roles[policy.TargetsRoleName].KeyIDs[0])
	assert.Equal(t, keyID, state.RootEnvelope.Signatures[0].KeyID)

	err = dsse.VerifyEnvelope(context.Background(), state.RootEnvelope, []d.Verifier{sv}, 1)
	assert.Nil(t, err)
}

func TestRemoveTopLevelTargetsKey(t *testing.T) {
	r, keyBytes := createTestRepositoryWithRoot(t)

	rootKey, err := tuf.LoadKeyFromBytes(keyBytes)
	if err != nil {
		t.Fatal(err)
	}
	rootKeyID, err := rootKey.ID()
	if err != nil {
		t.Fatal(err)
	}
	sv, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(keyBytes)
	if err != nil {
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
	targetsKeyID, err := targetsKey.ID()
	if err != nil {
		t.Fatal(err)
	}

	err = r.AddTopLevelTargetsKey(context.Background(), keyBytes, targetsKeyBytes, false)
	if err != nil {
		t.Fatal(err)
	}

	state, err := policy.LoadCurrentState(context.Background(), r.r)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata()
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 3, rootMetadata.Version)
	assert.Equal(t, rootKeyID, rootMetadata.Roles[policy.TargetsRoleName].KeyIDs[0])
	assert.Contains(t, rootMetadata.Roles[policy.TargetsRoleName].KeyIDs, rootKeyID)
	assert.Contains(t, rootMetadata.Roles[policy.TargetsRoleName].KeyIDs, targetsKeyID)
	err = dsse.VerifyEnvelope(context.Background(), state.RootEnvelope, []d.Verifier{sv}, 1)
	assert.Nil(t, err)

	err = r.RemoveTopLevelTargetsKey(context.Background(), keyBytes, rootKeyID, false)
	assert.Nil(t, err)

	state, err = policy.LoadCurrentState(context.Background(), r.r)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err = state.GetRootMetadata()
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 4, rootMetadata.Version)
	assert.Contains(t, rootMetadata.Roles[policy.TargetsRoleName].KeyIDs, targetsKeyID)
	err = dsse.VerifyEnvelope(context.Background(), state.RootEnvelope, []d.Verifier{sv}, 1)
	assert.Nil(t, err)
}
