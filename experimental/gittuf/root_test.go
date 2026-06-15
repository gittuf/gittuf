// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	rootopts "github.com/gittuf/gittuf/experimental/gittuf/options/root"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var hookBytes = artifacts.SampleHookScript

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
		assert.Equal(t, key.KeyID, state.Metadata.RootEnvelope.Signatures[0].KeyID)

		assert.True(t, getRootPrincipalIDs(t, rootMetadata).Has(key.KeyID))

		_, err = dsse.VerifyEnvelope(testCtx, state.Metadata.RootEnvelope, []sslibdsse.Verifier{verifier}, 1)
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
		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)

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

	t.Run("with GPG signer", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		r := &Repository{r: repo}

		// Make a test GPG keyring in tempdir to use for tests
		gpg.SetupTestGPGHomeDir(t, artifacts.GPGKey1Private)

		fingerprintGPG := "157507bbe151e378ce8126c1dcfe043cdd2db96e"

		if err := repo.SetGitConfig("gpg.format", "openpgp"); err != nil {
			t.Fatal(err)
		}
		if err := repo.SetGitConfig("user.signingkey", fingerprintGPG); err != nil {
			t.Fatal(err)
		}

		signer, err := gpg.NewSignerFromKeyID(fingerprintGPG)
		require.Nil(t, err)

		location := "https://example.com/repository/location"
		err = r.InitializeRoot(testCtx, signer, false, rootopts.WithRepositoryLocation(location))
		assert.Nil(t, err)
		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)

		if err := policy.Apply(testCtx, repo, false); err != nil {
			t.Fatalf("failed to apply policy staging changes into policy, err = %s", err)
		}

		state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyRef)
		require.Nil(t, err)

		rootMetadata, err := state.GetRootMetadata(false)
		require.Nil(t, err)

		rootPrincipals, err := rootMetadata.GetRootPrincipals()
		require.Nil(t, err)
		assert.Contains(t, rootPrincipals[0].Keys(), signer.MetadataKey())

		rootEnvelope := state.Metadata.RootEnvelope
		_, err = dsse.VerifyEnvelope(t.Context(), rootEnvelope, []sslibdsse.Verifier{signer.Verifier}, 1)
		assert.Nil(t, err)
	})

	t.Run("fails when root already applied (policy ref exists)", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")

		signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
		err := r.InitializeRoot(testCtx, signer, false)
		assert.ErrorIs(t, err, ErrCannotReinitialize)
	})

	t.Run("fails when root staged but not applied (policy-staging has root)", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		r := &Repository{r: repo}
		signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

		err := r.InitializeRoot(testCtx, signer, false)
		assert.Nil(t, err)

		err = r.InitializeRoot(testCtx, signer, false)
		assert.ErrorIs(t, err, ErrCannotReinitialize)
	})

	t.Run("miscellaneous error checking", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		r := &Repository{r: repo}
		signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

		// Test signCommit
		err := repo.SetGitConfig("user.signingkey", "")
		if err != nil {
			t.Fatal(err)
		}

		err = r.InitializeRoot(testCtx, signer, true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)
	})
}

func TestSetRepositoryLocation(t *testing.T) {
	r := createTestRepositoryWithRoot(t, "")

	sv := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

	location := "https://example.com/repository/location"
	err := r.SetRepositoryLocation(testCtx, sv, location, false)
	assert.Nil(t, err)
	err = r.StagePolicy(testCtx, "", true, false)
	require.Nil(t, err)

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata(false)
	assert.Nil(t, err)
	assert.Equal(t, location, rootMetadata.GetRepositoryLocation())

	t.Run("miscellaneous error checking", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		r := &Repository{r: repo}
		rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

		// Test signCommit
		err = repo.SetGitConfig("user.signingkey", "")
		if err != nil {
			t.Fatal(err)
		}

		err := r.SetRepositoryLocation(testCtx, rootSigner, "https://example.com/new-location", true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)

		// Test non-existent policy
		err = r.SetRepositoryLocation(testCtx, rootSigner, "https://example.com/new-location", false)
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)

		// Test unauthorized signer
		r = createTestRepositoryWithRoot(t, "")
		unauthorizedSigner := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)

		err = r.SetRepositoryLocation(testCtx, unauthorizedSigner, "https://example.com/unauthorized", false)
		assert.ErrorIs(t, err, ErrUnauthorizedKey)
	})
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
	err = r.StagePolicy(testCtx, "", true, false)
	require.Nil(t, err)

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata(false)
	assert.Nil(t, err)

	assert.Equal(t, originalKeyID, state.Metadata.RootEnvelope.Signatures[0].KeyID)
	assert.Equal(t, set.NewSetFromItems(originalKeyID, newRootKey.KeyID), getRootPrincipalIDs(t, rootMetadata))

	_, err = dsse.VerifyEnvelope(testCtx, state.Metadata.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)

	t.Run("miscellaneous error checking", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		r := &Repository{r: repo}

		// Test signCommit
		err = repo.SetGitConfig("user.signingkey", "")
		if err != nil {
			t.Fatal(err)
		}

		err = r.AddRootKey(testCtx, sv, newRootKey, true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)

		// Test non-existent policy
		err = r.AddRootKey(testCtx, sv, newRootKey, false)
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)

		// Test unauthorized signer
		r = createTestRepositoryWithRoot(t, "")

		unauthorizedSigner := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)
		newRootKey = tufv01.NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targetsPubKeyBytes))

		err = r.AddRootKey(testCtx, unauthorizedSigner, newRootKey, false)
		assert.ErrorIs(t, err, ErrUnauthorizedKey)
	})
}

func TestRemoveRootKey(t *testing.T) {
	r := createTestRepositoryWithRoot(t, "")

	originalSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	rootKey := tufv01.NewKeyFromSSLibKey(originalSigner.MetadataKey())

	err := r.AddRootKey(testCtx, originalSigner, rootKey, false)
	if err != nil {
		t.Fatal(err)
	}
	err = r.StagePolicy(testCtx, "", true, false)
	require.Nil(t, err)

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata(false)
	if err != nil {
		t.Fatal(err)
	}

	rootPrincipals, err := rootMetadata.GetRootPrincipals()
	assert.Nil(t, err)
	assert.Equal(t, 1, len(rootPrincipals))

	newRootKey := tufv01.NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targetsPubKeyBytes))

	err = r.AddRootKey(testCtx, originalSigner, newRootKey, false)
	if err != nil {
		t.Fatal(err)
	}
	err = r.StagePolicy(testCtx, "", true, false)
	require.Nil(t, err)

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

	_, err = dsse.VerifyEnvelope(testCtx, state.Metadata.RootEnvelope, []sslibdsse.Verifier{originalSigner}, 1)
	assert.Nil(t, err)

	newSigner := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)

	// We can use the newly added root key to revoke the old one
	err = r.RemoveRootKey(testCtx, newSigner, rootKey.KeyID, false)
	assert.Nil(t, err)
	err = r.StagePolicy(testCtx, "", true, false)
	require.Nil(t, err)

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

	_, err = dsse.VerifyEnvelope(testCtx, state.Metadata.RootEnvelope, []sslibdsse.Verifier{newSigner}, 1)
	assert.Nil(t, err)

	t.Run("miscellaneous error checking", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		r := &Repository{r: repo}

		// Test signCommit
		err = repo.SetGitConfig("user.signingkey", "")
		if err != nil {
			t.Fatal(err)
		}

		err = r.RemoveRootKey(testCtx, newSigner, rootKey.KeyID, true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)

		// Test non-existent policy
		err = r.RemoveRootKey(testCtx, newSigner, rootKey.KeyID, false)
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)

		// Test unauthorized signer
		r = createTestRepositoryWithRoot(t, "")

		unauthorizedSigner := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)
		newRootKey = tufv01.NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targetsPubKeyBytes))

		err = r.RemoveRootKey(testCtx, unauthorizedSigner, rootKey.KeyID, false)
		assert.ErrorIs(t, err, ErrUnauthorizedKey)

		// Test error with removing key
		err = r.RemoveRootKey(testCtx, originalSigner, newRootKey.KeyID, false)
		assert.ErrorIs(t, err, tuf.ErrCannotMeetThreshold)
	})
}

