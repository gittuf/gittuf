// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package inspect

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/gittuf/gittuf/internal/policy"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInspect(t *testing.T) {
	t.Run("no repository", func(t *testing.T) {
		tmpDir := t.TempDir()

		t.Chdir(tmpDir)

		_, _, _, err := cmd.ExecuteCommandC(New())
		assert.ErrorContains(t, err, "unable to identify git directory")
	})

	t.Run("uninitialized policy", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		t.Chdir(tmpDir)

		_, _, _, err := cmd.ExecuteCommandC(New())
		assert.ErrorContains(t, err, "requested Git reference not found")
	})

	t.Run("success", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600))
		require.NoError(t, os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600))

		t.Chdir(tmpDir)

		repo, err := gittuf.LoadRepository(".")
		require.NoError(t, err)

		signer, err := gittuf.LoadSigner(repo, keyPath)
		require.NoError(t, err)

		require.NoError(t, repo.InitializeRoot(t.Context(), signer, false))

		newKey, err := gittuf.LoadPublicKey(repo, keyPath+".pub")
		require.NoError(t, err)

		require.NoError(t, repo.AddTopLevelTargetsKey(t.Context(), signer, newKey, false))

		require.NoError(t, repo.InitializeTargets(t.Context(), signer, policy.TargetsRoleName, false))

		initialCommitID, err := repo.GetGitRepository().GetReference(policy.PolicyStagingRef)
		require.NoError(t, err)

		require.NoError(t, repo.AddPrincipalToTargets(t.Context(), signer, policy.TargetsRoleName, []tuf.Principal{newKey}, false))
		require.NoError(t, repo.AddDelegation(t.Context(), signer, policy.TargetsRoleName, "protect-main", []string{newKey.ID()}, []string{"git:refs/heads/main"}, 1, false))

		currentCommitID, err := repo.GetGitRepository().GetReference(policy.PolicyStagingRef)
		require.NoError(t, err)

		_, stdout, _, err := cmd.ExecuteCommandC(New())
		assert.NoError(t, err)
		assert.Contains(t, stdout.String(), `"type": "targets"`)
		assert.Contains(t, stdout.String(), "protect-main")

		_, stdout, _, err = cmd.ExecuteCommandC(New(), "--revision", initialCommitID.String())
		assert.NoError(t, err)
		assert.Contains(t, stdout.String(), `"type": "targets"`)
		assert.NotContains(t, stdout.String(), "protect-main")

		_, stdout, _, err = cmd.ExecuteCommandC(New(), "--revision", currentCommitID.String())
		assert.NoError(t, err)
		assert.Contains(t, stdout.String(), `"type": "targets"`)
		assert.Contains(t, stdout.String(), "protect-main")
	})

	t.Run("invalid policy name", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600))
		require.NoError(t, os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600))

		t.Chdir(tmpDir)

		repo, err := gittuf.LoadRepository(".")
		require.NoError(t, err)

		signer, err := gittuf.LoadSigner(repo, keyPath)
		require.NoError(t, err)

		require.NoError(t, repo.InitializeRoot(t.Context(), signer, false))

		_, _, _, err = cmd.ExecuteCommandC(New(), "--policy-name", "does-not-exist")
		assert.ErrorContains(t, err, "unable to find requested metadata file")
	})

	t.Run("invalid revision hash", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600))
		require.NoError(t, os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600))

		t.Chdir(tmpDir)

		repo, err := gittuf.LoadRepository(".")
		require.NoError(t, err)

		signer, err := gittuf.LoadSigner(repo, keyPath)
		require.NoError(t, err)

		require.NoError(t, repo.InitializeRoot(t.Context(), signer, false))

		newKey, err := gittuf.LoadPublicKey(repo, keyPath+".pub")
		require.NoError(t, err)

		require.NoError(t, repo.AddTopLevelTargetsKey(t.Context(), signer, newKey, false))
		require.NoError(t, repo.InitializeTargets(t.Context(), signer, policy.TargetsRoleName, false))

		_, _, _, err = cmd.ExecuteCommandC(New(), "--revision", "invalid-commit-hash")
		assert.ErrorContains(t, err, "hash string is wrong length")
	})

	t.Run("non-existent revision commit", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600))
		require.NoError(t, os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600))

		t.Chdir(tmpDir)

		repo, err := gittuf.LoadRepository(".")
		require.NoError(t, err)

		signer, err := gittuf.LoadSigner(repo, keyPath)
		require.NoError(t, err)

		require.NoError(t, repo.InitializeRoot(t.Context(), signer, false))

		newKey, err := gittuf.LoadPublicKey(repo, keyPath+".pub")
		require.NoError(t, err)

		require.NoError(t, repo.AddTopLevelTargetsKey(t.Context(), signer, newKey, false))
		require.NoError(t, repo.InitializeTargets(t.Context(), signer, policy.TargetsRoleName, false))

		_, _, _, err = cmd.ExecuteCommandC(New(), "--revision", "0000000000000000000000000000000000000000")
		assert.ErrorContains(t, err, "unable to read commit")
	})
}
