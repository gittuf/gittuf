// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"testing"

	rootopts "github.com/gittuf/gittuf/experimental/gittuf/options/root"
	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitializeRoot(t *testing.T) {
	t.Run("no repository location", func(t *testing.T) {
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
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, key.KeyID, state.RootEnvelope.Signatures[0].KeyID)

		assert.True(t, getRootPrincipalIDs(t, rootMetadata).Has(key.KeyID))

		_, err = dsse.VerifyEnvelope(testCtx, state.RootEnvelope, []sslibdsse.Verifier{verifier}, 1)
		assert.Nil(t, err)
	})

	t.Run("with repository location", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		r := &Repository{r: repo}

		signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

		location := "https://example.com/repository/location"
		err := r.InitializeRoot(testCtx, signer, false, rootopts.WithRepositoryLocation(location))
		assert.Nil(t, err)

		if err := policy.Apply(testCtx, repo, false); err != nil {
			t.Fatalf("failed to apply policy staging changes into policy, err = %s", err)
		}

		state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyRef)
		if err != nil {
			t.Fatal(err)
		}

		rootMetadata, err := state.GetRootMetadata(false)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, location, rootMetadata.GetRepositoryLocation())
	})
}

func TestSetRepositoryLocation(t *testing.T) {
	r := createTestRepositoryWithRoot(t, "")

	sv := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

	location := "https://example.com/repository/location"
	err := r.SetRepositoryLocation(testCtx, sv, location, false)
	assert.Nil(t, err)

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata(false)
	assert.Nil(t, err)
	assert.Equal(t, location, rootMetadata.GetRepositoryLocation())
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

func TestAddGitHubApp(t *testing.T) {
	r := createTestRepositoryWithRoot(t, "")

	sv := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	key := tufv01.NewKeyFromSSLibKey(sv.MetadataKey())

	err := r.AddGitHubApp(testCtx, sv, key, false)
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

func TestRemoveGitHubApp(t *testing.T) {
	r := createTestRepositoryWithRoot(t, "")

	sv := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	key := tufv01.NewKeyFromSSLibKey(sv.MetadataKey())

	err := r.AddGitHubApp(testCtx, sv, key, false)
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

	err = r.RemoveGitHubApp(testCtx, sv, false)
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

		err = r.AddGitHubApp(testCtx, sv, key, false)
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

	err = r.AddGitHubApp(testCtx, sv, key, false)
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

func TestAddGlobalRuleThreshold(t *testing.T) {
	t.Setenv(dev.DevModeKey, "1")

	r := createTestRepositoryWithRoot(t, "")

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata(false)
	if err != nil {
		t.Fatal(err)
	}

	globalRules := rootMetadata.GetGlobalRules()
	assert.Empty(t, globalRules)

	rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

	err = r.AddGlobalRuleThreshold(testCtx, rootSigner, "require-approval-for-main", []string{"git:refs/heads/main"}, 1, false)
	assert.Nil(t, err)

	state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef) // we haven't applied
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err = state.GetRootMetadata(false)
	if err != nil {
		t.Fatal(err)
	}

	globalRules = rootMetadata.GetGlobalRules()
	assert.Len(t, globalRules, 1)
	assert.Equal(t, "require-approval-for-main", globalRules[0].GetName())
	assert.Equal(t, []string{"git:refs/heads/main"}, globalRules[0].(tuf.GlobalRuleThreshold).GetProtectedNamespaces())
	assert.Equal(t, 1, globalRules[0].(tuf.GlobalRuleThreshold).GetThreshold())

	err = r.AddGlobalRuleThreshold(testCtx, rootSigner, "require-approval-for-main", []string{"git:refs/heads/main"}, 1, false)
	assert.ErrorIs(t, err, tuf.ErrGlobalRuleAlreadyExists)
}

func TestAddGlobalRuleBlockForcePushes(t *testing.T) {
	t.Setenv(dev.DevModeKey, "1")

	r := createTestRepositoryWithRoot(t, "")

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata(false)
	if err != nil {
		t.Fatal(err)
	}

	globalRules := rootMetadata.GetGlobalRules()
	assert.Empty(t, globalRules)

	rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

	err = r.AddGlobalRuleBlockForcePushes(testCtx, rootSigner, "block-force-pushes-for-main", []string{"git:refs/heads/main"}, false)
	assert.Nil(t, err)

	state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef) // we haven't applied
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err = state.GetRootMetadata(false)
	if err != nil {
		t.Fatal(err)
	}

	globalRules = rootMetadata.GetGlobalRules()
	assert.Len(t, globalRules, 1)
	assert.Equal(t, "block-force-pushes-for-main", globalRules[0].GetName())
	assert.Equal(t, []string{"git:refs/heads/main"}, globalRules[0].(tuf.GlobalRuleBlockForcePushes).GetProtectedNamespaces())
}
func TestRemoveGlobalRule(t *testing.T) {
	t.Setenv(dev.DevModeKey, "1")

	t.Run("remove threshold global rule", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")

		rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

		err := r.AddGlobalRuleThreshold(testCtx, rootSigner, "require-approval-for-main", []string{"git:refs/heads/main"}, 1, false)
		assert.Nil(t, err)

		state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		rootMetadata, err := state.GetRootMetadata(false)
		if err != nil {
			t.Fatal(err)
		}

		globalRules := rootMetadata.GetGlobalRules()
		assert.Len(t, globalRules, 1)

		err = r.RemoveGlobalRule(testCtx, rootSigner, "require-approval-for-main", false)
		assert.Nil(t, err)

		state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		rootMetadata, err = state.GetRootMetadata(false)
		if err != nil {
			t.Fatal(err)
		}

		globalRules = rootMetadata.GetGlobalRules()
		assert.Empty(t, globalRules)
	})

	t.Run("remove force push global rule", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")

		rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

		err := r.AddGlobalRuleBlockForcePushes(testCtx, rootSigner, "block-force-pushes-for-main", []string{"git:refs/heads/main"}, false)
		assert.Nil(t, err)

		state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		rootMetadata, err := state.GetRootMetadata(false)
		if err != nil {
			t.Fatal(err)
		}

		globalRules := rootMetadata.GetGlobalRules()
		assert.Len(t, globalRules, 1)

		err = r.RemoveGlobalRule(testCtx, rootSigner, "block-force-pushes-for-main", false)
		assert.Nil(t, err)

		state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		rootMetadata, err = state.GetRootMetadata(false)
		if err != nil {
			t.Fatal(err)
		}

		globalRules = rootMetadata.GetGlobalRules()
		assert.Empty(t, globalRules)
	})

	t.Run("remove global rule when none exist", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")

		rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

		state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		rootMetadata, err := state.GetRootMetadata(false)
		if err != nil {
			t.Fatal(err)
		}

		globalRules := rootMetadata.GetGlobalRules()
		assert.Empty(t, globalRules)

		err = r.RemoveGlobalRule(testCtx, rootSigner, "require-approval-for-main", false)
		assert.ErrorIs(t, err, tuf.ErrGlobalRuleNotFound)
	})
}