func TestAddTopLevelTargetsKey(t *testing.T) {
	r := createTestRepositoryWithRoot(t, "")

	sv := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	key := tufv01.NewKeyFromSSLibKey(sv.MetadataKey())

	err := r.AddTopLevelTargetsKey(testCtx, sv, key, false)
	assert.Nil(t, err)
	err = r.StagePolicy(testCtx, "", true, false)
	require.Nil(t, err)

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata(false)
	assert.Nil(t, err)
	assert.Equal(t, key.KeyID, state.Metadata.RootEnvelope.Signatures[0].KeyID)
	assert.True(t, getRootPrincipalIDs(t, rootMetadata).Has(key.KeyID))
	assert.True(t, getPrimaryRuleFilePrincipalIDs(t, rootMetadata).Has(key.KeyID))

	_, err = dsse.VerifyEnvelope(testCtx, state.Metadata.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)

	t.Run("miscellaneous error checking", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		nr := &Repository{r: repo}

		// Test signCommit
		err = repo.SetGitConfig("user.signingkey", "")
		if err != nil {
			t.Fatal(err)
		}

		err = nr.AddTopLevelTargetsKey(testCtx, sv, key, true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)

		// Test non-existent policy
		err = nr.AddTopLevelTargetsKey(testCtx, sv, key, false)
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)

		// Test unauthorized signer
		unauthorizedSigner := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)

		err = r.AddTopLevelTargetsKey(testCtx, unauthorizedSigner, key, false)
		assert.ErrorIs(t, err, ErrUnauthorizedKey)
	})
}

func TestRemoveTopLevelTargetsKey(t *testing.T) {
	r := createTestRepositoryWithRoot(t, "")

	sv := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	rootKey := tufv01.NewKeyFromSSLibKey(sv.MetadataKey())

	err := r.AddTopLevelTargetsKey(testCtx, sv, rootKey, false)
	if err != nil {
		t.Fatal(err)
	}
	err = r.StagePolicy(testCtx, "", true, false)
	require.Nil(t, err)

	targetsKey := tufv01.NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targetsPubKeyBytes))

	err = r.AddTopLevelTargetsKey(testCtx, sv, targetsKey, false)
	if err != nil {
		t.Fatal(err)
	}
	err = r.StagePolicy(testCtx, "", true, false)
	require.Nil(t, err)

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

	_, err = dsse.VerifyEnvelope(testCtx, state.Metadata.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)

	err = r.RemoveTopLevelTargetsKey(testCtx, sv, rootKey.KeyID, false)
	assert.Nil(t, err)
	err = r.StagePolicy(testCtx, "", true, false)
	require.Nil(t, err)

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
	_, err = dsse.VerifyEnvelope(testCtx, state.Metadata.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)

	t.Run("miscellaneous error checking", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		nr := &Repository{r: repo}

		// Test signCommit
		err = repo.SetGitConfig("user.signingkey", "")
		if err != nil {
			t.Fatal(err)
		}

		err = nr.RemoveTopLevelTargetsKey(testCtx, sv, rootKey.KeyID, true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)

		// Test non-existent policy
		err = nr.RemoveTopLevelTargetsKey(testCtx, sv, rootKey.KeyID, false)
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)

		// Test unauthorized signer
		unauthorizedSigner := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)

		err = r.RemoveTopLevelTargetsKey(testCtx, unauthorizedSigner, rootKey.KeyID, false)
		assert.ErrorIs(t, err, ErrUnauthorizedKey)

		// Test error with removing key
		err = r.RemoveTopLevelTargetsKey(testCtx, sv, rootKey.KeyID, false)
		assert.ErrorIs(t, err, tuf.ErrCannotMeetThreshold)
	})
}

func TestAddGitHubApp(t *testing.T) {
	r := createTestRepositoryWithRoot(t, "")

	sv := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	key := tufv01.NewKeyFromSSLibKey(sv.MetadataKey())

	err := r.AddGitHubApp(testCtx, sv, "github-app", key, false)
	assert.Nil(t, err)
	err = r.StagePolicy(testCtx, "", true, false)
	require.Nil(t, err)

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata(false)
	assert.Nil(t, err)

	appPrincipals, err := rootMetadata.GetGitHubAppPrincipals("github-app")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, key, appPrincipals[0])

	_, err = dsse.VerifyEnvelope(testCtx, state.Metadata.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)

	t.Run("miscellaneous error checking", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		nr := &Repository{r: repo}

		// Test signCommit
		err = repo.SetGitConfig("user.signingkey", "")
		if err != nil {
			t.Fatal(err)
		}

		err := nr.AddGitHubApp(testCtx, sv, "github-app", key, true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)

		// Test non-existent policy
		err = nr.AddGitHubApp(testCtx, sv, "github-app", key, false)
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)

		// Test unauthorized signer
		r := createTestRepositoryWithRoot(t, "")
		unauthorizedSigner := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)
		key := tufv01.NewKeyFromSSLibKey(unauthorizedSigner.MetadataKey())

		err = r.AddGitHubApp(testCtx, unauthorizedSigner, "github-app", key, false)
		assert.ErrorIs(t, err, ErrUnauthorizedKey)
	})

	t.Run("default app name", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")
		sv := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
		key := tufv01.NewKeyFromSSLibKey(sv.MetadataKey())

		err := r.AddGitHubApp(testCtx, sv, "", key, false)
		require.Nil(t, err)

		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)

		state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
		require.Nil(t, err)

		rootMetadata, err := state.GetRootMetadata(false)
		require.Nil(t, err)

		appPrincipals, err := rootMetadata.GetGitHubAppPrincipals(tuf.GitHubAppRoleName)
		require.Nil(t, err)
		require.Len(t, appPrincipals, 1)
		assert.Equal(t, key, appPrincipals[0])
	})
}

func TestRemoveGitHubApp(t *testing.T) {
	r := createTestRepositoryWithRoot(t, "")

	sv := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	key := tufv01.NewKeyFromSSLibKey(sv.MetadataKey())

	err := r.AddGitHubApp(testCtx, sv, "github-app", key, false)
	if err != nil {
		t.Fatal(err)
	}
	err = r.StagePolicy(testCtx, "", true, false)
	require.Nil(t, err)

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata(false)
	if err != nil {
		t.Fatal(err)
	}

	appPrincipals, err := rootMetadata.GetGitHubAppPrincipals("github-app")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, key, appPrincipals[0])

	_, err = dsse.VerifyEnvelope(testCtx, state.Metadata.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)

	err = r.RemoveGitHubApp(testCtx, sv, "github-app", false)
	assert.Nil(t, err)
	err = r.StagePolicy(testCtx, "", true, false)
	require.Nil(t, err)

	state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err = state.GetRootMetadata(false)
	if err != nil {
		t.Fatal(err)
	}

	appPrincipals, err = rootMetadata.GetGitHubAppPrincipals("github-app")
	// We see an error (correctly that the app is trusted but no key is present)
	assert.ErrorIs(t, err, tuf.ErrGitHubAppInformationNotFoundInRoot)
	assert.Empty(t, appPrincipals)

	_, err = dsse.VerifyEnvelope(testCtx, state.Metadata.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)

	t.Run("miscellaneous error checking", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		nr := &Repository{r: repo}

		// Test signCommit
		err = repo.SetGitConfig("user.signingkey", "")
		if err != nil {
			t.Fatal(err)
		}

		err = nr.RemoveGitHubApp(testCtx, sv, "github-app", true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)

		// Test non-existent policy
		err = nr.RemoveGitHubApp(testCtx, sv, "github-app", false)
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)

		// Test unauthorized signer
		sv := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)

		err = r.RemoveGitHubApp(testCtx, sv, "github-app", true)
		assert.ErrorIs(t, err, ErrUnauthorizedKey)
	})

	t.Run("default app name", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")
		sv := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
		key := tufv01.NewKeyFromSSLibKey(sv.MetadataKey())

		err := r.AddGitHubApp(testCtx, sv, "", key, false)
		require.Nil(t, err)

		err = r.RemoveGitHubApp(testCtx, sv, "", false)
		require.Nil(t, err)

		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)

		state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
		require.Nil(t, err)

		rootMetadata, err := state.GetRootMetadata(false)
		require.Nil(t, err)

		_, err = rootMetadata.GetGitHubAppPrincipals(tuf.GitHubAppRoleName)
		assert.ErrorIs(t, err, tuf.ErrGitHubAppInformationNotFoundInRoot)
	})
}

