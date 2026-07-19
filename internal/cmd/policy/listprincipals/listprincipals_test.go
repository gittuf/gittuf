// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package listprincipals

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gittuf/gittuf/experimental/gittuf"
	rootopts "github.com/gittuf/gittuf/experimental/gittuf/options/root"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/gittuf/gittuf/internal/policy"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListPrincipals(t *testing.T) {
	t.Run("no repository", func(t *testing.T) {
		tmpDir := t.TempDir()

		cwd, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(cwd) //nolint:errcheck

		require.NoError(t, os.Chdir(tmpDir))

		_, _, _, err = cmd.ExecuteCommandC(New())
		assert.ErrorContains(t, err, "unable to identify git directory")
	})

	t.Run("success", func(t *testing.T) {
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
		newKey, err := gittuf.LoadPublicKey(keyPath + ".pub")
		require.NoError(t, err)

		require.NoError(t, repo.AddTopLevelTargetsKey(t.Context(), signer, newKey, false, trustpolicyopts.WithRSLEntry()))

		require.NoError(t, repo.InitializeTargets(t.Context(), signer, policy.TargetsRoleName, false, trustpolicyopts.WithRSLEntry()))

		require.NoError(t, repo.AddPrincipalToTargets(t.Context(), signer, policy.TargetsRoleName, []tuf.Principal{newKey}, false, trustpolicyopts.WithRSLEntry()))

		require.NoError(t, repo.ApplyPolicy(t.Context(), "", true, false))

		_, stdout, _, err := cmd.ExecuteCommandC(New())
		assert.NoError(t, err)

		expectedOutput := fmt.Sprintf(`Principal %s:
    Keys:
        %s (%s)
`, newKey.ID(), newKey.Keys()[0].KeyID, newKey.Keys()[0].KeyType)

		assert.Equal(t, expectedOutput, strings.ReplaceAll(stdout.String(), "\r\n", "\n"))
	})

	t.Run("success with custom policy name", func(t *testing.T) {
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
		require.NoError(t, repo.InitializeTargets(t.Context(), signer, "custom-policy", false, trustpolicyopts.WithRSLEntry()))

		newKey, err := gittuf.LoadPublicKey(keyPath + ".pub")
		require.NoError(t, err)
		require.NoError(t, repo.AddPrincipalToTargets(t.Context(), signer, "custom-policy", []tuf.Principal{newKey}, false, trustpolicyopts.WithRSLEntry()))

		require.NoError(t, repo.ApplyPolicy(t.Context(), "", true, false))

		_, stdout, _, err := cmd.ExecuteCommandC(New(), "--policy-name", "custom-policy")
		assert.NoError(t, err)

		expectedOutput := fmt.Sprintf(`Principal %s:
    Keys:
        %s (%s)
`, newKey.ID(), newKey.Keys()[0].KeyID, newKey.Keys()[0].KeyType)

		assert.Equal(t, expectedOutput, strings.ReplaceAll(stdout.String(), "\r\n", "\n"))
	})
}
