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
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListGlobalRules(t *testing.T) {
	createRepositoryWithRoot := func(t *testing.T, location string, privateKey, publicKey []byte) (*gittuf.Repository, *ssh.Signer) {
		t.Helper()
		gitinterface.CreateTestGitRepository(t, location, false)
		keyPath := filepath.Join(location, "test-key")
		require.NoError(t, os.WriteFile(keyPath, privateKey, 0o600))
		require.NoError(t, os.WriteFile(keyPath+".pub", publicKey, 0o600))
		repo, err := gittuf.LoadRepository(location)
		require.NoError(t, err)
		signer, err := ssh.NewSignerFromFile(keyPath)
		require.NoError(t, err)
		require.NoError(t, repo.InitializeRoot(t.Context(), signer, false, rootopts.WithRSLEntry()))
		return repo, signer
	}

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
		createRepositoryWithRoot(t, tmpDir, artifacts.SSHED25519Private, artifacts.SSHED25519PublicSSH)
		t.Chdir(tmpDir)

		_, stdout, _, err := cmd.ExecuteCommandC(New(), "--target-ref", "policy-staging")
		assert.NoError(t, err)
		assert.Equal(t, "No global rules are currently defined.\n", stdout.String())
	})

	t.Run("success with rules", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo, signer := createRepositoryWithRoot(t, tmpDir, artifacts.SSHED25519Private, artifacts.SSHED25519PublicSSH)
		t.Chdir(tmpDir)

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

	t.Run("propagated controller rules", func(t *testing.T) {
		t.Setenv(dev.DevModeKey, "1")

		// Controllers must have different initial root keys.
		controllerKeys := map[string]struct {
			private []byte
			public  []byte
		}{
			"controller-a": {
				private: artifacts.SSHECDSAPrivate,
				public:  artifacts.SSHECDSAPublicSSH,
			},
			"controller-b": {
				private: artifacts.SSHED25519Private,
				public:  artifacts.SSHED25519PublicSSH,
			},
		}

		localOutput := `Global Rule: require-approval-for-main
    Type: threshold
    Refs affected:
        git:refs/heads/main
    Threshold: 2
`
		controllerOutput := `Global Rule: require-approval-for-main
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

		singleControllerDirectory := t.TempDir()
		multipleControllerDirectory := t.TempDir()
		tests := map[string]struct {
			controllerDirectory string
			controllers         []string
			localRules          bool
			expected            string
		}{
			"one controller": {
				controllerDirectory: singleControllerDirectory,
				controllers:         []string{"controller-b"},
				expected: fmt.Sprintf("Controller repository: controller-b\n    Location: %s\n", filepath.Join(singleControllerDirectory, "controller-b")) +
					controllerOutput,
			},
			"local rules and controllers sorted by name": {
				controllerDirectory: multipleControllerDirectory,
				controllers:         []string{"controller-b", "controller-a"},
				localRules:          true,
				expected: localOutput +
					fmt.Sprintf("Controller repository: controller-a\n    Location: %s\n", filepath.Join(multipleControllerDirectory, "controller-a")) +
					controllerOutput +
					fmt.Sprintf("Controller repository: controller-b\n    Location: %s\n", filepath.Join(multipleControllerDirectory, "controller-b")) +
					controllerOutput,
			},
		}
		for name, test := range tests {
			t.Run(name, func(t *testing.T) {
				location := t.TempDir()
				repo, signer := createRepositoryWithRoot(t, location, artifacts.SSHRSAPrivate, artifacts.SSHRSAPublicSSH)
				if test.localRules {
					require.NoError(t, repo.AddGlobalRuleThreshold(t.Context(), signer, "require-approval-for-main", []string{"git:refs/heads/main"}, 2, false))
				}

				for _, controller := range test.controllers {
					controllerLocation := filepath.Join(test.controllerDirectory, controller)
					keys := controllerKeys[controller]
					controllerRepo, controllerSigner := createRepositoryWithRoot(t, controllerLocation, keys.private, keys.public)
					require.NoError(t, controllerRepo.EnableController(t.Context(), controllerSigner, false))
					require.NoError(t, controllerRepo.AddGlobalRuleBlockForcePushes(t.Context(), controllerSigner, "block-force-pushes-for-main", []string{"git:refs/heads/main"}, false))
					require.NoError(t, controllerRepo.AddGlobalRuleThreshold(t.Context(), controllerSigner, "require-approval-for-main", []string{"git:refs/heads/main", "file:src/*", "git:refs/tags/*", "file:docs/*"}, 3, false))
					require.NoError(t, controllerRepo.StagePolicy(t.Context(), "", true, false))
					require.NoError(t, controllerRepo.ApplyPolicy(t.Context(), "", true, false))
					require.NoError(t, repo.AddControllerRepository(t.Context(), signer, controller, controllerLocation, []tuf.Principal{tufv01.NewKeyFromSSLibKey(controllerSigner.MetadataKey())}, false))
				}
				require.NoError(t, repo.StagePolicy(t.Context(), "", true, false))
				require.NoError(t, repo.ApplyPolicy(t.Context(), "", true, false))
				require.NoError(t, repo.PropagateChangesFromUpstreamRepositories(t.Context(), false))
				t.Chdir(location)

				_, stdout, _, err := cmd.ExecuteCommandC(New())
				require.NoError(t, err)
				assert.Equal(t, test.expected, strings.ReplaceAll(stdout.String(), "\r\n", "\n"))
			})
		}
	})
}