func TestTrustGitHubApp(t *testing.T) {
	r := createTestRepositoryWithRoot(t, "")

	sv := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	key := tufv01.NewKeyFromSSLibKey(sv.MetadataKey())

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := state.GetRootMetadata(false)
	assert.Nil(t, err)

	assert.False(t, rootMetadata.IsGitHubAppApprovalTrusted("github-app"))

	err = r.AddGitHubApp(testCtx, sv, "github-app", key, false)
	assert.Nil(t, err)

	err = r.TrustGitHubApp(testCtx, sv, "github-app", false)
	assert.Nil(t, err)

	err = r.StagePolicy(testCtx, "", true, false)
	require.Nil(t, err)

	state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err = state.GetRootMetadata(false)
	assert.Nil(t, err)

	assert.True(t, rootMetadata.IsGitHubAppApprovalTrusted("github-app"))
	_, err = dsse.VerifyEnvelope(testCtx, state.Metadata.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)

	// Test if we can trust again if already trusted
	err = r.TrustGitHubApp(testCtx, sv, "github-app", false)
	assert.Nil(t, err)

	t.Run("miscellaneous error checking", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		nr := &Repository{r: repo}

		// Test signCommit
		err = repo.SetGitConfig("user.signingkey", "")
		if err != nil {
			t.Fatal(err)
		}

		err = nr.TrustGitHubApp(testCtx, sv, "github-app", true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)

		// Test non-existent policy
		err = nr.TrustGitHubApp(testCtx, sv, "github-app", false)
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)

		// Test unauthorized signer
		sv := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)

		err = r.TrustGitHubApp(testCtx, sv, "github-app", true)
		assert.ErrorIs(t, err, ErrUnauthorizedKey)
	})

	t.Run("default app name with trusted no-op", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")
		sv := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
		key := tufv01.NewKeyFromSSLibKey(sv.MetadataKey())

		err := r.AddGitHubApp(testCtx, sv, "", key, false)
		require.Nil(t, err)

		err = r.TrustGitHubApp(testCtx, sv, "", false)
		require.Nil(t, err)

		err = r.TrustGitHubApp(testCtx, sv, "", false)
		require.Nil(t, err)

		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)

		state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
		require.Nil(t, err)

		rootMetadata, err := state.GetRootMetadata(false)
		require.Nil(t, err)
		assert.True(t, rootMetadata.IsGitHubAppApprovalTrusted(tuf.GitHubAppRoleName))
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

	assert.False(t, rootMetadata.IsGitHubAppApprovalTrusted("github-app"))

	err = r.AddGitHubApp(testCtx, sv, "github-app", key, false)
	assert.Nil(t, err)

	err = r.TrustGitHubApp(testCtx, sv, "github-app", false)
	assert.Nil(t, err)

	err = r.StagePolicy(testCtx, "", true, false)
	require.Nil(t, err)

	state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err = state.GetRootMetadata(false)
	assert.Nil(t, err)

	assert.True(t, rootMetadata.IsGitHubAppApprovalTrusted("github-app"))
	_, err = dsse.VerifyEnvelope(testCtx, state.Metadata.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)

	err = r.UntrustGitHubApp(testCtx, sv, "github-app", false)
	assert.Nil(t, err)
	err = r.StagePolicy(testCtx, "", true, false)
	require.Nil(t, err)

	state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err = state.GetRootMetadata(false)
	assert.Nil(t, err)

	assert.False(t, rootMetadata.IsGitHubAppApprovalTrusted("github-app"))
	_, err = dsse.VerifyEnvelope(testCtx, state.Metadata.RootEnvelope, []sslibdsse.Verifier{sv}, 1)
	assert.Nil(t, err)

	t.Run("miscellaneous error checking", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		nr := &Repository{r: repo}

		// Test signCommit
		err = repo.SetGitConfig("user.signingkey", "")
		if err != nil {
			t.Fatal(err)
		}

		err = nr.UntrustGitHubApp(testCtx, sv, "github-app", true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)

		// Test non-existent policy
		err = nr.UntrustGitHubApp(testCtx, sv, "github-app", false)
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)

		// Test unauthorized signer
		sv := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)

		err = r.UntrustGitHubApp(testCtx, sv, "github-app", true)
		assert.ErrorIs(t, err, ErrUnauthorizedKey)
	})

	t.Run("default app name with untrusted no-op", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")
		sv := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
		key := tufv01.NewKeyFromSSLibKey(sv.MetadataKey())

		err := r.AddGitHubApp(testCtx, sv, "", key, false)
		require.Nil(t, err)

		err = r.TrustGitHubApp(testCtx, sv, "", false)
		require.Nil(t, err)

		err = r.UntrustGitHubApp(testCtx, sv, "", false)
		require.Nil(t, err)

		err = r.UntrustGitHubApp(testCtx, sv, "", false)
		require.Nil(t, err)

		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)

		state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
		require.Nil(t, err)

		rootMetadata, err := state.GetRootMetadata(false)
		require.Nil(t, err)
		assert.False(t, rootMetadata.IsGitHubAppApprovalTrusted(tuf.GitHubAppRoleName))
	})
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

	err = r.StagePolicy(testCtx, "", true, false)
	require.Nil(t, err)

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

	t.Run("miscellaneous error checking", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		nr := &Repository{r: repo}

		// Test signCommit
		err = repo.SetGitConfig("user.signingkey", "")
		if err != nil {
			t.Fatal(err)
		}

		err = nr.UpdateRootThreshold(testCtx, nil, 1, true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)

		// Test non-existent policy
		err = nr.UpdateRootThreshold(testCtx, signer, 1, false)
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)

		// Test unauthorized signer
		r = createTestRepositoryWithRoot(t, "")

		sv := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)

		err = r.UpdateRootThreshold(testCtx, sv, 1, true)
		assert.ErrorIs(t, err, ErrUnauthorizedKey)
	})
}

func TestUpdateTopLevelTargetsThreshold(t *testing.T) {
	r := createTestRepositoryWithRoot(t, "")

	sv := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	key := tufv01.NewKeyFromSSLibKey(sv.MetadataKey())

	if err := r.AddTopLevelTargetsKey(testCtx, sv, key, false); err != nil {
		t.Fatal(err)
	}

	err := r.StagePolicy(testCtx, "", true, false)
	require.Nil(t, err)

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

	err = r.StagePolicy(testCtx, "", true, false)
	require.Nil(t, err)

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

	t.Run("miscellaneous error checking", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		nr := &Repository{r: repo}

		// Test signCommit
		err = repo.SetGitConfig("user.signingkey", "")
		if err != nil {
			t.Fatal(err)
		}

		err = nr.UpdateTopLevelTargetsThreshold(testCtx, nil, 1, true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)

		// Test non-existent policy
		err = nr.UpdateTopLevelTargetsThreshold(testCtx, sv, 1, false)
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)

		// Test unauthorized signer
		r = createTestRepositoryWithRoot(t, "")

		sv = setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)

		err = r.UpdateTopLevelTargetsThreshold(testCtx, sv, 1, true)
		assert.ErrorIs(t, err, ErrUnauthorizedKey)
	})
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

	err = r.StagePolicy(testCtx, "", true, false)
	require.Nil(t, err)

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 2, len(state.Metadata.RootEnvelope.Signatures))

	t.Run("miscellaneous error checking", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		nr := &Repository{r: repo}

		// Test signCommit
		err = repo.SetGitConfig("user.signingkey", "")
		if err != nil {
			t.Fatal(err)
		}

		err = nr.SignRoot(testCtx, nil, true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)

		// Test non-existent policy
		err = nr.SignRoot(testCtx, rootSigner, false)
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)
	})
}

func TestAddGlobalRuleThreshold(t *testing.T) {
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

	err = r.StagePolicy(testCtx, "", true, false)
	require.Nil(t, err)

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

	t.Run("miscellaneous error checking", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		nr := &Repository{r: repo}

		// Test signCommit
		err = repo.SetGitConfig("user.signingkey", "")
		if err != nil {
			t.Fatal(err)
		}

		err = nr.AddGlobalRuleThreshold(testCtx, nil, "", nil, 1, true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)

		// Test non-existent policy
		err = nr.AddGlobalRuleThreshold(testCtx, rootSigner, "", nil, 1, false)
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)

		// Test unauthorized signer
		r = createTestRepositoryWithRoot(t, "")

		sv := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)

		err = r.AddGlobalRuleThreshold(testCtx, sv, "", nil, 1, false)
		assert.ErrorIs(t, err, ErrUnauthorizedKey)
	})
}

