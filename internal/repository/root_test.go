// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"testing"

	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/stretchr/testify/assert"
)

func TestInitializeRoot(t *testing.T) {
	// The helper also runs InitializeRoot for this test
	r := createTestRepositoryWithRoot(t, "")

	key := ssh.NewKeyFromBytes(t, rootPubKeyBytes)
	verifier, err := ssh.NewVerifierFromKey(key)
	if err != nil {
		t.Fatal(err)
	}

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata()
	assert.Nil(t, err)
	assert.True(t, rootMetadata.Roles[policy.RootRoleName].KeyIDs.Has(key.KeyID))
	assert.Equal(t, key.KeyID, state.RootEnvelope.Signatures[0].KeyID)

	_, err = dsse.VerifyEnvelope(testCtx, state.RootEnvelope, []sslibdsse.Verifier{verifier}, 1)
	assert.Nil(t, err)
}

func TestAddRootKey(t *testing.T) {
	r := createTestRepositoryWithRoot(t, "")

	sv := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	originalKeyID, err := sv.KeyID()
	if err != nil {
		t.Fatal(err)
	}

	newRootKey := ssh.NewKeyFromBytes(t, targetsPubKeyBytes)

	err = r.AddRootKey(testCtx, sv, newRootKey, false)
	assert.Nil(t, err)

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata()
	assert.Nil(t, err)
	assert.Equal(t, set.NewSetFromItems(originalKeyID, newRootKey.KeyID), rootMetadata.Roles[policy.RootRoleName].KeyIDs)
	assert.Equal(t, originalKeyID, state.RootEnvelope.Signatures[0].KeyID)
	assert.Equal(t, 2, len(state.RootPublicKeys))

	_, err = dsse.VerifyEnvelope(testCtx, state.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)
}

func TestRemoveRootKey(t *testing.T) {
	r := createTestRepositoryWithRoot(t, "")

	originalSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	rootKey := originalSigner.MetadataKey()

	err := r.AddRootKey(testCtx, originalSigner, rootKey, false)
	if err != nil {
		t.Fatal(err)
	}

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata()
	if err != nil {
		t.Fatal(err)
	}

	// We should have no additions as we tried to add the same key
	assert.Equal(t, 1, len(state.RootPublicKeys))
	assert.Equal(t, 1, rootMetadata.Roles[policy.RootRoleName].KeyIDs.Len())

	newRootKey := ssh.NewKeyFromBytes(t, targetsPubKeyBytes)

	err = r.AddRootKey(testCtx, originalSigner, newRootKey, false)
	if err != nil {
		t.Fatal(err)
	}

	state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}
	rootMetadata, err = state.GetRootMetadata()
	if err != nil {
		t.Fatal(err)
	}

	assert.True(t, rootMetadata.Roles[policy.RootRoleName].KeyIDs.Has(rootKey.KeyID))
	assert.True(t, rootMetadata.Roles[policy.RootRoleName].KeyIDs.Has(newRootKey.KeyID))
	assert.Equal(t, 2, len(state.RootPublicKeys))

	_, err = dsse.VerifyEnvelope(testCtx, state.RootEnvelope, []sslibdsse.Verifier{originalSigner}, 1)
	assert.Nil(t, err)

	newSigner := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)

	// We can use the newly added root key to revoke the old one
	err = r.RemoveRootKey(testCtx, newSigner, rootKey.KeyID, false)
	assert.Nil(t, err)

	state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err = state.GetRootMetadata()
	if err != nil {
		t.Fatal(err)
	}

	assert.True(t, rootMetadata.Roles[policy.RootRoleName].KeyIDs.Has(newRootKey.KeyID))
	assert.Equal(t, 1, rootMetadata.Roles[policy.RootRoleName].KeyIDs.Len())
	assert.Equal(t, 1, len(state.RootPublicKeys))

	_, err = dsse.VerifyEnvelope(testCtx, state.RootEnvelope, []sslibdsse.Verifier{newSigner}, 1)
	assert.Nil(t, err)
}

