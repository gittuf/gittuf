// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package removeperson

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
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemovePerson(t *testing.T) {
	t.Run("no repository", func(t *testing.T) {
		tmpDir := t.TempDir()

		cwd, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(cwd) //nolint:errcheck

		require.NoError(t, os.Chdir(tmpDir))

		_, _, _, err = cmd.ExecuteCommandC(New(&persistent.Options{}), "--person-ID", "dummy-person")
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

		person := &tufv02.Person{
			PersonID:   "jane.doe",
			PublicKeys: map[string]*tufv02.Key{newKey.ID(): newKey.(*tufv02.Key)},
		}

		require.NoError(t, repo.AddPrincipalToTargets(t.Context(), signer, policy.TargetsRoleName, []tuf.Principal{person}, false, trustpolicyopts.WithRSLEntry()))

		command := New(&persistent.Options{SigningKey: keyPath, WithRSLEntry: true})
		_, _, _, err = cmd.ExecuteCommandC(command, "--person-ID", "jane.doe")
		assert.NoError(t, err)

		// Verification
		state, err := policy.LoadCurrentState(t.Context(), repo.GetGitRepository(), policy.PolicyStagingRef)
		require.NoError(t, err)

		targetsMetadata, err := state.GetTargetsMetadata(policy.TargetsRoleName, false)
		assert.Nil(t, err)

		_, ok := targetsMetadata.GetPrincipals()["jane.doe"]
		assert.False(t, ok)
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

		newKey, err := gittuf.LoadPublicKey(keyPath + ".pub")
		require.NoError(t, err)

		require.NoError(t, repo.InitializeTargets(t.Context(), signer, "custom-policy", false, trustpolicyopts.WithRSLEntry()))

		person := &tufv02.Person{
			PersonID:   "jane.doe",
			PublicKeys: map[string]*tufv02.Key{newKey.ID(): newKey.(*tufv02.Key)},
		}

		require.NoError(t, repo.AddPrincipalToTargets(t.Context(), signer, "custom-policy", []tuf.Principal{person}, false, trustpolicyopts.WithRSLEntry()))

		command := New(&persistent.Options{SigningKey: keyPath, WithRSLEntry: true})
		_, _, _, err = cmd.ExecuteCommandC(command, "--policy-name", "custom-policy", "--person-ID", "jane.doe")
		assert.NoError(t, err)

		// Verification
		state, err := policy.LoadCurrentState(t.Context(), repo.GetGitRepository(), policy.PolicyStagingRef)
		require.NoError(t, err)

		targetsMetadata, err := state.GetTargetsMetadata("custom-policy", false)
		assert.Nil(t, err)

		_, ok := targetsMetadata.GetPrincipals()["jane.doe"]
		assert.False(t, ok)
	})

	t.Run("failing signer", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		cwd, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(cwd) //nolint:errcheck

		require.NoError(t, os.Chdir(tmpDir))

		command := New(&persistent.Options{SigningKey: "invalid-key"})
		_, _, _, err = cmd.ExecuteCommandC(command, "--person-ID", "dummy-person")
		assert.ErrorContains(t, err, "failed to run command")
	})
}