func TestAddGlobalRuleBlockForcePushes(t *testing.T) {
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

	err = r.StagePolicy(testCtx, "", true, false)
	require.Nil(t, err)

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

	t.Run("miscellaneous error checking", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		nr := &Repository{r: repo}

		// Test signCommit
		err = repo.SetGitConfig("user.signingkey", "")
		if err != nil {
			t.Fatal(err)
		}

		err = nr.AddGlobalRuleBlockForcePushes(testCtx, nil, "", nil, true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)

		// Test non-existent policy
		err = nr.AddGlobalRuleBlockForcePushes(testCtx, rootSigner, "", nil, false)
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)

		// Test unauthorized signer
		r = createTestRepositoryWithRoot(t, "")

		sv := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)

		err = r.AddGlobalRuleBlockForcePushes(testCtx, sv, "", nil, false)
		assert.ErrorIs(t, err, ErrUnauthorizedKey)
	})
}
func TestRemoveGlobalRule(t *testing.T) {
	t.Run("remove threshold global rule", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")

		rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

		err := r.AddGlobalRuleThreshold(testCtx, rootSigner, "require-approval-for-main", []string{"git:refs/heads/main"}, 1, false)
		assert.Nil(t, err)

		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)

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

		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)

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

		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)

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

		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)

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

	t.Run("miscellaneous error checking", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		nr := &Repository{r: repo}

		// Test signCommit
		err := repo.SetGitConfig("user.signingkey", "")
		if err != nil {
			t.Fatal(err)
		}

		err = nr.RemoveGlobalRule(testCtx, nil, "", true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)

		// Test non-existent policy

		sv := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)

		err = nr.RemoveGlobalRule(testCtx, sv, "", false)
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)

		// Test unauthorized signer
		r := createTestRepositoryWithRoot(t, "")

		err = r.RemoveGlobalRule(testCtx, sv, "", false)
		assert.ErrorIs(t, err, ErrUnauthorizedKey)
	})
}

func TestUpdateGlobalRule(t *testing.T) {
	t.Run("update threshold in threshold global rule", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")

		rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

		err := r.AddGlobalRuleThreshold(testCtx, rootSigner, "require-approval-for-main", []string{"git:refs/heads/main"}, 1, false)
		assert.Nil(t, err)

		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)

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

		err = r.UpdateGlobalRuleThreshold(testCtx, rootSigner, "require-approval-for-main", []string{"git:refs/heads/main"}, 2, false)
		assert.Nil(t, err)

		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)

		state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
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
		assert.Equal(t, 2, globalRules[0].(tuf.GlobalRuleThreshold).GetThreshold())
	})

	t.Run("update pattern in threshold global rule", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")

		rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

		err := r.AddGlobalRuleThreshold(testCtx, rootSigner, "require-approval-for-main", []string{"git:refs/heads/main"}, 1, false)
		assert.Nil(t, err)

		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)

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

		err = r.UpdateGlobalRuleThreshold(testCtx, rootSigner, "require-approval-for-main", []string{"git:refs/heads/*"}, 1, false)
		assert.Nil(t, err)

		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)

		state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
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
		assert.Equal(t, []string{"git:refs/heads/*"}, globalRules[0].(tuf.GlobalRuleThreshold).GetProtectedNamespaces())
		assert.Equal(t, 1, globalRules[0].(tuf.GlobalRuleThreshold).GetThreshold())
	})

	t.Run("update force push global rule", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")

		rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

		err := r.AddGlobalRuleBlockForcePushes(testCtx, rootSigner, "block-force-pushes-for-main", []string{"git:refs/heads/main"}, false)
		assert.Nil(t, err)

		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)

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

		err = r.UpdateGlobalRuleBlockForcePushes(testCtx, rootSigner, "block-force-pushes-for-main", []string{"git:refs/heads/*"}, false)
		assert.Nil(t, err)

		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)

		state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
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
		assert.Equal(t, []string{"git:refs/heads/*"}, globalRules[0].(tuf.GlobalRuleBlockForcePushes).GetProtectedNamespaces())
	})

	t.Run("update global rule when none exist", func(t *testing.T) {
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

		err = r.UpdateGlobalRuleThreshold(testCtx, rootSigner, "require-approval-for-main", []string{"git:refs/heads/main"}, 2, false)
		assert.ErrorIs(t, err, tuf.ErrGlobalRuleNotFound)
		err = r.UpdateGlobalRuleBlockForcePushes(testCtx, rootSigner, "block-force-pushes-for-main", []string{"git:refs/heads/main"}, false)
		assert.ErrorIs(t, err, tuf.ErrGlobalRuleNotFound)
	})

	t.Run("update threshold global rule that does not exist", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")

		rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

		err := r.AddGlobalRuleThreshold(testCtx, rootSigner, "require-approval-for-main", []string{"git:refs/heads/main"}, 1, false)
		assert.Nil(t, err)

		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)

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

		err = r.UpdateGlobalRuleThreshold(testCtx, rootSigner, "require-2-approvals-for-main", []string{"git:refs/heads/main"}, 2, false)
		assert.ErrorIs(t, err, tuf.ErrGlobalRuleNotFound)

		state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
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
	})

	t.Run("update force push global rule that does not exist", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")

		rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

		err := r.AddGlobalRuleBlockForcePushes(testCtx, rootSigner, "block-force-pushes-for-main", []string{"git:refs/heads/main"}, false)
		assert.Nil(t, err)

		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)

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

		err = r.UpdateGlobalRuleBlockForcePushes(testCtx, rootSigner, "block-force-pushes-for-all", []string{"git:refs/heads/*"}, false)
		assert.ErrorIs(t, err, tuf.ErrGlobalRuleNotFound)

		state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
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
	})

	t.Run("update force push global rule to threshold global rule", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")

		rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

		err := r.AddGlobalRuleBlockForcePushes(testCtx, rootSigner, "block-force-pushes-for-main", []string{"git:refs/heads/main"}, false)
		assert.Nil(t, err)

		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)

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

		err = r.UpdateGlobalRuleThreshold(testCtx, rootSigner, "block-force-pushes-for-main", []string{"git:refs/heads/main"}, 1, false)
		assert.ErrorIs(t, err, tuf.ErrCannotUpdateGlobalRuleType)

		state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
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
	})

	t.Run("update threshold global rule to force push global rule", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")

		rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

		err := r.AddGlobalRuleThreshold(testCtx, rootSigner, "require-approval-for-main", []string{"git:refs/heads/main"}, 1, false)
		assert.Nil(t, err)

		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)

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

		err = r.UpdateGlobalRuleBlockForcePushes(testCtx, rootSigner, "require-approval-for-main", []string{"git:refs/heads/main"}, false)
		assert.ErrorIs(t, err, tuf.ErrCannotUpdateGlobalRuleType)

		state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
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
		assert.Equal(t, []string{"git:refs/heads/main"}, globalRules[0].(tuf.GlobalRuleBlockForcePushes).GetProtectedNamespaces())
	})

	t.Run("miscellaneous error checking", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		nr := &Repository{r: repo}

		// Test signCommit
		err := repo.SetGitConfig("user.signingkey", "")
		if err != nil {
			t.Fatal(err)
		}

		err = nr.UpdateGlobalRuleThreshold(testCtx, nil, "", nil, 1, true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)

		err = nr.UpdateGlobalRuleBlockForcePushes(testCtx, nil, "", nil, true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)

		// Test non-existent policy
		sv := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)

		err = nr.UpdateGlobalRuleThreshold(testCtx, sv, "", nil, 1, false)
		assert.ErrorIs(t, err, rsl.ErrRSLEntryNotFound)

		err = nr.UpdateGlobalRuleBlockForcePushes(testCtx, sv, "", nil, false)
		assert.ErrorIs(t, err, rsl.ErrRSLEntryNotFound)

		// Test unauthorized signer
		r := createTestRepositoryWithRoot(t, "")

		err = r.UpdateGlobalRuleThreshold(testCtx, sv, "", nil, 1, false)
		assert.ErrorIs(t, err, ErrUnauthorizedKey)

		err = r.UpdateGlobalRuleBlockForcePushes(testCtx, sv, "", nil, false)
		assert.ErrorIs(t, err, ErrUnauthorizedKey)
	})
}

