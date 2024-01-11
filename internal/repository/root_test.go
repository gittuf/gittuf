// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"testing"

	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/signerverifier"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	sslibdsse "github.com/secure-systems-lab/go-securesystemslib/dsse"
	sslibsignerverifier "github.com/secure-systems-lab/go-securesystemslib/signerverifier"
	"github.com/stretchr/testify/assert"
)

func TestInitializeRoot(t *testing.T) {
	// The helper also runs InitializeRoot for this test
	r, rootKeyBytes := createTestRepositoryWithRoot(t, "")

	key, err := tuf.LoadKeyFromBytes(rootKeyBytes)
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
	assert.Equal(t, key.KeyID, rootMetadata.Roles[policy.RootRoleName].KeyIDs[0])
	assert.Equal(t, key.KeyID, state.RootEnvelope.Signatures[0].KeyID)

	err = dsse.VerifyEnvelope(context.Background(), state.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)
}

func TestAddRootKey(t *testing.T) {
	r, keyBytes := createTestRepositoryWithRoot(t, "")

	key, err := tuf.LoadKeyFromBytes(keyBytes)
	if err != nil {
		t.Fatal(err)
	}
	sv, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(keyBytes)
	if err != nil {
		t.Fatal(err)
	}

	var newRootKey *sslibsignerverifier.SSLibKey

	newRootKey, err = tuf.LoadKeyFromBytes(targetsKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	err = r.AddRootKey(context.Background(), keyBytes, newRootKey, false)
	assert.Nil(t, err)

	state, err := policy.LoadCurrentState(context.Background(), r.r)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata()
	assert.Nil(t, err)
	assert.Equal(t, 2, rootMetadata.Version)
	assert.Equal(t, []string{key.KeyID, newRootKey.KeyID}, rootMetadata.Roles[policy.RootRoleName].KeyIDs)
	assert.Equal(t, key.KeyID, state.RootEnvelope.Signatures[0].KeyID)

	err = dsse.VerifyEnvelope(context.Background(), state.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)
}

func TestRemoveRootKey(t *testing.T) {
	r, keyBytes := createTestRepositoryWithRoot(t, "")

	rootKey, err := tuf.LoadKeyFromBytes(keyBytes)
	if err != nil {
		t.Fatal(err)
	}
	sv, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(keyBytes)
	if err != nil {
		t.Fatal(err)
	}

	err = r.AddRootKey(context.Background(), keyBytes, rootKey, false)
	if err != nil {
		t.Fatal(err)
	}

	newRootKey, err := tuf.LoadKeyFromBytes(targetsKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	err = r.AddRootKey(context.Background(), keyBytes, newRootKey, false)
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
	assert.Contains(t, rootMetadata.Roles[policy.RootRoleName].KeyIDs, rootKey.KeyID)
	assert.Contains(t, rootMetadata.Roles[policy.RootRoleName].KeyIDs, newRootKey.KeyID)
	err = dsse.VerifyEnvelope(context.Background(), state.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)

	err = r.RemoveRootKey(context.Background(), keyBytes, rootKey.KeyID, false)
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
	assert.Contains(t, rootMetadata.Roles[policy.RootRoleName].KeyIDs, newRootKey.KeyID)
	err = dsse.VerifyEnvelope(context.Background(), state.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)
}

func TestAddTopLevelTargetsKey(t *testing.T) {
	r, keyBytes := createTestRepositoryWithRoot(t, "")

	key, err := tuf.LoadKeyFromBytes(keyBytes)
	if err != nil {
		t.Fatal(err)
	}
	sv, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(keyBytes)
	if err != nil {
		t.Fatal(err)
	}

	err = r.AddTopLevelTargetsKey(context.Background(), keyBytes, key, false)
	assert.Nil(t, err)

	state, err := policy.LoadCurrentState(context.Background(), r.r)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata()
	assert.Nil(t, err)
	assert.Equal(t, 2, rootMetadata.Version)
	assert.Equal(t, key.KeyID, rootMetadata.Roles[policy.RootRoleName].KeyIDs[0])
	assert.Equal(t, key.KeyID, rootMetadata.Roles[policy.TargetsRoleName].KeyIDs[0])
	assert.Equal(t, key.KeyID, state.RootEnvelope.Signatures[0].KeyID)

	err = dsse.VerifyEnvelope(context.Background(), state.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)
}

func TestRemoveTopLevelTargetsKey(t *testing.T) {
	r, keyBytes := createTestRepositoryWithRoot(t, "")

	rootKey, err := tuf.LoadKeyFromBytes(keyBytes)
	if err != nil {
		t.Fatal(err)
	}
	sv, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(keyBytes)
	if err != nil {
		t.Fatal(err)
	}

	err = r.AddTopLevelTargetsKey(context.Background(), keyBytes, rootKey, false)
	if err != nil {
		t.Fatal(err)
	}

	targetsKey, err := tuf.LoadKeyFromBytes(targetsKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	err = r.AddTopLevelTargetsKey(context.Background(), keyBytes, targetsKey, false)
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
	assert.Equal(t, rootKey.KeyID, rootMetadata.Roles[policy.TargetsRoleName].KeyIDs[0])
	assert.Contains(t, rootMetadata.Roles[policy.TargetsRoleName].KeyIDs, rootKey.KeyID)
	assert.Contains(t, rootMetadata.Roles[policy.TargetsRoleName].KeyIDs, targetsKey.KeyID)
	err = dsse.VerifyEnvelope(context.Background(), state.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)

	err = r.RemoveTopLevelTargetsKey(context.Background(), keyBytes, rootKey.KeyID, false)
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
	assert.Contains(t, rootMetadata.Roles[policy.TargetsRoleName].KeyIDs, targetsKey.KeyID)
	err = dsse.VerifyEnvelope(context.Background(), state.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)
}
