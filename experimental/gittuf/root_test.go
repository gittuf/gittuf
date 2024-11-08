// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"testing"

	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
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

	rootMetadata, err := state.GetRootMetadata(false)
	assert.Nil(t, err)
	assert.Equal(t, key.KeyID, state.RootEnvelope.Signatures[0].KeyID)

	assert.True(t, getRootPrincipalIDs(t, rootMetadata).Has(key.KeyID))

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

	newRootKey := tufv01.NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targetsPubKeyBytes))

	err = r.AddRootKey(testCtx, sv, newRootKey, false)
	assert.Nil(t, err)

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata(false)
	assert.Nil(t, err)

	assert.Equal(t, originalKeyID, state.RootEnvelope.Signatures[0].KeyID)
	assert.Equal(t, 2, len(state.RootPublicKeys))

	assert.Equal(t, set.NewSetFromItems(originalKeyID, newRootKey.KeyID), getRootPrincipalIDs(t, rootMetadata))

	_, err = dsse.VerifyEnvelope(testCtx, state.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)
}

func TestRemoveRootKey(t *testing.T) {
	r := createTestRepositoryWithRoot(t, "")

	originalSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	rootKey := tufv01.NewKeyFromSSLibKey(originalSigner.MetadataKey())

	err := r.AddRootKey(testCtx, originalSigner, rootKey, false)
	if err != nil {
		t.Fatal(err)
	}

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata(false)
	if err != nil {
		t.Fatal(err)
	}

	// We should have no additions as we tried to add the same key
	assert.Equal(t, 1, len(state.RootPublicKeys))
	rootPrincipals, err := rootMetadata.GetRootPrincipals()
	assert.Nil(t, err)
	assert.Equal(t, 1, len(rootPrincipals))

	newRootKey := tufv01.NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targetsPubKeyBytes))

	err = r.AddRootKey(testCtx, originalSigner, newRootKey, false)
	if err != nil {
		t.Fatal(err)
	}

	state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}
	rootMetadata, err = state.GetRootMetadata(false)
	if err != nil {
		t.Fatal(err)
	}

	rootPrincipalIDs := getRootPrincipalIDs(t, rootMetadata)
	assert.True(t, rootPrincipalIDs.Has(rootKey.KeyID))
	assert.True(t, rootPrincipalIDs.Has(newRootKey.KeyID))
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

	rootMetadata, err = state.GetRootMetadata(false)
	if err != nil {
		t.Fatal(err)
	}

	rootPrincipalIDs = getRootPrincipalIDs(t, rootMetadata)
	assert.True(t, rootPrincipalIDs.Has(newRootKey.KeyID))
	assert.Equal(t, 1, rootPrincipalIDs.Len())
	assert.Equal(t, 1, len(state.RootPublicKeys))

	_, err = dsse.VerifyEnvelope(testCtx, state.RootEnvelope, []sslibdsse.Verifier{newSigner}, 1)
	assert.Nil(t, err)
}

func TestAddTopLevelTargetsKey(t *testing.T) {
	r := createTestRepositoryWithRoot(t, "")

	sv := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	key := tufv01.NewKeyFromSSLibKey(sv.MetadataKey())

	err := r.AddTopLevelTargetsKey(testCtx, sv, key, false)
	assert.Nil(t, err)

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata(false)
	assert.Nil(t, err)
	assert.Equal(t, key.KeyID, state.RootEnvelope.Signatures[0].KeyID)
	assert.True(t, getRootPrincipalIDs(t, rootMetadata).Has(key.KeyID))
	assert.True(t, getPrimaryRuleFilePrincipalIDs(t, rootMetadata).Has(key.KeyID))

	_, err = dsse.VerifyEnvelope(testCtx, state.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)
}

