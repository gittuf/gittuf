// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package listrules

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

func TestListRules(t *testing.T) {
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

		require.NoError(t, repo.AddDelegation(t.Context(), signer, policy.TargetsRoleName, "protect-main", []string{newKey.ID()}, []string{"git:refs/heads/main", "file:path/to/file"}, 1, false, trustpolicyopts.WithRSLEntry()))

		require.NoError(t, repo.AddDelegation(t.Context(), signer, policy.TargetsRoleName, "protect-tags", []string{newKey.ID()}, []string{"git:refs/tags/"}, 1, false, trustpolicyopts.WithRSLEntry()))

		require.NoError(t, repo.ApplyPolicy(t.Context(), "", true, false))

		_, stdout, _, err := cmd.ExecuteCommandC(New())
		assert.NoError(t, err)

		out := strings.ReplaceAll(stdout.String(), "\r\n", "\n")
		expectedProtectMain := fmt.Sprintf(
			"Rule protect-main:\n    Paths affected:\n        file:path/to/file\n    Refs affected:\n        git:refs/heads/main\n    Authorized keys:\n        %s\n    Required valid signatures: 1",
			newKey.ID(),
		)
		expectedProtectTags := fmt.Sprintf(
			"Rule protect-tags:\n    Refs affected:\n        git:refs/tags/\n    Authorized keys:\n        %s\n    Required valid signatures: 1",
			newKey.ID(),
		)
		assert.Contains(t, out, expectedProtectMain)
		assert.Contains(t, out, expectedProtectTags)
	})

	t.Run("invalid target ref", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		cwd, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(cwd) //nolint:errcheck

		require.NoError(t, os.Chdir(tmpDir))

		command := New()
		command.SetArgs([]string{"--target-ref", "refs/gittuf/invalid"})
		_, _, _, err = cmd.ExecuteCommandC(command)
		assert.ErrorContains(t, err, "unable to find RSL entry")
	})
}