func TestListGlobalRules(t *testing.T) {
	t.Setenv(dev.DevModeKey, "1")

	t.Run("list global rules after add and remove", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")

		rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

		err := r.AddGlobalRuleThreshold(testCtx, rootSigner, "require-approval-for-main", []string{"git:refs/heads/main"}, 1, false)
		assert.Nil(t, err)

		globalRules, err := r.ListGlobalRules(testCtx)
		assert.Nil(t, err)
		assert.Len(t, globalRules, 1)

		err = r.RemoveGlobalRule(testCtx, rootSigner, "require-approval-for-main", false)
		assert.Nil(t, err)

		globalRules, err = r.ListGlobalRules(testCtx)
		assert.Nil(t, err)
		assert.Empty(t, globalRules)
	})
}

func TestAddPropagationDirective(t *testing.T) {
	t.Setenv(dev.DevModeKey, "1")

	t.Run("with tuf v01 metadata", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")

		state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyRef)
		if err != nil {
			t.Fatal(err)
		}

		rootMetadata, err := state.GetRootMetadata(false)
		if err != nil {
			t.Fatal(err)
		}

		directives := rootMetadata.GetPropagationDirectives()
		assert.Empty(t, directives)

		rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

		err = r.AddPropagationDirective(testCtx, rootSigner, "test", "https://example.com/git/repository", "refs/heads/main", "refs/heads/main", "upstream/", false)
		assert.Nil(t, err)

		state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef) // we haven't applied
		if err != nil {
			t.Fatal(err)
		}

		rootMetadata, err = state.GetRootMetadata(false)
		if err != nil {
			t.Fatal(err)
		}

		directives = rootMetadata.GetPropagationDirectives()
		assert.Len(t, directives, 1)
		assert.Equal(t, tufv01.NewPropagationDirective("test", "https://example.com/git/repository", "refs/heads/main", "refs/heads/main", "upstream/"), directives[0])
	})

	t.Run("with tuf v02 metadata", func(t *testing.T) {
		t.Setenv(tufv02.AllowV02MetadataKey, "1")

		r := createTestRepositoryWithRoot(t, "")

		state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyRef)
		if err != nil {
			t.Fatal(err)
		}

		rootMetadata, err := state.GetRootMetadata(false)
		if err != nil {
			t.Fatal(err)
		}

		directives := rootMetadata.GetPropagationDirectives()
		assert.Empty(t, directives)

		rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

		err = r.AddPropagationDirective(testCtx, rootSigner, "test", "https://example.com/git/repository", "refs/heads/main", "refs/heads/main", "upstream/", false)
		assert.Nil(t, err)

		state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef) // we haven't applied
		if err != nil {
			t.Fatal(err)
		}

		rootMetadata, err = state.GetRootMetadata(false)
		if err != nil {
			t.Fatal(err)
		}

		directives = rootMetadata.GetPropagationDirectives()
		assert.Len(t, directives, 1)
		assert.Equal(t, tufv02.NewPropagationDirective("test", "https://example.com/git/repository", "refs/heads/main", "refs/heads/main", "upstream/"), directives[0])
	})
}

