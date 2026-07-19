// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package sign

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/experimental/gittuf"
	rootopts "github.com/gittuf/gittuf/experimental/gittuf/options/root"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/gittuf/gittuf/internal/policy"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSign(t *testing.T) {
	t.Run("no repository", func(t *testing.T) {
		tmpDir := t.TempDir()

		cwd, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(cwd) //nolint:errcheck

		require.NoError(t, os.Chdir(tmpDir))

		_, _, _, err = cmd.ExecuteCommandC(New(&persistent.Options{}))
		assert.ErrorContains(t, err, "unable to identify git directory")
	})

	t.Run("success", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600))
		require.NoError(t, os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600))

		keyPath2 := filepath.Join(tmpDir, "test-key-2")
		require.NoError(t, os.WriteFile(keyPath2, artifacts.SSHRSAPrivate, 0o600))
		require.NoError(t, os.WriteFile(keyPath2+".pub", artifacts.SSHRSAPublicSSH, 0o600))

		cwd, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(cwd) //nolint:errcheck

		require.NoError(t, os.Chdir(tmpDir))

		repo, err := gittuf.LoadRepository(".")
		require.NoError(t, err)
		signer, err := gittuf.LoadSigner(repo, keyPath)
		require.NoError(t, err)
		require.NoError(t, repo.InitializeRoot(t.Context(), signer, false, rootopts.WithRSLEntry()))

		newKey, err := gittuf.LoadPublicKey(keyPath + ".pub")
		require.NoError(t, err)

		require.NoError(t, repo.AddTopLevelTargetsKey(t.Context(), signer, newKey, false, trustpolicyopts.WithRSLEntry()))

		require.NoError(t, repo.InitializeTargets(t.Context(), signer, policy.TargetsRoleName, false, trustpolicyopts.WithRSLEntry()))

		// Initially, the TargetsEnvelope has 1 signature (from initialization)
		state, err := policy.LoadCurrentState(t.Context(), repo.GetGitRepository(), policy.PolicyStagingRef)
		require.NoError(t, err)
		assert.Len(t, state.Metadata.TargetsEnvelope.Signatures, 1)

		// Sign targets with key2
		command := New(&persistent.Options{SigningKey: keyPath2, WithRSLEntry: true})
		_, _, _, err = cmd.ExecuteCommandC(command)
		assert.NoError(t, err)

		// Verification
		state, err = policy.LoadCurrentState(t.Context(), repo.GetGitRepository(), policy.PolicyStagingRef)
		require.NoError(t, err)
		// Now, the TargetsEnvelope should have 2 signatures (one from keyPath, one from keyPath2)
		assert.Len(t, state.Metadata.TargetsEnvelope.Signatures, 2)
	})

	t.Run("success with custom policy name", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600))
		require.NoError(t, os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600))

		keyPath2 := filepath.Join(tmpDir, "test-key-2")
		require.NoError(t, os.WriteFile(keyPath2, artifacts.SSHRSAPrivate, 0o600))
		require.NoError(t, os.WriteFile(keyPath2+".pub", artifacts.SSHRSAPublicSSH, 0o600))

		cwd, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(cwd) //nolint:errcheck

		require.NoError(t, os.Chdir(tmpDir))

		repo, err := gittuf.LoadRepository(".")
		require.NoError(t, err)
		signer, err := gittuf.LoadSigner(repo, keyPath)
		require.NoError(t, err)
		require.NoError(t, repo.InitializeRoot(t.Context(), signer, false, rootopts.WithRSLEntry()))

		newKey, err := gittuf.LoadPublicKey(keyPath + ".pub")
		require.NoError(t, err)

		require.NoError(t, repo.AddTopLevelTargetsKey(t.Context(), signer, newKey, false, trustpolicyopts.WithRSLEntry()))

		require.NoError(t, repo.InitializeTargets(t.Context(), signer, policy.TargetsRoleName, false, trustpolicyopts.WithRSLEntry()))

		require.NoError(t, repo.AddPrincipalToTargets(t.Context(), signer, policy.TargetsRoleName, []tuf.Principal{newKey}, false, trustpolicyopts.WithRSLEntry()))

		require.NoError(t, repo.AddDelegation(t.Context(), signer, policy.TargetsRoleName, "custom-policy", []string{newKey.ID()}, []string{"*"}, 1, false, trustpolicyopts.WithRSLEntry()))

		require.NoError(t, repo.InitializeTargets(t.Context(), signer, "custom-policy", false, trustpolicyopts.WithRSLEntry()))

		// Verify custom-policy envelope initial signatures
		state, err := policy.LoadCurrentState(t.Context(), repo.GetGitRepository(), policy.PolicyStagingRef)
		require.NoError(t, err)
		assert.Len(t, state.Metadata.DelegationEnvelopes["custom-policy"].Signatures, 1)

		// Sign custom-policy with key2
		command := New(&persistent.Options{SigningKey: keyPath2, WithRSLEntry: true})
		_, _, _, err = cmd.ExecuteCommandC(command, "--policy-name", "custom-policy")
		assert.NoError(t, err)

		// Verification
		state, err = policy.LoadCurrentState(t.Context(), repo.GetGitRepository(), policy.PolicyStagingRef)
		require.NoError(t, err)
		assert.Len(t, state.Metadata.DelegationEnvelopes["custom-policy"].Signatures, 2)
	})

	t.Run("failing signer", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		cwd, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(cwd) //nolint:errcheck

		require.NoError(t, os.Chdir(tmpDir))

		command := New(&persistent.Options{SigningKey: "invalid-key"})
		_, _, _, err = cmd.ExecuteCommandC(command)
		assert.ErrorContains(t, err, "failed to run command")
	})

	t.Run("policy metadata not found", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600))
		require.NoError(t, os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600))

		cwd, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(cwd) //nolint:errcheck

		require.NoError(t, os.Chdir(tmpDir))

		repo, err := gittuf.LoadRepository(".")
		require.NoError(t, err)
		signer, err := gittuf.LoadSigner(repo, keyPath)
		require.NoError(t, err)
		require.NoError(t, repo.InitializeRoot(t.Context(), signer, false, rootopts.WithRSLEntry()))

		command := New(&persistent.Options{SigningKey: keyPath, WithRSLEntry: true})
		_, _, _, err = cmd.ExecuteCommandC(command, "--policy-name", "non-existent-policy")
		assert.ErrorIs(t, err, policy.ErrMetadataNotFound)
	})
}