func TestRemoveTopLevelTargetsKey(t *testing.T) {
	r := createTestRepositoryWithRoot(t, "")

	sv := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	rootKey := tufv01.NewKeyFromSSLibKey(sv.MetadataKey())

	err := r.AddTopLevelTargetsKey(testCtx, sv, rootKey, false)
	if err != nil {
		t.Fatal(err)
	}

	targetsKey := tufv01.NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targetsPubKeyBytes))

	err = r.AddTopLevelTargetsKey(testCtx, sv, targetsKey, false)
	if err != nil {
		t.Fatal(err)
	}

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata(false)
	if err != nil {
		t.Fatal(err)
	}

	targetsPrincipalIDs := getPrimaryRuleFilePrincipalIDs(t, rootMetadata)
	assert.True(t, targetsPrincipalIDs.Has(rootKey.KeyID))
	assert.True(t, targetsPrincipalIDs.Has(targetsKey.KeyID))

	_, err = dsse.VerifyEnvelope(testCtx, state.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)

	err = r.RemoveTopLevelTargetsKey(testCtx, sv, rootKey.KeyID, false)
	assert.Nil(t, err)

	state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err = state.GetRootMetadata(false)
	if err != nil {
		t.Fatal(err)
	}

	targetsPrincipalIDs = getPrimaryRuleFilePrincipalIDs(t, rootMetadata)
	assert.True(t, targetsPrincipalIDs.Has(targetsKey.KeyID))
	_, err = dsse.VerifyEnvelope(testCtx, state.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)
}

func TestAddGitHubAppKey(t *testing.T) {
	r := createTestRepositoryWithRoot(t, "")

	sv := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	key := tufv01.NewKeyFromSSLibKey(sv.MetadataKey())

	err := r.AddGitHubAppKey(testCtx, sv, key, false)
	assert.Nil(t, err)

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata(false)
	assert.Nil(t, err)

	appPrincipals, err := rootMetadata.GetGitHubAppPrincipals()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, key, appPrincipals[0])

	_, err = dsse.VerifyEnvelope(testCtx, state.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)
}

func TestRemoveGitHubAppKey(t *testing.T) {
	r := createTestRepositoryWithRoot(t, "")

	sv := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	key := tufv01.NewKeyFromSSLibKey(sv.MetadataKey())

	err := r.AddGitHubAppKey(testCtx, sv, key, false)
	if err != nil {
		t.Fatal(err)
	}

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata(false)
	if err != nil {
		t.Fatal(err)
	}

	appPrincipals, err := rootMetadata.GetGitHubAppPrincipals()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, key, appPrincipals[0])

	_, err = dsse.VerifyEnvelope(testCtx, state.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)

	err = r.RemoveGitHubAppKey(testCtx, sv, false)
	assert.Nil(t, err)

	state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err = state.GetRootMetadata(false)
	if err != nil {
		t.Fatal(err)
	}

	appPrincipals, err = rootMetadata.GetGitHubAppPrincipals()
	// We see an error (correctly that the app is trusted but no key is present)
	assert.ErrorIs(t, err, tuf.ErrGitHubAppInformationNotFoundInRoot)
	assert.Empty(t, appPrincipals)

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
		assert.ErrorIs(t, err, tuf.ErrGitHubAppInformationNotFoundInRoot)
	})

	t.Run("GitHub app role defined", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")

		sv := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
		key := tufv01.NewKeyFromSSLibKey(sv.MetadataKey())

		state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		rootMetadata, err := state.GetRootMetadata(false)
		assert.Nil(t, err)

		assert.False(t, rootMetadata.IsGitHubAppApprovalTrusted())

		err = r.AddGitHubAppKey(testCtx, sv, key, false)
		assert.Nil(t, err)

		err = r.TrustGitHubApp(testCtx, sv, false)
		assert.Nil(t, err)

		state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		rootMetadata, err = state.GetRootMetadata(false)
		assert.Nil(t, err)

		assert.True(t, rootMetadata.IsGitHubAppApprovalTrusted())
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
	key := tufv01.NewKeyFromSSLibKey(sv.MetadataKey())

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata(false)
	assert.Nil(t, err)

	assert.False(t, rootMetadata.IsGitHubAppApprovalTrusted())

	err = r.AddGitHubAppKey(testCtx, sv, key, false)
	assert.Nil(t, err)

	err = r.TrustGitHubApp(testCtx, sv, false)
	assert.Nil(t, err)

	state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err = state.GetRootMetadata(false)
	assert.Nil(t, err)

	assert.True(t, rootMetadata.IsGitHubAppApprovalTrusted())
	_, err = dsse.VerifyEnvelope(testCtx, state.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)

	err = r.UntrustGitHubApp(testCtx, sv, false)
	assert.Nil(t, err)

	state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err = state.GetRootMetadata(false)
	assert.Nil(t, err)

	assert.False(t, rootMetadata.IsGitHubAppApprovalTrusted())
	_, err = dsse.VerifyEnvelope(testCtx, state.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)
}

