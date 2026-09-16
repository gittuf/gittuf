// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package listglobalrules

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
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
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

		_, stdout, _, err := cmd.ExecuteCommandC(New())
		assert.ErrorContains(t, err, "unable to find RSL entry")
		assert.NotContains(t, stdout.String(), "No global rules are currently defined.")
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
		assert.Equal(t, "No global rules are currently defined.\n", stdout.String())
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

func TestListGlobalRulesWithControllers(t *testing.T) {
	t.Setenv(dev.DevModeKey, "1")

	const localOutput = `Global Rule: require-approval-for-main
    Type: threshold
    Refs affected:
        git:refs/heads/main
    Threshold: 2
`
	const controllerOutput = `Global Rule: require-approval-for-main
    Type: threshold
    Paths affected:
        file:src/*
        file:docs/*
    Refs affected:
        git:refs/heads/main
        git:refs/tags/*
    Threshold: 3
Global Rule: block-force-pushes-for-main
    Type: block-force-pushes
    Refs affected:
        git:refs/heads/main
`

	for _, test := range []struct {
		name            string
		controllers     int
		localRules      bool
		controllerRules bool
		targets         bool
	}{
		{name: "controller without rules", controllers: 1},
		{name: "local rules and empty controller", controllers: 1, localRules: true},
		{name: "one controller", controllers: 1, controllerRules: true},
		{name: "local and controller rules", controllers: 1, localRules: true, controllerRules: true},
		{name: "multiple controllers in sorted order", controllers: 2, controllerRules: true},
		{name: "local and multiple controllers with targets metadata", controllers: 2, localRules: true, controllerRules: true, targets: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			location := t.TempDir()
			repo, signer := createRepositoryWithRoot(t, location, artifacts.SSHRSAPrivate, artifacts.SSHRSAPublicSSH)
			if test.localRules {
				require.NoError(t, repo.AddGlobalRuleThreshold(t.Context(), signer, "require-approval-for-main", []string{"git:refs/heads/main"}, 2, false))
			}
			if test.targets {
				require.NoError(t, repo.AddTopLevelTargetsKey(t.Context(), signer, tufv01.NewKeyFromSSLibKey(signer.MetadataKey()), false))
				require.NoError(t, repo.InitializeTargets(t.Context(), signer, policy.TargetsRoleName, false))
			}

			controllerOutputs := ""
			// Register controllers in reverse order to check sorting.
			for i, controller := range []struct {
				name       string
				privateKey []byte
				publicKey  []byte
			}{
				{"controller-b", artifacts.SSHED25519Private, artifacts.SSHED25519PublicSSH},
				{"controller-a", artifacts.SSHECDSAPrivate, artifacts.SSHECDSAPublicSSH},
			} {
				if i >= test.controllers {
					break
				}
				controllerLocation := t.TempDir()
				controllerRepo, controllerSigner := createRepositoryWithRoot(t, controllerLocation, controller.privateKey, controller.publicKey)
				require.NoError(t, controllerRepo.EnableController(t.Context(), controllerSigner, false))
				if test.controllerRules {
					require.NoError(t, controllerRepo.AddGlobalRuleBlockForcePushes(t.Context(), controllerSigner, "block-force-pushes-for-main", []string{"git:refs/heads/main"}, false))
					require.NoError(t, controllerRepo.AddGlobalRuleThreshold(t.Context(), controllerSigner, "require-approval-for-main", []string{"git:refs/heads/main", "file:src/*", "git:refs/tags/*", "file:docs/*"}, 3, false))
					controllerOutputs = fmt.Sprintf("Controller repository: %s\n    Location: %s\n", controller.name, controllerLocation) + controllerOutput + controllerOutputs
				}
				require.NoError(t, controllerRepo.StagePolicy(t.Context(), "", true, false))
				require.NoError(t, controllerRepo.ApplyPolicy(t.Context(), "", true, false))
				require.NoError(t, repo.AddControllerRepository(t.Context(), signer, controller.name, controllerLocation, []tuf.Principal{tufv01.NewKeyFromSSLibKey(controllerSigner.MetadataKey())}, false))
			}
			require.NoError(t, repo.StagePolicy(t.Context(), "", true, false))
			require.NoError(t, repo.ApplyPolicy(t.Context(), "", true, false))
			require.NoError(t, repo.PropagateChangesFromUpstreamRepositories(t.Context(), false))
			t.Chdir(location)

			expected := controllerOutputs
			if test.localRules {
				expected = localOutput + expected
			}
			if expected == "" {
				expected = "No global rules are currently defined.\n"
			}
			for range 3 {
				_, stdout, _, err := cmd.ExecuteCommandC(New())
				require.NoError(t, err)
				assert.Equal(t, expected, strings.ReplaceAll(stdout.String(), "\r\n", "\n"))
			}

			localRules, err := repo.ListGlobalRules(t.Context(), policy.PolicyRef)
			require.NoError(t, err)
			if test.localRules {
				require.Len(t, localRules, 1)
				assert.Equal(t, "require-approval-for-main", localRules[0].GetName())
			} else {
				assert.Empty(t, localRules)
			}
		})
	}
}

func createRepositoryWithRoot(t *testing.T, location string, privateKey, publicKey []byte) (*gittuf.Repository, *ssh.Signer) {
	t.Helper()
	gitinterface.CreateTestGitRepository(t, location, false)
	keyPath := filepath.Join(location, "test-key")
	require.NoError(t, os.WriteFile(keyPath, privateKey, 0o600))
	require.NoError(t, os.WriteFile(keyPath+".pub", publicKey, 0o600))
	repo, err := gittuf.LoadRepository(location)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromFile(keyPath)
	require.NoError(t, err)
	require.NoError(t, repo.InitializeRoot(t.Context(), signer, false))
	return repo, signer
}
