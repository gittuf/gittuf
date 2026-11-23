// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addrootperson

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/gittuf/gittuf/internal/policy"
	policyopts "github.com/gittuf/gittuf/internal/policy/options/policy"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/gittuf/gittuf/pkg/rsl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddRootPerson(t *testing.T) {
	t.Run("no repository", func(t *testing.T) {
		tmpDir := t.TempDir()

		t.Chdir(tmpDir)

		pOpts := &persistent.Options{
			SigningKey: "dummy-key",
		}
		_, _, _, err := cmd.ExecuteCommandC(New(pOpts), "--person-ID", "jane.doe@example.com", "--public-key", "dummy-pub-key")
		assert.ErrorContains(t, err, "unable to identify git directory")
	})

	t.Run("invalid signer", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		t.Chdir(tmpDir)

		pOpts := &persistent.Options{
			SigningKey: "non-existent-key",
		}
		_, _, _, err := cmd.ExecuteCommandC(New(pOpts), "--person-ID", "jane.doe@example.com", "--public-key", "dummy-pub-key")
		assert.ErrorContains(t, err, "failed to run command")
	})

	t.Run("invalid public key", func(t *testing.T) {
		_, keyPath, _ := setupTestRepository(t)

		pOpts := &persistent.Options{
			SigningKey: keyPath,
		}
		_, _, _, err := cmd.ExecuteCommandC(New(pOpts), "--person-ID", "jane.doe@example.com", "--public-key", "non-existent-pub-key")
		assert.ErrorContains(t, err, "failed to run command")
	})

	t.Run("invalid associated identity", func(t *testing.T) {
		_, keyPath, _ := setupTestRepository(t)

		pOpts := &persistent.Options{
			SigningKey: keyPath,
		}
		_, _, _, err := cmd.ExecuteCommandC(New(pOpts), "--person-ID", "jane.doe@example.com", "--public-key", keyPath+".pub", "--associated-identity", "invalid-format")
		assert.ErrorContains(t, err, "invalid format for associated identity")
	})

	t.Run("invalid custom metadata", func(t *testing.T) {
		_, keyPath, _ := setupTestRepository(t)

		pOpts := &persistent.Options{
			SigningKey: keyPath,
		}
		_, _, _, err := cmd.ExecuteCommandC(New(pOpts), "--person-ID", "jane.doe@example.com", "--public-key", keyPath+".pub", "--custom", "invalid-format")
		assert.ErrorContains(t, err, "invalid format for custom metadata")
	})

	t.Run("invalid custom metadata key", func(t *testing.T) {
		repo, keyPath, newKeyPath := setupTestRepository(t)
		before, err := repo.GetStorer().GetReference(policy.PolicyStagingRef)
		require.NoError(t, err)
		_, _, _, err = cmd.ExecuteCommandC(New(&persistent.Options{SigningKey: keyPath}), "--person-ID", "alice", "--public-key", newKeyPath+".pub", "--custom", "bad key=value")
		require.ErrorIs(t, err, tuf.ErrInvalidCustomMetadataKey)
		after, err := repo.GetStorer().GetReference(policy.PolicyStagingRef)
		require.NoError(t, err)
		assert.Equal(t, before, after)
	})

	t.Run("custom metadata value containing '='", func(t *testing.T) {
		_, keyPath, newKeyPath := setupTestRepository(t)

		pOpts := &persistent.Options{
			SigningKey: keyPath,
		}
		_, _, _, err := cmd.ExecuteCommandC(New(pOpts), "--person-ID", "jane.doe@example.com", "--public-key", newKeyPath+".pub", "--custom", "profile=https://example.com/u?ref=1")
		assert.NoError(t, err)
	})

	t.Run("success", func(t *testing.T) {
		repo, keyPath, newKeyPath := setupTestRepository(t)

		signer, err := gittuf.LoadSigner(repo, keyPath)
		require.NoError(t, err)
		existingKey, err := gittuf.LoadPublicKey(newKeyPath + ".pub")
		require.NoError(t, err)
		require.NoError(t, repo.AddRootKey(t.Context(), signer, existingKey, false))
		require.NoError(t, repo.UpdateRootThreshold(t.Context(), signer, 2, false))
		before, err := policy.LoadCurrentState(t.Context(), repo.GetStorer(), policy.PolicyStagingRef, policyopts.BypassRSL())
		require.NoError(t, err)
		oldRoot, err := before.GetRootMetadata(false)
		require.NoError(t, err)
		oldPrincipals, err := oldRoot.GetRootPrincipals()
		require.NoError(t, err)

		pOpts := &persistent.Options{
			SigningKey: keyPath,
		}
		_, _, _, err = cmd.ExecuteCommandC(New(pOpts), "--person-ID", "jane.doe@example.com", "--public-key", newKeyPath+".pub", "--public-key", keyPath+".pub", "--associated-identity", "github::jane-doe", "--custom", "department=engineering")
		require.NoError(t, err)
		state, err := policy.LoadCurrentState(t.Context(), repo.GetStorer(), policy.PolicyStagingRef, policyopts.BypassRSL())
		require.NoError(t, err)
		root, err := state.GetRootMetadata(false)
		require.NoError(t, err)
		principals, err := root.GetRootPrincipals()
		require.NoError(t, err)
		require.Len(t, principals, 3)
		for _, existing := range oldPrincipals {
			assert.Contains(t, principals, existing)
		}
		threshold, err := root.GetRootThreshold()
		require.NoError(t, err)
		assert.Equal(t, 2, threshold)
		assert.Equal(t, oldRoot.GetVersion()+1, root.GetVersion())
		_, err = rsl.GetLatestEntry(repo.GetStorer())
		assert.ErrorIs(t, err, rsl.ErrRSLEntryNotFound)
		var person *tufv02.Person
		for _, principal := range principals {
			if principal.ID() == "jane.doe@example.com" {
				person, _ = principal.(*tufv02.Person)
			}
		}
		require.NotNil(t, person)
		assert.Len(t, person.PublicKeys, 2)
		assert.Equal(t, "jane-doe", person.AssociatedIdentities["github"])
		assert.Equal(t, "engineering", person.Custom["department"])
	})

	t.Run("success with RSL entry", func(t *testing.T) {
		repo, keyPath, newKeyPath := setupTestRepository(t)

		pOpts := &persistent.Options{
			SigningKey:   keyPath,
			WithRSLEntry: true,
		}
		_, _, _, err := cmd.ExecuteCommandC(New(pOpts), "--person-ID", "jane.doe@example.com", "--public-key", newKeyPath+".pub")
		require.NoError(t, err)
		latest, err := rsl.GetLatestEntry(repo.GetStorer())
		require.NoError(t, err)
		entry, ok := latest.(*rsl.ReferenceEntry)
		require.True(t, ok)
		tip, err := repo.GetStorer().GetReference(policy.PolicyStagingRef)
		require.NoError(t, err)
		assert.Equal(t, policy.PolicyStagingRef, entry.RefName)
		assert.Equal(t, tip, entry.TargetID)
	})
}