func TestAddTopLevelTargetsKey(t *testing.T) {
	r := createTestRepositoryWithRoot(t, "")

	sv := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	key := sv.MetadataKey()

	err := r.AddTopLevelTargetsKey(testCtx, sv, key, false)
	assert.Nil(t, err)

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata()
	assert.Nil(t, err)
	assert.True(t, rootMetadata.Roles[policy.RootRoleName].KeyIDs.Has(key.KeyID))
	assert.True(t, rootMetadata.Roles[policy.TargetsRoleName].KeyIDs.Has(key.KeyID))
	assert.Equal(t, key.KeyID, state.RootEnvelope.Signatures[0].KeyID)

	_, err = dsse.VerifyEnvelope(testCtx, state.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)
}

func TestRemoveTopLevelTargetsKey(t *testing.T) {
	r := createTestRepositoryWithRoot(t, "")

	sv := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	rootKey := sv.MetadataKey()

	err := r.AddTopLevelTargetsKey(testCtx, sv, rootKey, false)
	if err != nil {
		t.Fatal(err)
	}

	targetsKey := ssh.NewKeyFromBytes(t, targetsPubKeyBytes)

	err = r.AddTopLevelTargetsKey(testCtx, sv, targetsKey, false)
	if err != nil {
		t.Fatal(err)
	}

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata()
	if err != nil {
		t.Fatal(err)
	}

	assert.True(t, rootMetadata.Roles[policy.TargetsRoleName].KeyIDs.Has(rootKey.KeyID))
	assert.True(t, rootMetadata.Roles[policy.TargetsRoleName].KeyIDs.Has(targetsKey.KeyID))
	_, err = dsse.VerifyEnvelope(testCtx, state.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)

	err = r.RemoveTopLevelTargetsKey(testCtx, sv, rootKey.KeyID, false)
	assert.Nil(t, err)

	state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err = state.GetRootMetadata()
	if err != nil {
		t.Fatal(err)
	}

	assert.True(t, rootMetadata.Roles[policy.TargetsRoleName].KeyIDs.Has(targetsKey.KeyID))
	_, err = dsse.VerifyEnvelope(testCtx, state.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)
}

func TestAddGitHubAppKey(t *testing.T) {
	r := createTestRepositoryWithRoot(t, "")

	sv := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	key := sv.MetadataKey()

	err := r.AddGitHubAppKey(testCtx, sv, key, false)
	assert.Nil(t, err)

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata()
	assert.Nil(t, err)

	assert.True(t, rootMetadata.Roles[policy.GitHubAppRoleName].KeyIDs.Has(key.KeyID))
	_, err = dsse.VerifyEnvelope(testCtx, state.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)
}

func TestRemoveGitHubAppKey(t *testing.T) {
	r := createTestRepositoryWithRoot(t, "")

	sv := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	key := sv.MetadataKey()

	err := r.AddGitHubAppKey(testCtx, sv, key, false)
	if err != nil {
		t.Fatal(err)
	}

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata()
	if err != nil {
		t.Fatal(err)
	}

	assert.True(t, rootMetadata.Roles[policy.GitHubAppRoleName].KeyIDs.Has(key.KeyID))
	_, err = dsse.VerifyEnvelope(testCtx, state.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)

	err = r.RemoveGitHubAppKey(testCtx, sv, false)
	assert.Nil(t, err)

	state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err = state.GetRootMetadata()
	if err != nil {
		t.Fatal(err)
	}

	assert.Empty(t, rootMetadata.Roles[policy.GitHubAppRoleName].KeyIDs)
	_, err = dsse.VerifyEnvelope(testCtx, state.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)
}

func TestTrustGitHubApp(t *testing.T) {
	t.Run("GitHub app role not defined", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")

		sv := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

		err := r.TrustGitHubApp(testCtx, sv, false)
		assert.Nil(t, err)

		_, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
		assert.ErrorIs(t, err, policy.ErrNoGitHubAppRoleDeclared)
	})

	t.Run("GitHub app role defined", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")

		sv := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
		key := sv.MetadataKey()

		state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		rootMetadata, err := state.GetRootMetadata()
		assert.Nil(t, err)

		assert.False(t, rootMetadata.GitHubApprovalsTrusted)

		err = r.AddGitHubAppKey(testCtx, sv, key, false)
		assert.Nil(t, err)

		err = r.TrustGitHubApp(testCtx, sv, false)
		assert.Nil(t, err)

		state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		rootMetadata, err = state.GetRootMetadata()
		assert.Nil(t, err)

		assert.True(t, rootMetadata.GitHubApprovalsTrusted)
		_, err = dsse.VerifyEnvelope(testCtx, state.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
		assert.Nil(t, err)

		// Test if we can trust again if already trusted
		err = r.TrustGitHubApp(testCtx, sv, false)
		assert.Nil(t, err)
	})
}

