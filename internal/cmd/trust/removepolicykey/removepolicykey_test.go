// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package removepolicykey

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemovePolicyKey(t *testing.T) {
	t.Run("no repository", func(t *testing.T) {
		tmpDir := t.TempDir()

		cwd, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(cwd) //nolint:errcheck

		require.NoError(t, os.Chdir(tmpDir))

		pOpts := &persistent.Options{
			SigningKey: "dummy-key",
		}
		_, _, _, err = cmd.ExecuteCommandC(New(pOpts), "--policy-key-ID", "dummy-policy-key-id")
		assert.ErrorContains(t, err, "unable to identify git directory")
	})

	t.Run("invalid signer", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		cwd, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(cwd) //nolint:errcheck

		require.NoError(t, os.Chdir(tmpDir))

		pOpts := &persistent.Options{
			SigningKey: "non-existent-key",
		}
		_, _, _, err = cmd.ExecuteCommandC(New(pOpts), "--policy-key-ID", "dummy-policy-key-id")
		assert.ErrorContains(t, err, "failed to run command")
	})

	t.Run("success", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600))
		require.NoError(t, os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600))

		newKeyPath := filepath.Join(tmpDir, "new-test-key")
		require.NoError(t, os.WriteFile(newKeyPath, artifacts.SSHRSAPrivate, 0o600))
		require.NoError(t, os.WriteFile(newKeyPath+".pub", artifacts.SSHRSAPublicSSH, 0o600))

		cwd, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(cwd) //nolint:errcheck

		require.NoError(t, os.Chdir(tmpDir))

		// Initialize the repository first
		repo, err := gittuf.LoadRepository(".")
		require.NoError(t, err)
		signer, err := gittuf.LoadSigner(repo, keyPath)
		require.NoError(t, err)
		require.NoError(t, repo.InitializeRoot(t.Context(), signer, false))

		targetsKey, err := gittuf.LoadPublicKey(newKeyPath + ".pub")
		require.NoError(t, err)

		// Also add the signer's own public key as a targets key so that after
		// removing targetsKey, at least one key remains to meet the threshold.
		signerPubKey, err := gittuf.LoadPublicKey(keyPath + ".pub")
		require.NoError(t, err)
		require.NoError(t, repo.AddTopLevelTargetsKey(t.Context(), signer, signerPubKey, true))

		// Add the RSA key so we can remove it
		require.NoError(t, repo.AddTopLevelTargetsKey(t.Context(), signer, targetsKey, true))

		pOpts := &persistent.Options{
			SigningKey: keyPath,
		}

		_, _, _, err = cmd.ExecuteCommandC(New(pOpts), "--policy-key-ID", targetsKey.ID())
		assert.NoError(t, err)
	})

	t.Run("success with RSL entry", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600))
		require.NoError(t, os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600))

		newKeyPath := filepath.Join(tmpDir, "new-test-key")
		require.NoError(t, os.WriteFile(newKeyPath, artifacts.SSHRSAPrivate, 0o600))
		require.NoError(t, os.WriteFile(newKeyPath+".pub", artifacts.SSHRSAPublicSSH, 0o600))

		cwd, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(cwd) //nolint:errcheck

		require.NoError(t, os.Chdir(tmpDir))

		// Initialize the repository first
		repo, err := gittuf.LoadRepository(".")
		require.NoError(t, err)
		signer, err := gittuf.LoadSigner(repo, keyPath)
		require.NoError(t, err)
		require.NoError(t, repo.InitializeRoot(t.Context(), signer, false))

		targetsKey, err := gittuf.LoadPublicKey(newKeyPath + ".pub")
		require.NoError(t, err)

		// Also add the signer's own public key as a targets key so that after
		// removing targetsKey, at least one key remains to meet the threshold.
		signerPubKey, err := gittuf.LoadPublicKey(keyPath + ".pub")
		require.NoError(t, err)
		require.NoError(t, repo.AddTopLevelTargetsKey(t.Context(), signer, signerPubKey, true))

		// Add the RSA key so we can remove it
		require.NoError(t, repo.AddTopLevelTargetsKey(t.Context(), signer, targetsKey, true))

		pOpts := &persistent.Options{
			SigningKey:   keyPath,
			WithRSLEntry: true,
		}

		_, _, _, err = cmd.ExecuteCommandC(New(pOpts), "--policy-key-ID", targetsKey.ID())
		assert.NoError(t, err)
	})
}