func TestRequiredFlags(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		missing string
	}{
		{"person ID", []string{"--public-key", "dummy.pub"}, "person-ID"},
		{"public key", []string{"--person-ID", "alice"}, "public-key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, err := cmd.ExecuteCommandC(New(&persistent.Options{}), tc.args...)
			require.ErrorContains(t, err, "required flag")
			assert.ErrorContains(t, err, tc.missing)
		})
	}
}

func setupTestRepository(t *testing.T) (*gittuf.Repository, string, string) {
	t.Helper()
	tmpDir := t.TempDir()
	gitinterface.CreateTestGitRepository(t, tmpDir, false)
	keyPath := filepath.Join(tmpDir, "test-key")
	newKeyPath := filepath.Join(tmpDir, "new-test-key")
	require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600))
	require.NoError(t, os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600))
	require.NoError(t, os.WriteFile(newKeyPath, artifacts.SSHRSAPrivate, 0o600))
	require.NoError(t, os.WriteFile(newKeyPath+".pub", artifacts.SSHRSAPublicSSH, 0o600))
	t.Chdir(tmpDir)
	repo, err := gittuf.LoadRepository(".")
	require.NoError(t, err)
	signer, err := gittuf.LoadSigner(repo, keyPath)
	require.NoError(t, err)
	require.NoError(t, repo.InitializeRoot(t.Context(), signer, false))
	return repo, keyPath, newKeyPath
}
