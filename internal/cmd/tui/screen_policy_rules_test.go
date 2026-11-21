// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gittuf/gittuf/experimental/gittuf"
	rootopts "github.com/gittuf/gittuf/experimental/gittuf/options/root"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/gittuf/gittuf/internal/policy"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/gittuf/gittuf/pkg/rsl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolicyRulesHelpersWithRSLEntry(t *testing.T) {
	setup := func(t *testing.T, withRSL bool) (*options, *gittuf.Repository, string) {
		t.Helper()

		if runtime.GOOS == "windows" {
			t.Skip("skipping ssh-keygen signing on windows")
		}

		tmpDir := t.TempDir()
		t.Chdir(tmpDir)

		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600))
		require.NoError(t, os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600))

		repo, err := gittuf.LoadRepository(".")
		require.NoError(t, err)
		signer, err := gittuf.LoadSigner(repo, keyPath)
		require.NoError(t, err)
		require.NoError(t, repo.InitializeRoot(t.Context(), signer, false, rootopts.WithRSLEntry()))

		key, err := gittuf.LoadPublicKey(repo, keyPath+".pub")
		require.NoError(t, err)
		require.NoError(t, repo.AddTopLevelTargetsKey(t.Context(), signer, key, false, trustpolicyopts.WithRSLEntry()))
		require.NoError(t, repo.InitializeTargets(t.Context(), signer, policy.TargetsRoleName, false, trustpolicyopts.WithRSLEntry()))
		require.NoError(t, repo.AddPrincipalToTargets(t.Context(), signer, policy.TargetsRoleName, []tuf.Principal{key}, false, trustpolicyopts.WithRSLEntry()))

		opts := &options{
			p: &persistent.Options{
				SigningKey:   keyPath,
				WithRSLEntry: withRSL,
			},
			policyName: policy.TargetsRoleName,
		}

		return opts, repo, keyPath
	}

	t.Run("repoAddRule with create-rsl-entry creates RSL entry", func(t *testing.T) {
		opts, repo, keyPath := setup(t, true)
		key, err := gittuf.LoadPublicKey(repo, keyPath+".pub")
		require.NoError(t, err)

		initialRSL, err := rsl.GetLatestEntry(repo.GetStorer())
		require.NoError(t, err)

		r := rule{
			name:      "rule-1",
			pattern:   "refs/heads/main",
			threshold: 1,
		}

		err = repoAddRule(t.Context(), opts, r, []string{key.ID()}, []string{"refs/heads/main"})
		require.NoError(t, err)

		latestEntry, err := rsl.GetLatestEntry(repo.GetStorer())
		require.NoError(t, err)
		refEntry, isRefEntry := latestEntry.(*rsl.ReferenceEntry)
		require.True(t, isRefEntry)
		assert.Equal(t, policy.PolicyStagingRef, refEntry.RefName)
		assert.NotEqual(t, initialRSL.GetID(), latestEntry.GetID())
	})

	t.Run("repoAddRule without create-rsl-entry does not create RSL entry", func(t *testing.T) {
		opts, repo, keyPath := setup(t, false)
		key, err := gittuf.LoadPublicKey(repo, keyPath+".pub")
		require.NoError(t, err)

		initialRSL, err := rsl.GetLatestEntry(repo.GetStorer())
		require.NoError(t, err)

		r := rule{
			name:      "rule-1",
			pattern:   "refs/heads/main",
			threshold: 1,
		}

		err = repoAddRule(t.Context(), opts, r, []string{key.ID()}, []string{"refs/heads/main"})
		require.NoError(t, err)

		latestEntry, err := rsl.GetLatestEntry(repo.GetStorer())
		require.NoError(t, err)
		assert.Equal(t, initialRSL.GetID(), latestEntry.GetID())
	})

	t.Run("repoUpdateRule with create-rsl-entry creates RSL entry", func(t *testing.T) {
		opts, repo, keyPath := setup(t, true)
		key, err := gittuf.LoadPublicKey(repo, keyPath+".pub")
		require.NoError(t, err)

		r := rule{
			name:      "rule-1",
			pattern:   "refs/heads/main",
			threshold: 1,
		}
		require.NoError(t, repoAddRule(t.Context(), opts, r, []string{key.ID()}, []string{"refs/heads/main"}))

		afterAddRSL, err := rsl.GetLatestEntry(repo.GetStorer())
		require.NoError(t, err)

		err = repoUpdateRule(t.Context(), opts, r, []string{key.ID()}, []string{"refs/heads/main", "refs/heads/feature"})
		require.NoError(t, err)

		latestEntry, err := rsl.GetLatestEntry(repo.GetStorer())
		require.NoError(t, err)
		assert.NotEqual(t, afterAddRSL.GetID(), latestEntry.GetID())
	})

	t.Run("repoRemoveRule with create-rsl-entry creates RSL entry", func(t *testing.T) {
		opts, repo, keyPath := setup(t, true)
		key, err := gittuf.LoadPublicKey(repo, keyPath+".pub")
		require.NoError(t, err)

		r := rule{
			name:      "rule-1",
			pattern:   "refs/heads/main",
			threshold: 1,
		}
		require.NoError(t, repoAddRule(t.Context(), opts, r, []string{key.ID()}, []string{"refs/heads/main"}))

		afterAddRSL, err := rsl.GetLatestEntry(repo.GetStorer())
		require.NoError(t, err)

		err = repoRemoveRule(t.Context(), opts, r)
		require.NoError(t, err)

		latestEntry, err := rsl.GetLatestEntry(repo.GetStorer())
		require.NoError(t, err)
		assert.NotEqual(t, afterAddRSL.GetID(), latestEntry.GetID())
	})

	t.Run("repoReorderRules with create-rsl-entry creates RSL entry", func(t *testing.T) {
		opts, repo, keyPath := setup(t, true)
		key, err := gittuf.LoadPublicKey(repo, keyPath+".pub")
		require.NoError(t, err)

		r1 := rule{name: "rule-1", pattern: "refs/heads/main", threshold: 1}
		r2 := rule{name: "rule-2", pattern: "refs/heads/feature", threshold: 1}
		require.NoError(t, repoAddRule(t.Context(), opts, r1, []string{key.ID()}, []string{"refs/heads/main"}))
		require.NoError(t, repoAddRule(t.Context(), opts, r2, []string{key.ID()}, []string{"refs/heads/feature"}))

		afterAddsRSL, err := rsl.GetLatestEntry(repo.GetStorer())
		require.NoError(t, err)

		err = repoReorderRules(t.Context(), opts, []rule{r2, r1})
		require.NoError(t, err)

		latestEntry, err := rsl.GetLatestEntry(repo.GetStorer())
		require.NoError(t, err)
		assert.NotEqual(t, afterAddsRSL.GetID(), latestEntry.GetID())
	})
}
