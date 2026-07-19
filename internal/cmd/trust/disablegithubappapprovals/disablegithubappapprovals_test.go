// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package disablegithubappapprovals

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/gittuf/gittuf/internal/policy"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDisableGitHubAppApprovals(t *testing.T) {
	t.Run("no repository", func(t *testing.T) {
		tmpDir := t.TempDir()
		currentDir, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(currentDir) //nolint:errcheck

		pOpts := &persistent.Options{
			SigningKey: "dummy-key",
		}
		_, _, _, err = cmd.ExecuteCommandC(New(pOpts))
		assert.ErrorContains(t, err, "unable to identify git directory")
	})

	t.Run("invalid signer", func(t *testing.T) {
		tmpDir := t.TempDir()
		currentDir, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(currentDir) //nolint:errcheck

		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		pOpts := &persistent.Options{
			SigningKey: "non-existent-key",
		}
		_, _, _, err = cmd.ExecuteCommandC(New(pOpts))
		assert.ErrorContains(t, err, "failed to run command")
	})

	t.Run("success already untrusted", func(t *testing.T) {
		tmpDir := t.TempDir()
		currentDir, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(currentDir) //nolint:errcheck

		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600))
		require.NoError(t, os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600))

		repo, err := gittuf.LoadRepository(".")
		require.NoError(t, err)
		signer, err := gittuf.LoadSigner(repo, keyPath)
		require.NoError(t, err)
		require.NoError(t, repo.InitializeRoot(t.Context(), signer, false))

		pOpts := &persistent.Options{
			SigningKey: keyPath,
		}
		_, _, _, err = cmd.ExecuteCommandC(New(pOpts), "--app-name", "github-app")
		assert.NoError(t, err)
	})

	t.Run("success disable trusted app", func(t *testing.T) {
		tmpDir := t.TempDir()
		currentDir, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(currentDir) //nolint:errcheck

		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600))
		require.NoError(t, os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600))

		repo, err := gittuf.LoadRepository(".")
		require.NoError(t, err)
		signer, err := gittuf.LoadSigner(repo, keyPath)
		require.NoError(t, err)
		require.NoError(t, repo.InitializeRoot(t.Context(), signer, false))

		key, err := gittuf.LoadPublicKey(keyPath)
		require.NoError(t, err)

		err = repo.AddGitHubApp(t.Context(), signer, "github-app", key, false)
		assert.NoError(t, err)

		err = repo.TrustGitHubApp(t.Context(), signer, "github-app", false)
		assert.NoError(t, err)

		err = repo.StagePolicy(t.Context(), "", true, false)
		assert.NoError(t, err)

		state, err := policy.LoadCurrentState(t.Context(), repo.GetGitRepository(), policy.PolicyStagingRef)
		assert.NoError(t, err)
		rootMetadata, err := state.GetRootMetadata(false)
		assert.NoError(t, err)
		assert.True(t, rootMetadata.IsGitHubAppApprovalTrusted("github-app"))

		pOpts := &persistent.Options{
			SigningKey: keyPath,
		}
		_, _, _, err = cmd.ExecuteCommandC(New(pOpts), "--app-name", "github-app")
		assert.NoError(t, err)

		err = repo.StagePolicy(t.Context(), "", true, false)
		assert.NoError(t, err)

		state, err = policy.LoadCurrentState(t.Context(), repo.GetGitRepository(), policy.PolicyStagingRef)
		assert.NoError(t, err)
		rootMetadata, err = state.GetRootMetadata(false)
		assert.NoError(t, err)
		assert.False(t, rootMetadata.IsGitHubAppApprovalTrusted("github-app"))
	})

	t.Run("success disable trusted app with RSL entry", func(t *testing.T) {
		tmpDir := t.TempDir()
		currentDir, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(currentDir) //nolint:errcheck

		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600))
		require.NoError(t, os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600))

		repo, err := gittuf.LoadRepository(".")
		require.NoError(t, err)
		signer, err := gittuf.LoadSigner(repo, keyPath)
		require.NoError(t, err)
		require.NoError(t, repo.InitializeRoot(t.Context(), signer, false))

		key, err := gittuf.LoadPublicKey(keyPath)
		require.NoError(t, err)

		err = repo.AddGitHubApp(t.Context(), signer, "github-app", key, false)
		assert.NoError(t, err)

		err = repo.TrustGitHubApp(t.Context(), signer, "github-app", false)
		assert.NoError(t, err)

		err = repo.StagePolicy(t.Context(), "", true, false)
		assert.NoError(t, err)

		pOpts := &persistent.Options{
			SigningKey:   keyPath,
			WithRSLEntry: true,
		}
		_, _, _, err = cmd.ExecuteCommandC(New(pOpts), "--app-name", "github-app")
		assert.NoError(t, err)
	})
}
