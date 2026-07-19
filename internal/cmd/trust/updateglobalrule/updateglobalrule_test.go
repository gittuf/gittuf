// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package updateglobalrule

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/experimental/gittuf"
	rootopts "github.com/gittuf/gittuf/experimental/gittuf/options/root"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateGlobalRule(t *testing.T) {
	t.Run("no repository", func(t *testing.T) {
		tmpDir := t.TempDir()

		cwd, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(cwd) //nolint:errcheck

		require.NoError(t, os.Chdir(tmpDir))

		pOpts := &persistent.Options{
			SigningKey: "dummy-key",
		}

		_, _, _, err = cmd.ExecuteCommandC(New(pOpts), "--rule-name", "test-rule", "--type", tuf.GlobalRuleThresholdType, "--rule-pattern", "git:refs/heads/main")
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

		_, _, _, err = cmd.ExecuteCommandC(New(pOpts), "--rule-name", "test-rule", "--type", tuf.GlobalRuleThresholdType, "--rule-pattern", "git:refs/heads/main")
		assert.ErrorContains(t, err, "failed to run command")
	})

	t.Run("missing rule-pattern for threshold type", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600))
		require.NoError(t, os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600))

		cwd, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(cwd) //nolint:errcheck

		require.NoError(t, os.Chdir(tmpDir))

		pOpts := &persistent.Options{
			SigningKey: keyPath,
		}

		_, _, _, err = cmd.ExecuteCommandC(New(pOpts), "--rule-name", "test-rule", "--type", tuf.GlobalRuleThresholdType)
		assert.ErrorContains(t, err, "required flag --rule-pattern not set for global rule type 'threshold'")
	})

	t.Run("missing rule-pattern for block force pushes type", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600))
		require.NoError(t, os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600))

		cwd, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(cwd) //nolint:errcheck

		require.NoError(t, os.Chdir(tmpDir))

		pOpts := &persistent.Options{
			SigningKey: keyPath,
		}

		_, _, _, err = cmd.ExecuteCommandC(New(pOpts), "--rule-name", "test-rule", "--type", tuf.GlobalRuleBlockForcePushesType)
		assert.ErrorContains(t, err, "required flag --rule-pattern not set for global rule type 'block-force-pushes'")
	})

	t.Run("unknown rule type", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600))
		require.NoError(t, os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600))

		cwd, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(cwd) //nolint:errcheck

		require.NoError(t, os.Chdir(tmpDir))

		pOpts := &persistent.Options{
			SigningKey: keyPath,
		}

		_, _, _, err = cmd.ExecuteCommandC(New(pOpts), "--rule-name", "test-rule", "--type", "invalid-type", "--rule-pattern", "git:refs/heads/main")
		assert.ErrorIs(t, err, tuf.ErrUnknownGlobalRuleType)
	})

	t.Run("success with threshold type", func(t *testing.T) {
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

		require.NoError(t, repo.AddGlobalRuleThreshold(t.Context(), signer, "test-rule", []string{"git:refs/heads/main"}, 1, false, trustpolicyopts.WithRSLEntry()))

		pOpts := &persistent.Options{
			SigningKey: keyPath,
		}

		_, _, _, err = cmd.ExecuteCommandC(New(pOpts), "--rule-name", "test-rule", "--type", tuf.GlobalRuleThresholdType, "--rule-pattern", "git:refs/heads/main", "--rule-pattern", "git:refs/heads/dev", "--threshold", "2")
		assert.NoError(t, err)
	})

	t.Run("success with block force pushes type", func(t *testing.T) {
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

		require.NoError(t, repo.AddGlobalRuleBlockForcePushes(t.Context(), signer, "test-rule-bfp", []string{"git:refs/heads/main"}, false, trustpolicyopts.WithRSLEntry()))

		pOpts := &persistent.Options{
			SigningKey: keyPath,
		}

		_, _, _, err = cmd.ExecuteCommandC(New(pOpts), "--rule-name", "test-rule-bfp", "--type", tuf.GlobalRuleBlockForcePushesType, "--rule-pattern", "git:refs/heads/main", "--rule-pattern", "git:refs/heads/dev")
		assert.NoError(t, err)
	})

	t.Run("success with RSL entry", func(t *testing.T) {
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

		require.NoError(t, repo.AddGlobalRuleThreshold(t.Context(), signer, "test-rule", []string{"git:refs/heads/main"}, 1, false, trustpolicyopts.WithRSLEntry()))

		pOpts := &persistent.Options{
			SigningKey:   keyPath,
			WithRSLEntry: true,
		}

		_, _, _, err = cmd.ExecuteCommandC(New(pOpts), "--rule-name", "test-rule", "--type", tuf.GlobalRuleThresholdType, "--rule-pattern", "git:refs/heads/main")
		assert.NoError(t, err)
	})
}
