// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package listprincipals

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/gittuf/gittuf/experimental/gittuf"
	rootopts "github.com/gittuf/gittuf/experimental/gittuf/options/root"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/gittuf/gittuf/internal/policy"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListPrincipals(t *testing.T) {
	t.Run("no repository", func(t *testing.T) {
		tmpDir := t.TempDir()

		t.Chdir(tmpDir)

		_, _, _, err := cmd.ExecuteCommandC(New())
		assert.ErrorContains(t, err, "unable to identify git directory")
	})

	t.Run("success empty", func(t *testing.T) {
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
		require.NoError(t, repo.InitializeRoot(t.Context(), signer, false, rootopts.WithRSLEntry()))
		newKey, err := gittuf.LoadPublicKey(keyPath + ".pub")
		require.NoError(t, err)

		require.NoError(t, repo.AddTopLevelTargetsKey(t.Context(), signer, newKey, false, trustpolicyopts.WithRSLEntry()))

		require.NoError(t, repo.InitializeTargets(t.Context(), signer, policy.TargetsRoleName, false, trustpolicyopts.WithRSLEntry()))

		require.NoError(t, repo.ApplyPolicy(t.Context(), "", true, false))

		_, stdout, _, err := cmd.ExecuteCommandC(New())
		assert.NoError(t, err)

		output := stdout.String()
		assert.Contains(t, output, "No principals are currently defined.")
		assert.NotContains(t, output, "Principal ")
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

	t.Run("success with multiple principals is sorted", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600))
		require.NoError(t, os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600))
		rsaKeyPath := filepath.Join(tmpDir, "rsa-key.pub")
		require.NoError(t, os.WriteFile(rsaKeyPath, artifacts.SSHRSAPublicSSH, 0o600))
		ecdsaKeyPath := filepath.Join(tmpDir, "ecdsa-key.pub")
		require.NoError(t, os.WriteFile(ecdsaKeyPath, artifacts.SSHECDSAPublicSSH, 0o600))

		t.Chdir(tmpDir)

		repo, err := gittuf.LoadRepository(".")
		require.NoError(t, err)
		signer, err := gittuf.LoadSigner(repo, keyPath)
		require.NoError(t, err)
		require.NoError(t, repo.InitializeRoot(t.Context(), signer, false, rootopts.WithRSLEntry()))
		newKey, err := gittuf.LoadPublicKey(keyPath + ".pub")
		require.NoError(t, err)
		rsaKey, err := gittuf.LoadPublicKey(rsaKeyPath)
		require.NoError(t, err)
		ecdsaKey, err := gittuf.LoadPublicKey(ecdsaKeyPath)
		require.NoError(t, err)

		require.NoError(t, repo.AddTopLevelTargetsKey(t.Context(), signer, newKey, false, trustpolicyopts.WithRSLEntry()))

		require.NoError(t, repo.InitializeTargets(t.Context(), signer, policy.TargetsRoleName, false, trustpolicyopts.WithRSLEntry()))

		person := &tufv02.Person{
			PersonID: "jane.doe",
			PublicKeys: map[string]*tufv02.Key{
				newKey.ID():   newKey.(*tufv02.Key),
				rsaKey.ID():   rsaKey.(*tufv02.Key),
				ecdsaKey.ID(): ecdsaKey.(*tufv02.Key),
			},
			AssociatedIdentities: map[string]string{
				"github": "jane-doe",
				"gitlab": "jdoe",
			},
			Custom: map[string]string{
				"team": "security",
				"role": "maintainer",
			},
		}

		require.NoError(t, repo.AddPrincipalToTargets(t.Context(), signer, policy.TargetsRoleName, []tuf.Principal{rsaKey, person, ecdsaKey, newKey}, false, trustpolicyopts.WithRSLEntry()))

		require.NoError(t, repo.ApplyPolicy(t.Context(), "", true, false))

		principals := []tuf.Principal{newKey, rsaKey, ecdsaKey, person}
		sort.Slice(principals, func(i, j int) bool {
			return principals[i].ID() < principals[j].ID()
		})

		expectedOutput := ""
		for i, principal := range principals {
			expectedOutput += fmt.Sprintf("Principal %s:\n    Keys:\n", principal.ID())

			keys := principal.Keys()
			sort.Slice(keys, func(i, j int) bool {
				return keys[i].KeyID < keys[j].KeyID
			})
			for _, key := range keys {
				expectedOutput += fmt.Sprintf("        %s (%s)\n", key.KeyID, key.KeyType)
			}

			if principal.ID() == person.ID() {
				expectedOutput += `    Custom Metadata:
        (associated identity) github: jane-doe
        (associated identity) gitlab: jdoe
        role: maintainer
        team: security
`
			}

			if i < len(principals)-1 {
				expectedOutput += "\n"
			}
		}

		// Map iteration order is random, so run the command a few times to
		// make sure the output doesn't change between runs
		for range 10 {
			_, stdout, _, err := cmd.ExecuteCommandC(New())
			require.NoError(t, err)
			assert.Equal(t, expectedOutput, strings.ReplaceAll(stdout.String(), "\r\n", "\n"))
		}
	})

	t.Run("success with custom policy name", func(t *testing.T) {
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
