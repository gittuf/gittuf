// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package listglobalrules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gittuf/gittuf/experimental/gittuf"
	rootopts "github.com/gittuf/gittuf/experimental/gittuf/options/root"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/cmd"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListGlobalRules(t *testing.T) {
	t.Run("no repository", func(t *testing.T) {
		tmpDir := t.TempDir()

		cwd, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(cwd) //nolint:errcheck

		require.NoError(t, os.Chdir(tmpDir))

		_, _, _, err = cmd.ExecuteCommandC(New())
		assert.ErrorContains(t, err, "unable to identify git directory")
	})

	t.Run("uninitialized policy", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		cwd, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(cwd) //nolint:errcheck

		require.NoError(t, os.Chdir(tmpDir))

		_, _, _, err = cmd.ExecuteCommandC(New())
		assert.ErrorContains(t, err, "unable to find RSL entry")
	})

	t.Run("success no rules", func(t *testing.T) {
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

		_, stdout, _, err := cmd.ExecuteCommandC(New(), "--target-ref", "policy-staging")
		assert.NoError(t, err)
		assert.Contains(t, stdout.String(), "No global rules are currently defined.")
	})

	t.Run("success with rules", func(t *testing.T) {
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

		// Add threshold global rule
		require.NoError(t, repo.AddGlobalRuleThreshold(t.Context(), signer, "require-approval-for-main", []string{"git:refs/heads/main", "file:src/*"}, 1, false, trustpolicyopts.WithRSLEntry()))

		// Add block force pushes global rule
		require.NoError(t, repo.AddGlobalRuleBlockForcePushes(t.Context(), signer, "block-force-pushes-for-main", []string{"git:refs/heads/main"}, false, trustpolicyopts.WithRSLEntry()))

		_, stdout, _, err := cmd.ExecuteCommandC(New(), "--target-ref", "policy-staging")
		assert.NoError(t, err)

		expected := `Global Rule: require-approval-for-main
    Type: threshold
    Paths affected:
        file:src/*
    Refs affected:
        git:refs/heads/main
    Threshold: 1
Global Rule: block-force-pushes-for-main
    Type: block-force-pushes
    Refs affected:
        git:refs/heads/main
`

		output := strings.ReplaceAll(stdout.String(), "\r\n", "\n")
		assert.Equal(t, expected, output)
	})
}