func TestUpdateRootThreshold(t *testing.T) {
	r := createTestRepositoryWithRoot(t, "")

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata(false)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, getRootPrincipalIDs(t, rootMetadata).Len())

	rootThreshold, err := rootMetadata.GetRootThreshold()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, rootThreshold)

	signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

	secondKey := tufv01.NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targetsPubKeyBytes))

	if err := r.AddRootKey(testCtx, signer, secondKey, false); err != nil {
		t.Fatal(err)
	}

	err = r.UpdateRootThreshold(testCtx, signer, 2, false)
	assert.Nil(t, err)

	state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err = state.GetRootMetadata(false)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 2, getRootPrincipalIDs(t, rootMetadata).Len())

	rootThreshold, err = rootMetadata.GetRootThreshold()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 2, rootThreshold)
}

func TestUpdateTopLevelTargetsThreshold(t *testing.T) {
	r := createTestRepositoryWithRoot(t, "")

	sv := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	key := tufv01.NewKeyFromSSLibKey(sv.MetadataKey())

	if err := r.AddTopLevelTargetsKey(testCtx, sv, key, false); err != nil {
		t.Fatal(err)
	}

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata(false)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, getPrimaryRuleFilePrincipalIDs(t, rootMetadata).Len())

	targetsThreshold, err := rootMetadata.GetPrimaryRuleFileThreshold()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, targetsThreshold)

	targetsKey := tufv01.NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targetsPubKeyBytes))

	if err := r.AddTopLevelTargetsKey(testCtx, sv, targetsKey, false); err != nil {
		t.Fatal(err)
	}

	err = r.UpdateTopLevelTargetsThreshold(testCtx, sv, 2, false)
	assert.Nil(t, err)

	state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err = state.GetRootMetadata(false)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 2, getPrimaryRuleFilePrincipalIDs(t, rootMetadata).Len())

	targetsThreshold, err = rootMetadata.GetPrimaryRuleFileThreshold()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 2, targetsThreshold)
}

func TestSignRoot(t *testing.T) {
	r := createTestRepositoryWithRoot(t, "")

	rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

	// Add targets key as a root key
	secondKey := tufv01.NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targetsPubKeyBytes))
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

func getRootPrincipalIDs(t *testing.T, rootMetadata tuf.RootMetadata) *set.Set[string] {
	t.Helper()

	principals, err := rootMetadata.GetRootPrincipals()
	if err != nil {
		t.Fatal(err)
	}

	principalIDs := set.NewSet[string]()
	for _, principal := range principals {
		principalIDs.Add(principal.ID())
	}

	return principalIDs
}

func getPrimaryRuleFilePrincipalIDs(t *testing.T, rootMetadata tuf.RootMetadata) *set.Set[string] {
	t.Helper()

	principals, err := rootMetadata.GetPrimaryRuleFilePrincipals()
	if err != nil {
		t.Fatal(err)
	}

	principalIDs := set.NewSet[string]()
	for _, principal := range principals {
		principalIDs.Add(principal.ID())
	}

	return principalIDs
}