func TestListGlobalRules(t *testing.T) {
	t.Run("list global rules after add and remove", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")

		rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

		err := r.AddGlobalRuleThreshold(testCtx, rootSigner, "require-approval-for-main", []string{"git:refs/heads/main"}, 1, false, trustpolicyopts.WithRSLEntry())
		assert.Nil(t, err)

		globalRules, err := r.ListGlobalRules(testCtx, policy.PolicyStagingRef)
		assert.Nil(t, err)
		assert.Len(t, globalRules, 1)

		err = r.RemoveGlobalRule(testCtx, rootSigner, "require-approval-for-main", false, trustpolicyopts.WithRSLEntry())
		assert.Nil(t, err)

		globalRules, err = r.ListGlobalRules(testCtx, policy.PolicyStagingRef)
		assert.Nil(t, err)
		assert.Empty(t, globalRules)
	})
}

func TestAddPropagationDirective(t *testing.T) {
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

		err = r.AddPropagationDirective(testCtx, rootSigner, "test", "https://example.com/git/repository", "refs/heads/main", "", "refs/heads/main", "upstream/", false)
		assert.Nil(t, err)

		err = r.StagePolicy(testCtx, "", true, false)
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
		assert.Len(t, directives, 1)
		assert.Equal(t, tufv01.NewPropagationDirective("test", "https://example.com/git/repository", "refs/heads/main", "", "refs/heads/main", "upstream/"), directives[0])
	})

	t.Run("with tuf v02 metadata", func(t *testing.T) {
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

		err = r.AddPropagationDirective(testCtx, rootSigner, "test", "https://example.com/git/repository", "refs/heads/main", "upstreamPath/", "refs/heads/main", "upstream/", false)
		assert.Nil(t, err)

		err = r.StagePolicy(testCtx, "", true, false)
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
		assert.Len(t, directives, 1)
		assert.Equal(t, tufv02.NewPropagationDirective("test", "https://example.com/git/repository", "refs/heads/main", "upstreamPath/", "refs/heads/main", "upstream/"), directives[0])
	})

	t.Run("miscellaneous error checking", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		nr := &Repository{r: repo}

		// Test signCommit
		err := repo.SetGitConfig("user.signingkey", "")
		if err != nil {
			t.Fatal(err)
		}

		err = nr.AddPropagationDirective(testCtx, nil, "", "", "", "", "", "", true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)

		// Test non-existent policy
		sv := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)

		err = nr.AddPropagationDirective(testCtx, sv, "", "", "", "", "", "", false)
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)

		// Test unauthorized signer
		r := createTestRepositoryWithRoot(t, "")

		err = r.AddPropagationDirective(testCtx, sv, "", "", "", "", "", "", false)
		assert.ErrorIs(t, err, ErrUnauthorizedKey)
	})
}

func TestUpdatePropagationDirective(t *testing.T) {
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

		err = r.AddPropagationDirective(testCtx, rootSigner, "test", "https://example.com/git/repository", "refs/heads/main", "upstreamPath/", "refs/heads/main", "upstream/", false)
		assert.Nil(t, err)

		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)

		err = r.UpdatePropagationDirective(testCtx, rootSigner, "test", "https://newexample.com/git/repository", "refs/newheads/main", "upstreamPath/", "refs/newheads/main", "newupstream/", false)
		assert.Nil(t, err)

		err = r.StagePolicy(testCtx, "", true, false)
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
		assert.Len(t, directives, 1)
		assert.Equal(t, tufv01.NewPropagationDirective("test", "https://newexample.com/git/repository", "refs/newheads/main", "upstreamPath/", "refs/newheads/main", "newupstream/"), directives[0])
	})

	t.Run("with tuf v02 metadata", func(t *testing.T) {
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

		err = r.AddPropagationDirective(testCtx, rootSigner, "test", "https://example.com/git/repository", "refs/heads/main", "upstreamPath/", "refs/heads/main", "upstream/", false)
		assert.Nil(t, err)

		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)

		err = r.UpdatePropagationDirective(testCtx, rootSigner, "test", "https://newexample.com/git/repository", "refs/newheads/main", "upstreamPath/", "refs/newheads/main", "newupstream/", false)
		assert.Nil(t, err)

		err = r.StagePolicy(testCtx, "", true, false)
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
		assert.Len(t, directives, 1)
		assert.Equal(t, tufv02.NewPropagationDirective("test", "https://newexample.com/git/repository", "refs/newheads/main", "upstreamPath/", "refs/newheads/main", "newupstream/"), directives[0])
	})

	t.Run("miscellaneous error checking", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		nr := &Repository{r: repo}

		// Test signCommit
		err := repo.SetGitConfig("user.signingkey", "")
		if err != nil {
			t.Fatal(err)
		}

		err = nr.UpdatePropagationDirective(testCtx, nil, "", "", "", "", "", "", true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)

		// Test non-existent policy
		sv := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)

		err = nr.UpdatePropagationDirective(testCtx, sv, "", "", "", "", "", "", false)
		assert.ErrorIs(t, err, rsl.ErrRSLEntryNotFound)

		// Test unauthorized signer
		r := createTestRepositoryWithRoot(t, "")

		err = r.UpdatePropagationDirective(testCtx, sv, "", "", "", "", "", "", false)
		assert.ErrorIs(t, err, ErrUnauthorizedKey)
	})
}

func TestRemovePropagationDirective(t *testing.T) {
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

		err = r.AddPropagationDirective(testCtx, rootSigner, "test", "https://example.com/git/repository", "refs/heads/main", "upstreamPath/", "refs/heads/main", "upstream/", false)
		require.Nil(t, err)

		err = r.StagePolicy(testCtx, "", true, false)
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
		require.Equal(t, tufv01.NewPropagationDirective("test", "https://example.com/git/repository", "refs/heads/main", "upstreamPath/", "refs/heads/main", "upstream/"), directives[0])

		err = r.RemovePropagationDirective(testCtx, rootSigner, "test", false)
		assert.Nil(t, err)

		err = r.StagePolicy(testCtx, "", true, false)
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
		require.Empty(t, directives)

		err = r.RemovePropagationDirective(testCtx, rootSigner, "test", false)
		assert.ErrorIs(t, err, tuf.ErrPropagationDirectiveNotFound)
	})

	t.Run("with tuf v02 metadata", func(t *testing.T) {
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

		err = r.AddPropagationDirective(testCtx, rootSigner, "test", "https://example.com/git/repository", "refs/heads/main", "", "refs/heads/main", "upstream/", false)
		require.Nil(t, err)

		err = r.StagePolicy(testCtx, "", true, false)
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
		require.Equal(t, tufv02.NewPropagationDirective("test", "https://example.com/git/repository", "refs/heads/main", "", "refs/heads/main", "upstream/"), directives[0])

		err = r.RemovePropagationDirective(testCtx, rootSigner, "test", false)
		assert.Nil(t, err)

		err = r.StagePolicy(testCtx, "", true, false)
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
		require.Empty(t, directives)

		err = r.RemovePropagationDirective(testCtx, rootSigner, "test", false)
		assert.ErrorIs(t, err, tuf.ErrPropagationDirectiveNotFound)
	})

	t.Run("miscellaneous error checking", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		nr := &Repository{r: repo}

		// Test signCommit
		err := repo.SetGitConfig("user.signingkey", "")
		if err != nil {
			t.Fatal(err)
		}

		err = nr.RemovePropagationDirective(testCtx, nil, "", true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)

		// Test non-existent policy
		sv := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)

		err = nr.RemovePropagationDirective(testCtx, sv, "", false)
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)

		// Test unauthorized signer
		r := createTestRepositoryWithRoot(t, "")

		err = r.RemovePropagationDirective(testCtx, sv, "", false)
		assert.ErrorIs(t, err, ErrUnauthorizedKey)
	})
}

