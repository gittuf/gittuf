// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"testing"

	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/signerverifier"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	sslibsv "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/signerverifier"
	"github.com/gittuf/gittuf/internal/tuf"
	sslibdsse "github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/stretchr/testify/assert"
)

func TestInitializeRoot(t *testing.T) {
	// The helper also runs InitializeRoot for this test
	r, rootKeyBytes := createTestRepositoryWithRoot(t, "")

	key, err := tuf.LoadKeyFromBytes(rootKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := sslibsv.NewVerifierFromSSLibKey(key)
	if err != nil {
		t.Fatal(err)
	}

	state, err := policy.LoadCurrentState(testCtx, r.r)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata()
	assert.Nil(t, err)
	assert.Equal(t, key.KeyID, rootMetadata.Roles[policy.RootRoleName].KeyIDs[0])
	assert.Equal(t, key.KeyID, state.RootEnvelope.Signatures[0].KeyID)

	err = dsse.VerifyEnvelope(testCtx, state.RootEnvelope, []sslibdsse.Verifier{verifier}, 1)
	assert.Nil(t, err)
}

func TestAddRootKey(t *testing.T) {
	r, keyBytes := createTestRepositoryWithRoot(t, "")

	sv, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(keyBytes) //nolint:staticcheck
	if err != nil {
		t.Fatal(err)
	}
	originalKeyID, err := sv.KeyID()
	if err != nil {
		t.Fatal(err)
	}

	var newRootKey *sslibsv.SSLibKey

	newRootKey, err = tuf.LoadKeyFromBytes(targetsKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	err = r.AddRootKey(testCtx, sv, newRootKey, false)
	assert.Nil(t, err)

	state, err := policy.LoadCurrentState(testCtx, r.r)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata()
	assert.Nil(t, err)
	assert.Equal(t, 2, rootMetadata.Version)
	assert.Equal(t, []string{originalKeyID, newRootKey.KeyID}, rootMetadata.Roles[policy.RootRoleName].KeyIDs)
	assert.Equal(t, originalKeyID, state.RootEnvelope.Signatures[0].KeyID)
	assert.Equal(t, 2, len(state.RootPublicKeys))

	err = dsse.VerifyEnvelope(testCtx, state.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)
}

func TestRemoveRootKey(t *testing.T) {
	r, keyBytes := createTestRepositoryWithRoot(t, "")

	rootKey, err := tuf.LoadKeyFromBytes(keyBytes)
	if err != nil {
		t.Fatal(err)
	}
	originalSigner, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(keyBytes) //nolint:staticcheck
	if err != nil {
		t.Fatal(err)
	}

	err = r.AddRootKey(testCtx, originalSigner, rootKey, false)
	if err != nil {
		t.Fatal(err)
	}

	state, err := policy.LoadCurrentState(testCtx, r.r)
	if err != nil {
		t.Fatal(err)
	}
	rootMetadata, err := state.GetRootMetadata()
	if err != nil {
		t.Fatal(err)
	}

	// We should have no additions as we tried to add the same key
	assert.Equal(t, 2, rootMetadata.Version)
	assert.Equal(t, 1, len(state.RootPublicKeys))
	assert.Equal(t, 1, len(rootMetadata.Roles[policy.RootRoleName].KeyIDs))

	newRootKey, err := tuf.LoadKeyFromBytes(targetsPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	err = r.AddRootKey(testCtx, originalSigner, newRootKey, false)
	if err != nil {
		t.Fatal(err)
	}

	state, err = policy.LoadCurrentState(testCtx, r.r)
	if err != nil {
		t.Fatal(err)
	}
	rootMetadata, err = state.GetRootMetadata()
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 3, rootMetadata.Version)
	assert.Contains(t, rootMetadata.Roles[policy.RootRoleName].KeyIDs, rootKey.KeyID)
	assert.Contains(t, rootMetadata.Roles[policy.RootRoleName].KeyIDs, newRootKey.KeyID)
	assert.Equal(t, 2, len(state.RootPublicKeys))

	err = dsse.VerifyEnvelope(testCtx, state.RootEnvelope, []sslibdsse.Verifier{originalSigner}, 1)
	assert.Nil(t, err)

	err = r.RemoveRootKey(testCtx, originalSigner, rootKey.KeyID, false)
	// Self root revocation currently is not supported
	// This is linked to the policy package comment about using prior state for
	// getRootVerifier
	assert.ErrorIs(t, err, policy.ErrVerifierConditionsUnmet)

	newSigner, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(targetsKeyBytes) //nolint:staticcheck
	if err != nil {
		t.Fatal(err)
	}

	// We can use the newly added root key to revoke the old one though
	err = r.RemoveRootKey(testCtx, newSigner, rootKey.KeyID, false)
	assert.Nil(t, err)

	state, err = policy.LoadCurrentState(testCtx, r.r)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err = state.GetRootMetadata()
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 4, rootMetadata.Version)
	assert.Contains(t, rootMetadata.Roles[policy.RootRoleName].KeyIDs, newRootKey.KeyID)
	assert.Equal(t, 1, len(rootMetadata.Roles[policy.RootRoleName].KeyIDs))
	assert.Equal(t, 1, len(state.RootPublicKeys))

	err = dsse.VerifyEnvelope(testCtx, state.RootEnvelope, []sslibdsse.Verifier{newSigner}, 1)
	assert.Nil(t, err)
}

func TestAddTopLevelTargetsKey(t *testing.T) {
	r, keyBytes := createTestRepositoryWithRoot(t, "")

	key, err := tuf.LoadKeyFromBytes(keyBytes)
	if err != nil {
		t.Fatal(err)
	}
	sv, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(keyBytes) //nolint:staticcheck
	if err != nil {
		t.Fatal(err)
	}

	err = r.AddTopLevelTargetsKey(testCtx, sv, key, false)
	assert.Nil(t, err)

	state, err := policy.LoadCurrentState(testCtx, r.r)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata()
	assert.Nil(t, err)
	assert.Equal(t, 2, rootMetadata.Version)
	assert.Equal(t, key.KeyID, rootMetadata.Roles[policy.RootRoleName].KeyIDs[0])
	assert.Equal(t, key.KeyID, rootMetadata.Roles[policy.TargetsRoleName].KeyIDs[0])
	assert.Equal(t, key.KeyID, state.RootEnvelope.Signatures[0].KeyID)

	err = dsse.VerifyEnvelope(testCtx, state.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)
}

func TestRemoveTopLevelTargetsKey(t *testing.T) {
	r, keyBytes := createTestRepositoryWithRoot(t, "")

	rootKey, err := tuf.LoadKeyFromBytes(keyBytes)
	if err != nil {
		t.Fatal(err)
	}
	sv, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(keyBytes) //nolint:staticcheck
	if err != nil {
		t.Fatal(err)
	}

	err = r.AddTopLevelTargetsKey(testCtx, sv, rootKey, false)
	if err != nil {
		t.Fatal(err)
	}

	targetsKey, err := tuf.LoadKeyFromBytes(targetsKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	err = r.AddTopLevelTargetsKey(testCtx, sv, targetsKey, false)
	if err != nil {
		t.Fatal(err)
	}

	state, err := policy.LoadCurrentState(testCtx, r.r)
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
	err = dsse.VerifyEnvelope(testCtx, state.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)

	err = r.RemoveTopLevelTargetsKey(testCtx, sv, rootKey.KeyID, false)
	assert.Nil(t, err)

	state, err = policy.LoadCurrentState(testCtx, r.r)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err = state.GetRootMetadata()
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 4, rootMetadata.Version)
	assert.Contains(t, rootMetadata.Roles[policy.TargetsRoleName].KeyIDs, targetsKey.KeyID)
	err = dsse.VerifyEnvelope(testCtx, state.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)
}