func TestUntrustGitHubApp(t *testing.T) {
	r := createTestRepositoryWithRoot(t, "")

	sv := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	key := sv.MetadataKey()

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata()
	assert.Nil(t, err)

	assert.False(t, rootMetadata.GitHubApprovalsTrusted)

	err = r.AddGitHubAppKey(testCtx, sv, key, false)
	assert.Nil(t, err)

	err = r.TrustGitHubApp(testCtx, sv, false)
	assert.Nil(t, err)

	state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err = state.GetRootMetadata()
	assert.Nil(t, err)

	assert.True(t, rootMetadata.GitHubApprovalsTrusted)
	_, err = dsse.VerifyEnvelope(testCtx, state.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)

	err = r.UntrustGitHubApp(testCtx, sv, false)
	assert.Nil(t, err)

	state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err = state.GetRootMetadata()
	assert.Nil(t, err)

	assert.False(t, rootMetadata.GitHubApprovalsTrusted)
	_, err = dsse.VerifyEnvelope(testCtx, state.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)
}

func TestUpdateRootThreshold(t *testing.T) {
	r := createTestRepositoryWithRoot(t, "")

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata()
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, rootMetadata.Roles[policy.RootRoleName].KeyIDs.Len())
	assert.Equal(t, 1, rootMetadata.Roles[policy.RootRoleName].Threshold)

	signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

	secondKey := ssh.NewKeyFromBytes(t, targetsPubKeyBytes)

	if err := r.AddRootKey(testCtx, signer, secondKey, false); err != nil {
		t.Fatal(err)
	}

	err = r.UpdateRootThreshold(testCtx, signer, 2, false)
	assert.Nil(t, err)

	state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err = state.GetRootMetadata()
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 2, rootMetadata.Roles[policy.RootRoleName].KeyIDs.Len())
	assert.Equal(t, 2, rootMetadata.Roles[policy.RootRoleName].Threshold)
}

func TestUpdateTopLevelTargetsThreshold(t *testing.T) {
	r := createTestRepositoryWithRoot(t, "")

	sv := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	key := sv.MetadataKey()

	if err := r.AddTopLevelTargetsKey(testCtx, sv, key, false); err != nil {
		t.Fatal(err)
	}

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata()
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, rootMetadata.Roles[policy.TargetsRoleName].KeyIDs.Len())
	assert.Equal(t, 1, rootMetadata.Roles[policy.TargetsRoleName].Threshold)

	targetsKey := ssh.NewKeyFromBytes(t, targetsPubKeyBytes)

	if err := r.AddTopLevelTargetsKey(testCtx, sv, targetsKey, false); err != nil {
		t.Fatal(err)
	}

	err = r.UpdateTopLevelTargetsThreshold(testCtx, sv, 2, false)
	assert.Nil(t, err)

	state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err = state.GetRootMetadata()
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 2, rootMetadata.Roles[policy.TargetsRoleName].KeyIDs.Len())
	assert.Equal(t, 2, rootMetadata.Roles[policy.TargetsRoleName].Threshold)
}

func TestSignRoot(t *testing.T) {
	r := createTestRepositoryWithRoot(t, "")

	rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

	// Add targets key as a root key
	secondKey := ssh.NewKeyFromBytes(t, targetsPubKeyBytes)
	if err := r.AddRootKey(testCtx, rootSigner, secondKey, false); err != nil {
		t.Fatal(err)
	}

	secondSigner := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)

	// Add signature to root
	err := r.SignRoot(testCtx, secondSigner, false)
	assert.Nil(t, err)

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 2, len(state.RootEnvelope.Signatures))
}