func TestAddHook(t *testing.T) {
	t.Setenv(dev.DevModeKey, "1")

	rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	rootPubKey := tufv01.NewKeyFromSSLibKey(rootSigner.MetadataKey())

	t.Run("valid pre-commit hook", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")

		hookStage := tuf.HookStagePreCommit
		hookName := "test-hook"
		environment := tuf.HookEnvironmentLua
		timeout := 100
		principals := []string{rootPubKey.KeyID}

		hookHash, err := r.r.WriteBlob(hookBytes)
		if err != nil {
			t.Fatal(err)
		}

		sha256Hash := sha256.New()
		sha256Hash.Write(hookBytes)
		sha256HashSum := hex.EncodeToString(sha256Hash.Sum(nil))

		state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		rootMetadata, err := state.GetRootMetadata(false)
		assert.Nil(t, err)
		_, err = rootMetadata.GetHooks(tuf.HookStagePreCommit)
		assert.ErrorIs(t, err, tuf.ErrNoHooksDefined)

		err = r.AddHook(testCtx, rootSigner, []tuf.HookStage{hookStage}, hookName, hookBytes, environment, principals, timeout, true, trustpolicyopts.WithRSLEntry())
		assert.Nil(t, err)

		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)

		state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		rootMetadata, err = state.GetRootMetadata(false)
		assert.Nil(t, err)
		hooks, err := rootMetadata.GetHooks(tuf.HookStagePreCommit)
		assert.Nil(t, err)

		preCommitHook := tufv01.Hook{
			Name:         hookName,
			PrincipalIDs: set.NewSetFromItems(rootPubKey.KeyID),
			Hashes:       map[string]string{gitinterface.GitBlobHashName: hookHash.String(), gitinterface.SHA256HashName: sha256HashSum},
			Environment:  tuf.HookEnvironmentLua,
			Timeout:      100,
		}
		preCommitHooks := []tuf.Hook{&preCommitHook}
		assert.Equal(t, preCommitHooks, hooks)
	})

	t.Run("valid pre-push hook", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")

		hookStage := tuf.HookStagePrePush
		hookName := "test-hook"
		environment := tuf.HookEnvironmentLua
		timeout := 100
		principals := []string{rootPubKey.KeyID}

		hookHash, err := r.r.WriteBlob(hookBytes)
		if err != nil {
			t.Fatal(err)
		}

		sha256Hash := sha256.New()
		sha256Hash.Write(hookBytes)
		sha256HashSum := hex.EncodeToString(sha256Hash.Sum(nil))

		state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		rootMetadata, err := state.GetRootMetadata(false)
		assert.Nil(t, err)
		_, err = rootMetadata.GetHooks(tuf.HookStagePrePush)
		assert.ErrorIs(t, err, tuf.ErrNoHooksDefined)

		err = r.AddHook(testCtx, rootSigner, []tuf.HookStage{hookStage}, hookName, hookBytes, environment, principals, timeout, true, trustpolicyopts.WithRSLEntry())
		assert.Nil(t, err)

		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)

		state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		rootMetadata, err = state.GetRootMetadata(false)
		assert.Nil(t, err)
		hooks, err := rootMetadata.GetHooks(tuf.HookStagePrePush)
		assert.Nil(t, err)
		preCommitHook := tufv01.Hook{
			Name:         hookName,
			PrincipalIDs: set.NewSetFromItems(rootPubKey.KeyID),
			Hashes:       map[string]string{gitinterface.GitBlobHashName: hookHash.String(), gitinterface.SHA256HashName: sha256HashSum},
			Environment:  tuf.HookEnvironmentLua,
			Timeout:      100,
		}
		preCommitHooks := []tuf.Hook{&preCommitHook}
		assert.Equal(t, preCommitHooks, hooks)
	})

	t.Run("invalid hook name", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")
		err := r.AddHook(testCtx, rootSigner, []tuf.HookStage{tuf.HookStagePreCommit}, "", hookBytes, tuf.HookEnvironmentLua, []string{rootPubKey.KeyID}, 100, false)
		assert.ErrorIs(t, err, ErrNoHookName)
	})

	t.Run("invalid hook timeout", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")
		err := r.AddHook(testCtx, rootSigner, []tuf.HookStage{tuf.HookStagePreCommit}, "timeout-hook", hookBytes, tuf.HookEnvironmentLua, []string{rootPubKey.KeyID}, 0, false)
		assert.ErrorIs(t, err, ErrInvalidHookTimeout)
	})
}

func TestRemoveHook(t *testing.T) {
	t.Setenv(dev.DevModeKey, "1")

	rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	rootPubKey := tufv01.NewKeyFromSSLibKey(rootSigner.MetadataKey())

	t.Run("valid pre-commit hook", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")

		hookStage := tuf.HookStagePreCommit
		hookName := "test-hook"
		environment := tuf.HookEnvironmentLua
		timeout := 100
		principals := []string{rootPubKey.KeyID}

		state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		// Check that there are no hooks present
		rootMetadata, err := state.GetRootMetadata(false)
		assert.Nil(t, err)
		_, err = rootMetadata.GetHooks(tuf.HookStagePreCommit)
		assert.ErrorIs(t, err, tuf.ErrNoHooksDefined)

		// Attempt to remove without any hooks defined
		err = r.RemoveHook(testCtx, rootSigner, []tuf.HookStage{hookStage}, hookName, false)
		assert.ErrorIs(t, err, tuf.ErrNoHooksDefined)

		// Add hook
		if err := r.AddHook(testCtx, rootSigner, []tuf.HookStage{hookStage}, hookName, hookBytes, environment, principals, timeout, true, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}
		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)
		state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		// Check for hook
		rootMetadata, err = state.GetRootMetadata(false)
		assert.Nil(t, err)
		hooks, err := rootMetadata.GetHooks(tuf.HookStagePreCommit)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(hooks))

		// Remove hook
		err = r.RemoveHook(testCtx, rootSigner, []tuf.HookStage{hookStage}, hookName, false, trustpolicyopts.WithRSLEntry())
		assert.Nil(t, err)

		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)

		// Check that the hook was removed
		state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}
		rootMetadata, err = state.GetRootMetadata(false)
		assert.Nil(t, err)
		hooks, err = rootMetadata.GetHooks(tuf.HookStagePreCommit)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(hooks))
	})

	t.Run("valid pre-push hook", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")

		hookStage := tuf.HookStagePrePush
		hookName := "test-hook"
		environment := tuf.HookEnvironmentLua
		timeout := 100
		principals := []string{rootPubKey.KeyID}

		state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		// Check that there are no hooks present
		rootMetadata, err := state.GetRootMetadata(false)
		assert.Nil(t, err)
		_, err = rootMetadata.GetHooks(tuf.HookStagePrePush)
		assert.ErrorIs(t, err, tuf.ErrNoHooksDefined)

		// Attempt to remove without any hooks defined
		err = r.RemoveHook(testCtx, rootSigner, []tuf.HookStage{hookStage}, hookName, false)
		assert.ErrorIs(t, err, tuf.ErrNoHooksDefined)

		// Add hook
		if err := r.AddHook(testCtx, rootSigner, []tuf.HookStage{hookStage}, hookName, hookBytes, environment, principals, timeout, true, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}
		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)
		state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		// Check for hook
		rootMetadata, err = state.GetRootMetadata(false)
		assert.Nil(t, err)
		hooks, err := rootMetadata.GetHooks(tuf.HookStagePrePush)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(hooks))

		// Remove hook
		err = r.RemoveHook(testCtx, rootSigner, []tuf.HookStage{hookStage}, hookName, false, trustpolicyopts.WithRSLEntry())
		assert.Nil(t, err)
		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)

		// Check that the hook was removed
		state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}
		rootMetadata, err = state.GetRootMetadata(false)
		assert.Nil(t, err)
		hooks, err = rootMetadata.GetHooks(tuf.HookStagePrePush)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(hooks))
	})

	t.Run("with signCommit", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")
		hookStage := tuf.HookStagePreCommit
		hookName := "signed-remove-hook"
		environment := tuf.HookEnvironmentLua
		timeout := 100
		principals := []string{rootPubKey.KeyID}

		err := r.AddHook(testCtx, rootSigner, []tuf.HookStage{hookStage}, hookName, hookBytes, environment, principals, timeout, true)
		require.Nil(t, err)

		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)

		err = r.RemoveHook(testCtx, rootSigner, []tuf.HookStage{hookStage}, hookName, true)
		assert.Nil(t, err)
	})
}