func TestRemovePropagationDirective(t *testing.T) {
	t.Setenv(dev.DevModeKey, "1")

	t.Run("with tuf v01 metadata", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")

		state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyRef)
		if err != nil {
			t.Fatal(err)
		}

		rootMetadata, err := state.GetRootMetadata(false)
		if err != nil {
			t.Fatal(err)
		}

		directives := rootMetadata.GetPropagationDirectives()
		assert.Empty(t, directives)

		rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

		err = r.AddPropagationDirective(testCtx, rootSigner, "test", "https://example.com/git/repository", "refs/heads/main", "refs/heads/main", "upstream/", false)
		require.Nil(t, err)

		state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef) // we haven't applied
		if err != nil {
			t.Fatal(err)
		}

		rootMetadata, err = state.GetRootMetadata(false)
		if err != nil {
			t.Fatal(err)
		}

		directives = rootMetadata.GetPropagationDirectives()
		require.Len(t, directives, 1)
		require.Equal(t, tufv01.NewPropagationDirective("test", "https://example.com/git/repository", "refs/heads/main", "refs/heads/main", "upstream/"), directives[0])

		err = r.RemovePropagationDirective(testCtx, rootSigner, "test", false)
		assert.Nil(t, err)

		state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef) // we haven't applied
		if err != nil {
			t.Fatal(err)
		}

		rootMetadata, err = state.GetRootMetadata(false)
		if err != nil {
			t.Fatal(err)
		}

		directives = rootMetadata.GetPropagationDirectives()
		require.Empty(t, directives)

		err = r.RemovePropagationDirective(testCtx, rootSigner, "test", false)
		assert.ErrorIs(t, err, tuf.ErrPropagationDirectiveNotFound)
	})

	t.Run("with tuf v02 metadata", func(t *testing.T) {
		t.Setenv(tufv02.AllowV02MetadataKey, "1")

		r := createTestRepositoryWithRoot(t, "")

		state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyRef)
		if err != nil {
			t.Fatal(err)
		}

		rootMetadata, err := state.GetRootMetadata(false)
		if err != nil {
			t.Fatal(err)
		}

		directives := rootMetadata.GetPropagationDirectives()
		require.Empty(t, directives)

		rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

		err = r.AddPropagationDirective(testCtx, rootSigner, "test", "https://example.com/git/repository", "refs/heads/main", "refs/heads/main", "upstream/", false)
		require.Nil(t, err)

		state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef) // we haven't applied
		if err != nil {
			t.Fatal(err)
		}

		rootMetadata, err = state.GetRootMetadata(false)
		if err != nil {
			t.Fatal(err)
		}

		directives = rootMetadata.GetPropagationDirectives()
		require.Len(t, directives, 1)
		require.Equal(t, tufv02.NewPropagationDirective("test", "https://example.com/git/repository", "refs/heads/main", "refs/heads/main", "upstream/"), directives[0])

		err = r.RemovePropagationDirective(testCtx, rootSigner, "test", false)
		assert.Nil(t, err)

		state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef) // we haven't applied
		if err != nil {
			t.Fatal(err)
		}

		rootMetadata, err = state.GetRootMetadata(false)
		if err != nil {
			t.Fatal(err)
		}

		directives = rootMetadata.GetPropagationDirectives()
		require.Empty(t, directives)

		err = r.RemovePropagationDirective(testCtx, rootSigner, "test", false)
		assert.ErrorIs(t, err, tuf.ErrPropagationDirectiveNotFound)
	})
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