func TestUpdateHook(t *testing.T) {
	t.Setenv(dev.DevModeKey, "1")

	rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	rootPubKey := tufv01.NewKeyFromSSLibKey(rootSigner.MetadataKey())

	t.Run("update pre-commit hook", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")

		hookStage := tuf.HookStagePreCommit
		hookName := "test-hook"
		environment := tuf.HookEnvironmentLua
		timeout := 100
		principals := []string{rootPubKey.KeyID}

		state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		rootMetadata, err := state.GetRootMetadata(false)
		assert.Nil(t, err)
		_, err = rootMetadata.GetHooks(tuf.HookStagePreCommit)
		assert.ErrorIs(t, err, tuf.ErrNoHooksDefined)

		err = r.AddHook(testCtx, rootSigner, []tuf.HookStage{hookStage}, hookName, hookBytes, environment, principals, timeout, true, trustpolicyopts.WithRSLEntry())
		assert.Nil(t, err)

		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)

		state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		rootMetadata, err = state.GetRootMetadata(false)
		assert.Nil(t, err)
		hooks, err := rootMetadata.GetHooks(tuf.HookStagePreCommit)
		assert.Nil(t, err)

		hookHash, err := r.r.WriteBlob(hookBytes)
		if err != nil {
			t.Fatal(err)
		}

		sha256Hash := sha256.New()
		sha256Hash.Write(hookBytes)
		sha256HashSum := hex.EncodeToString(sha256Hash.Sum(nil))

		preCommitHook := tufv01.Hook{
			Name:         hookName,
			PrincipalIDs: set.NewSetFromItems(rootPubKey.KeyID),
			Hashes:       map[string]string{gitinterface.GitBlobHashName: hookHash.String(), gitinterface.SHA256HashName: sha256HashSum},
			Environment:  environment,
			Timeout:      timeout,
		}
		preCommitHooks := []tuf.Hook{&preCommitHook}
		assert.Equal(t, preCommitHooks, hooks)

		newHookBytes := []byte("new hook content")
		newHookHash, err := r.r.WriteBlob(newHookBytes)
		if err != nil {
			t.Fatal(err)
		}

		newSha256Hash := sha256.New()
		newSha256Hash.Write(newHookBytes)
		newSha256HashSum := hex.EncodeToString(newSha256Hash.Sum(nil))

		err = r.UpdateHook(testCtx, rootSigner, []tuf.HookStage{hookStage}, hookName, newHookBytes, environment, principals, timeout,
			true, trustpolicyopts.WithRSLEntry())
		assert.Nil(t, err)
		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)

		state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		rootMetadata, err = state.GetRootMetadata(false)
		assert.Nil(t, err)
		hooks, err = rootMetadata.GetHooks(tuf.HookStagePreCommit)
		assert.Nil(t, err)

		preCommitHook = tufv01.Hook{
			Name:         hookName,
			PrincipalIDs: set.NewSetFromItems(rootPubKey.KeyID),
			Hashes:       map[string]string{gitinterface.GitBlobHashName: newHookHash.String(), gitinterface.SHA256HashName: newSha256HashSum},
			Environment:  environment,
			Timeout:      timeout,
		}
		preCommitHooks = []tuf.Hook{&preCommitHook}
		assert.Equal(t, preCommitHooks, hooks)
	})

	t.Run("update pre-push hook", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")

		hookStage := tuf.HookStagePrePush
		hookName := "test-hook"
		environment := tuf.HookEnvironmentLua
		timeout := 100
		principals := []string{rootPubKey.KeyID}

		state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		rootMetadata, err := state.GetRootMetadata(false)
		assert.Nil(t, err)
		_, err = rootMetadata.GetHooks(tuf.HookStagePrePush)
		assert.ErrorIs(t, err, tuf.ErrNoHooksDefined)

		err = r.AddHook(testCtx, rootSigner, []tuf.HookStage{hookStage}, hookName, hookBytes, environment, principals, timeout, true, trustpolicyopts.WithRSLEntry())
		assert.Nil(t, err)

		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)

		state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		rootMetadata, err = state.GetRootMetadata(false)
		assert.Nil(t, err)
		hooks, err := rootMetadata.GetHooks(tuf.HookStagePrePush)
		assert.Nil(t, err)

		hookHash, err := r.r.WriteBlob(hookBytes)
		if err != nil {
			t.Fatal(err)
		}

		sha256Hash := sha256.New()
		sha256Hash.Write(hookBytes)
		sha256HashSum := hex.EncodeToString(sha256Hash.Sum(nil))

		prePushHook := tufv01.Hook{
			Name:         hookName,
			PrincipalIDs: set.NewSetFromItems(rootPubKey.KeyID),
			Hashes:       map[string]string{gitinterface.GitBlobHashName: hookHash.String(), gitinterface.SHA256HashName: sha256HashSum},
			Environment:  environment,
			Timeout:      timeout,
		}
		prePushHooks := []tuf.Hook{&prePushHook}
		assert.Equal(t, prePushHooks, hooks)

		newHookBytes := []byte("new hook content")
		newHookHash, err := r.r.WriteBlob(newHookBytes)
		if err != nil {
			t.Fatal(err)
		}

		newSha256Hash := sha256.New()
		newSha256Hash.Write(newHookBytes)
		newSha256HashSum := hex.EncodeToString(newSha256Hash.Sum(nil))

		err = r.UpdateHook(testCtx, rootSigner, []tuf.HookStage{hookStage}, hookName, newHookBytes, environment, principals, timeout,
			true, trustpolicyopts.WithRSLEntry())
		assert.Nil(t, err)
		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)

		state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		rootMetadata, err = state.GetRootMetadata(false)
		assert.Nil(t, err)
		hooks, err = rootMetadata.GetHooks(tuf.HookStagePrePush)
		assert.Nil(t, err)

		prePushHook = tufv01.Hook{
			Name:         hookName,
			PrincipalIDs: set.NewSetFromItems(rootPubKey.KeyID),
			Hashes:       map[string]string{gitinterface.GitBlobHashName: newHookHash.String(), gitinterface.SHA256HashName: newSha256HashSum},
			Environment:  environment,
			Timeout:      timeout,
		}
		prePushHooks = []tuf.Hook{&prePushHook}
		assert.Equal(t, prePushHooks, hooks)
	})

	t.Run("invalid hook name", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")
		err := r.UpdateHook(testCtx, rootSigner, []tuf.HookStage{tuf.HookStagePreCommit}, "", hookBytes, tuf.HookEnvironmentLua, []string{rootPubKey.KeyID}, 100, false)
		assert.ErrorIs(t, err, ErrNoHookName)
	})

	t.Run("invalid hook timeout", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")
		err := r.UpdateHook(testCtx, rootSigner, []tuf.HookStage{tuf.HookStagePreCommit}, "test-hook", hookBytes, tuf.HookEnvironmentLua, []string{rootPubKey.KeyID}, 0, false)
		assert.ErrorIs(t, err, ErrInvalidHookTimeout)
	})
}

func TestIncrementRootVersion(t *testing.T) {
	r := createTestRepositoryWithRoot(t, "")

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyRef)
	require.Nil(t, err)

	oldRootMetadata, err := state.GetRootMetadata(false)
	require.Nil(t, err)

	rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

	err = r.IncrementRootVersion(testCtx, rootSigner, false)
	require.Nil(t, err)

	err = r.StagePolicy(testCtx, "", true, false)
	require.Nil(t, err)

	state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	newRootMetadata, err := state.GetRootMetadata(false)
	require.Nil(t, err)

	assert.Equal(t, oldRootMetadata.GetVersion()+1, newRootMetadata.GetVersion())

	// Check that the metadata is the same except for the version number
	newRootMetadata.(*tufv02.RootMetadata).Version = oldRootMetadata.GetVersion()
	assert.Equal(t, oldRootMetadata, newRootMetadata)

	t.Run("miscellaneous error checking", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		nr := &Repository{r: repo}

		// Test signCommit
		err := repo.SetGitConfig("user.signingkey", "")
		if err != nil {
			t.Fatal(err)
		}

		err = nr.IncrementRootVersion(testCtx, nil, true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)

		// Test non-existent policy
		sv := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)

		err = nr.IncrementRootVersion(testCtx, sv, false)
		assert.ErrorIs(t, err, rsl.ErrRSLEntryNotFound)

		// Test unauthorized signer
		r := createTestRepositoryWithRoot(t, "")

		err = r.IncrementRootVersion(testCtx, sv, false)
		assert.ErrorIs(t, err, ErrUnauthorizedKey)
	})
}

func TestListPropagationDirectives(t *testing.T) {
	t.Run("list propagation directives after add and remove", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")
		rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

		err := r.AddPropagationDirective(testCtx, rootSigner, "sync-main", "https://example.com/upstream", "refs/heads/main", "", "refs/heads/main", "upstream/", false)
		require.Nil(t, err)

		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)

		directives, err := r.ListPropagationDirectives(testCtx, policy.PolicyStagingRef)
		require.Nil(t, err)
		require.Len(t, directives, 1)

		assert.Equal(t, "sync-main", directives[0].GetName())
		assert.Equal(t, "https://example.com/upstream", directives[0].GetUpstreamRepository())
		assert.Equal(t, "refs/heads/main", directives[0].GetUpstreamReference())
		assert.Equal(t, "refs/heads/main", directives[0].GetDownstreamReference())

		err = r.RemovePropagationDirective(testCtx, rootSigner, "sync-main", false)
		require.Nil(t, err)

		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)

		directives, err = r.ListPropagationDirectives(testCtx, policy.PolicyStagingRef)
		require.Nil(t, err)
		assert.Empty(t, directives)
	})

	t.Run("with shorthand policy reference", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")
		rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

		err := r.AddPropagationDirective(testCtx, rootSigner, "sync-main", "https://example.com/upstream", "refs/heads/main", "", "refs/heads/main", "upstream/", false)
		require.Nil(t, err)

		err = r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)

		directives, err := r.ListPropagationDirectives(testCtx, "policy-staging")
		require.Nil(t, err)
		require.Len(t, directives, 1)

		assert.Equal(t, "sync-main", directives[0].GetName())
		assert.Equal(t, "https://example.com/upstream", directives[0].GetUpstreamRepository())
		assert.Equal(t, "refs/heads/main", directives[0].GetUpstreamReference())
		assert.Equal(t, "refs/heads/main", directives[0].GetDownstreamReference())
	})

	t.Run("returns error for unknown policy ref", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")

		directives, err := r.ListPropagationDirectives(testCtx, "does-not-exist")
		assert.Error(t, err)
		assert.Nil(t, directives)
	})
}

func TestEnableController(t *testing.T) {
	r := createTestRepositoryWithRoot(t, "")
	rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	require.Nil(t, err)

	rootMetadata, err := state.GetRootMetadata(false)
	require.Nil(t, err)
	assert.False(t, rootMetadata.IsController())

	err = r.EnableController(testCtx, rootSigner, false)
	require.Nil(t, err)

	err = r.StagePolicy(testCtx, "", true, false)
	require.Nil(t, err)

	state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	require.Nil(t, err)

	rootMetadata, err = state.GetRootMetadata(false)
	require.Nil(t, err)
	assert.True(t, rootMetadata.IsController())

	t.Run("miscellaneous error checking", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		nr := &Repository{r: repo}

		// Test signCommit
		err := repo.SetGitConfig("user.signingkey", "")
		if err != nil {
			t.Fatal(err)
		}

		err = nr.EnableController(testCtx, nil, true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)

		// Test non-existent policy
		sv := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)

		err = nr.EnableController(testCtx, sv, false)
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)

		// Test unauthorized signer
		r := createTestRepositoryWithRoot(t, "")

		err = r.EnableController(testCtx, sv, false)
		assert.ErrorIs(t, err, ErrUnauthorizedKey)
	})
}

func TestDisableController(t *testing.T) {
	r := createTestRepositoryWithRoot(t, "")
	rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

	err := r.EnableController(testCtx, rootSigner, false)
	require.Nil(t, err)

	err = r.StagePolicy(testCtx, "", true, false)
	require.Nil(t, err)

	err = r.DisableController(testCtx, rootSigner, false)
	require.Nil(t, err)

	err = r.StagePolicy(testCtx, "", true, false)
	require.Nil(t, err)

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	require.Nil(t, err)

	rootMetadata, err := state.GetRootMetadata(false)
	require.Nil(t, err)
	assert.False(t, rootMetadata.IsController())

	t.Run("miscellaneous error checking", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		nr := &Repository{r: repo}

		// Test signCommit
		err := repo.SetGitConfig("user.signingkey", "")
		if err != nil {
			t.Fatal(err)
		}

		err = nr.DisableController(testCtx, nil, true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)

		// Test non-existent policy
		sv := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)

		err = nr.DisableController(testCtx, sv, false)
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)

		// Test unauthorized signer
		r := createTestRepositoryWithRoot(t, "")

		err = r.DisableController(testCtx, sv, false)
		assert.ErrorIs(t, err, ErrUnauthorizedKey)
	})
}

func TestAddControllerRepository(t *testing.T) {
	r := createTestRepositoryWithRoot(t, "")
	rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	rootPrincipal := tufv01.NewKeyFromSSLibKey(rootSigner.MetadataKey())

	err := r.AddControllerRepository(testCtx, rootSigner, "controller-repo", "https://example.com/controller", []tuf.Principal{rootPrincipal}, false)
	require.Nil(t, err)

	err = r.StagePolicy(testCtx, "", true, false)
	require.Nil(t, err)

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	require.Nil(t, err)

	rootMetadata, err := state.GetRootMetadata(false)
	require.Nil(t, err)

	controllerRepos := rootMetadata.GetControllerRepositories()
	require.Len(t, controllerRepos, 1)
	assert.Equal(t, "controller-repo", controllerRepos[0].GetName())
	assert.Equal(t, "https://example.com/controller", controllerRepos[0].GetLocation())

	t.Run("miscellaneous error checking", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		nr := &Repository{r: repo}

		// Test signCommit
		err := repo.SetGitConfig("user.signingkey", "")
		if err != nil {
			t.Fatal(err)
		}

		err = nr.AddControllerRepository(testCtx, nil, "", "", nil, true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)

		// Test non-existent policy
		sv := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)

		err = nr.AddControllerRepository(testCtx, sv, "", "", nil, false)
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)

		// Test unauthorized signer
		r := createTestRepositoryWithRoot(t, "")

		err = r.AddControllerRepository(testCtx, sv, "", "", nil, false)
		assert.ErrorIs(t, err, ErrUnauthorizedKey)
	})
}

func TestAddNetworkRepository(t *testing.T) {
	r := createTestRepositoryWithRoot(t, "")
	rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	rootPrincipal := tufv01.NewKeyFromSSLibKey(rootSigner.MetadataKey())

	err := r.EnableController(testCtx, rootSigner, false)
	require.Nil(t, err)

	err = r.AddNetworkRepository(testCtx, rootSigner, "network-repo", "https://example.com/network", []tuf.Principal{rootPrincipal}, false)
	require.Nil(t, err)

	err = r.StagePolicy(testCtx, "", true, false)
	require.Nil(t, err)

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	require.Nil(t, err)

	rootMetadata, err := state.GetRootMetadata(false)
	require.Nil(t, err)

	networkRepos := rootMetadata.GetNetworkRepositories()
	require.Len(t, networkRepos, 1)
	assert.Equal(t, "network-repo", networkRepos[0].GetName())
	assert.Equal(t, "https://example.com/network", networkRepos[0].GetLocation())

	t.Run("requires controller", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")
		rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
		rootPrincipal := tufv01.NewKeyFromSSLibKey(rootSigner.MetadataKey())

		err := r.AddNetworkRepository(testCtx, rootSigner, "network-repo", "https://example.com/network", []tuf.Principal{rootPrincipal}, false)
		assert.ErrorIs(t, err, tuf.ErrNotAControllerRepository)
	})

	t.Run("miscellaneous error checking", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		nr := &Repository{r: repo}

		// Test signCommit
		err := repo.SetGitConfig("user.signingkey", "")
		if err != nil {
			t.Fatal(err)
		}

		err = nr.AddNetworkRepository(testCtx, nil, "", "", nil, true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)

		// Test non-existent policy
		sv := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)

		err = nr.AddNetworkRepository(testCtx, sv, "", "", nil, false)
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)

		// Test unauthorized signer
		r := createTestRepositoryWithRoot(t, "")

		err = r.AddNetworkRepository(testCtx, sv, "", "", nil, false)
		assert.ErrorIs(t, err, ErrUnauthorizedKey)
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
