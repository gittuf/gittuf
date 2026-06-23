// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package updaterule

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
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestUpdateRule(t *testing.T) {
	t.Run("no repository", func(t *testing.T) {
		tmpDir := t.TempDir()

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = os.Chdir(cwd)
		}()

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}

		_, _, _, err = cmd.ExecuteCommandC(New(&persistent.Options{}), "--rule-name", "rule-1", "--authorize", "principal-1", "--rule-pattern", "git:refs/heads/main")
		assert.ErrorContains(t, err, "unable to identify git directory")
	})

	t.Run("success", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		if err := os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600); err != nil {
			t.Fatal(err)
		}

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = os.Chdir(cwd)
		}()

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}

		repo, err := gittuf.LoadRepository(".")
		if err != nil {
			t.Fatal(err)
		}
		signer, err := gittuf.LoadSigner(repo, keyPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.InitializeRoot(t.Context(), signer, false, rootopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		newKey, err := gittuf.LoadPublicKey(keyPath + ".pub")
		if err != nil {
			t.Fatal(err)
		}

		if err := repo.AddTopLevelTargetsKey(t.Context(), signer, newKey, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		if err := repo.InitializeTargets(t.Context(), signer, policy.TargetsRoleName, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		if err := repo.AddPrincipalToTargets(t.Context(), signer, policy.TargetsRoleName, []tuf.Principal{newKey}, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		if err := repo.AddDelegation(t.Context(), signer, policy.TargetsRoleName, "rule-1", []string{newKey.ID()}, []string{"git:refs/heads/main"}, 1, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		// Initial check
		state, err := policy.LoadCurrentState(t.Context(), repo.GetGitRepository(), policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}
		targetsMetadata, err := state.GetTargetsMetadata(policy.TargetsRoleName, false)
		assert.Nil(t, err)
		rules := targetsMetadata.GetRules()
		assert.Equal(t, "rule-1", rules[0].ID())
		assert.Equal(t, []string{"git:refs/heads/main"}, rules[0].GetProtectedNamespaces())
		assert.Equal(t, 1, rules[0].GetThreshold())

		// Update rule
		command := New(&persistent.Options{SigningKey: keyPath, WithRSLEntry: true})
		_, _, _, err = cmd.ExecuteCommandC(command, "--rule-name", "rule-1", "--authorize-key", keyPath+".pub", "--rule-pattern", "git:refs/heads/feature", "--threshold", "1")
		assert.NoError(t, err)

		// Verification
		state, err = policy.LoadCurrentState(t.Context(), repo.GetGitRepository(), policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}
		targetsMetadata, err = state.GetTargetsMetadata(policy.TargetsRoleName, false)
		assert.Nil(t, err)
		rules = targetsMetadata.GetRules()
		assert.Equal(t, "rule-1", rules[0].ID())
		assert.Equal(t, []string{"git:refs/heads/feature"}, rules[0].GetProtectedNamespaces())
		assert.Equal(t, 1, rules[0].GetThreshold())
	})

	t.Run("success with custom policy name", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		if err := os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600); err != nil {
			t.Fatal(err)
		}

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = os.Chdir(cwd)
		}()

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}

		repo, err := gittuf.LoadRepository(".")
		if err != nil {
			t.Fatal(err)
		}
		signer, err := gittuf.LoadSigner(repo, keyPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.InitializeRoot(t.Context(), signer, false, rootopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		newKey, err := gittuf.LoadPublicKey(keyPath + ".pub")
		if err != nil {
			t.Fatal(err)
		}

		if err := repo.AddTopLevelTargetsKey(t.Context(), signer, newKey, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		if err := repo.InitializeTargets(t.Context(), signer, policy.TargetsRoleName, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		if err := repo.AddPrincipalToTargets(t.Context(), signer, policy.TargetsRoleName, []tuf.Principal{newKey}, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		if err := repo.AddDelegation(t.Context(), signer, policy.TargetsRoleName, "custom-policy", []string{newKey.ID()}, []string{"*"}, 1, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		if err := repo.InitializeTargets(t.Context(), signer, "custom-policy", false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		if err := repo.AddPrincipalToTargets(t.Context(), signer, "custom-policy", []tuf.Principal{newKey}, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		if err := repo.AddDelegation(t.Context(), signer, "custom-policy", "rule-1", []string{newKey.ID()}, []string{"git:refs/heads/main"}, 1, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		// Update rule in custom-policy
		command := New(&persistent.Options{SigningKey: keyPath, WithRSLEntry: true})
		_, _, _, err = cmd.ExecuteCommandC(command, "--policy-name", "custom-policy", "--rule-name", "rule-1", "--authorize", newKey.ID(), "--rule-pattern", "git:refs/heads/feature")
		assert.NoError(t, err)

		// Verification
		state, err := policy.LoadCurrentState(t.Context(), repo.GetGitRepository(), policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}
		targetsMetadata, err := state.GetTargetsMetadata("custom-policy", false)
		assert.Nil(t, err)
		rules := targetsMetadata.GetRules()
		assert.Equal(t, "rule-1", rules[0].ID())
		assert.Equal(t, []string{"git:refs/heads/feature"}, rules[0].GetProtectedNamespaces())
	})

	t.Run("failing signer", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = os.Chdir(cwd)
		}()

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}

		command := New(&persistent.Options{SigningKey: "invalid-key"})
		_, _, _, err = cmd.ExecuteCommandC(command, "--rule-name", "rule-1", "--authorize", "principal-1", "--rule-pattern", "git:refs/heads/main")
		assert.ErrorContains(t, err, "failed to run command")
	})

	t.Run("policy metadata not found", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		if err := os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600); err != nil {
			t.Fatal(err)
		}

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = os.Chdir(cwd)
		}()

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}

		repo, err := gittuf.LoadRepository(".")
		if err != nil {
			t.Fatal(err)
		}
		signer, err := gittuf.LoadSigner(repo, keyPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.InitializeRoot(t.Context(), signer, false, rootopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		command := New(&persistent.Options{SigningKey: keyPath, WithRSLEntry: true})
		_, _, _, err = cmd.ExecuteCommandC(command, "--policy-name", "non-existent-policy", "--rule-name", "rule-1", "--authorize", "principal-1", "--rule-pattern", "git:refs/heads/main")
		assert.ErrorIs(t, err, policy.ErrMetadataNotFound)
	})

	t.Run("cannot manipulate rule with gittuf prefix", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		if err := os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600); err != nil {
			t.Fatal(err)
		}

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = os.Chdir(cwd)
		}()

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}

		repo, err := gittuf.LoadRepository(".")
		if err != nil {
			t.Fatal(err)
		}
		signer, err := gittuf.LoadSigner(repo, keyPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.InitializeRoot(t.Context(), signer, false, rootopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}
		newKey, err := gittuf.LoadPublicKey(keyPath + ".pub")
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.AddTopLevelTargetsKey(t.Context(), signer, newKey, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}
		if err := repo.InitializeTargets(t.Context(), signer, policy.TargetsRoleName, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		// Run updaterule for rule with gittuf prefix
		command := New(&persistent.Options{SigningKey: keyPath, WithRSLEntry: true})
		_, _, _, err = cmd.ExecuteCommandC(command, "--rule-name", "gittuf-some-rule", "--authorize", newKey.ID(), "--rule-pattern", "git:refs/heads/main")
		assert.ErrorContains(t, err, "cannot add or change rules whose names have the 'gittuf-' prefix")
	})

	t.Run("principal not found", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		if err := os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600); err != nil {
			t.Fatal(err)
		}

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = os.Chdir(cwd)
		}()

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}

		repo, err := gittuf.LoadRepository(".")
		if err != nil {
			t.Fatal(err)
		}
		signer, err := gittuf.LoadSigner(repo, keyPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.InitializeRoot(t.Context(), signer, false, rootopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}
		newKey, err := gittuf.LoadPublicKey(keyPath + ".pub")
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.AddTopLevelTargetsKey(t.Context(), signer, newKey, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}
		if err := repo.InitializeTargets(t.Context(), signer, policy.TargetsRoleName, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}
		if err := repo.AddPrincipalToTargets(t.Context(), signer, policy.TargetsRoleName, []tuf.Principal{newKey}, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}
		if err := repo.AddDelegation(t.Context(), signer, policy.TargetsRoleName, "rule-1", []string{newKey.ID()}, []string{"git:refs/heads/main"}, 1, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		command := New(&persistent.Options{SigningKey: keyPath, WithRSLEntry: true})
		_, _, _, err = cmd.ExecuteCommandC(command, "--rule-name", "rule-1", "--authorize", "non-existent-principal", "--rule-pattern", "git:refs/heads/main")
		assert.ErrorContains(t, err, "principal not found")
	})

	t.Run("invalid threshold", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		if err := os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600); err != nil {
			t.Fatal(err)
		}

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = os.Chdir(cwd)
		}()

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}

		repo, err := gittuf.LoadRepository(".")
		if err != nil {
			t.Fatal(err)
		}
		signer, err := gittuf.LoadSigner(repo, keyPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.InitializeRoot(t.Context(), signer, false, rootopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}
		newKey, err := gittuf.LoadPublicKey(keyPath + ".pub")
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.AddTopLevelTargetsKey(t.Context(), signer, newKey, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}
		if err := repo.InitializeTargets(t.Context(), signer, policy.TargetsRoleName, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}
		if err := repo.AddPrincipalToTargets(t.Context(), signer, policy.TargetsRoleName, []tuf.Principal{newKey}, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}
		if err := repo.AddDelegation(t.Context(), signer, policy.TargetsRoleName, "rule-1", []string{newKey.ID()}, []string{"git:refs/heads/main"}, 1, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		command := New(&persistent.Options{SigningKey: keyPath, WithRSLEntry: true})
		_, _, _, err = cmd.ExecuteCommandC(command, "--rule-name", "rule-1", "--authorize", newKey.ID(), "--rule-pattern", "git:refs/heads/main", "--threshold", "0")
		assert.ErrorContains(t, err, "threshold must be a positive integer")
	})

	t.Run("cannot meet threshold", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		if err := os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600); err != nil {
			t.Fatal(err)
		}

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = os.Chdir(cwd)
		}()

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}

		repo, err := gittuf.LoadRepository(".")
		if err != nil {
			t.Fatal(err)
		}
		signer, err := gittuf.LoadSigner(repo, keyPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.InitializeRoot(t.Context(), signer, false, rootopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}
		newKey, err := gittuf.LoadPublicKey(keyPath + ".pub")
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.AddTopLevelTargetsKey(t.Context(), signer, newKey, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}
		if err := repo.InitializeTargets(t.Context(), signer, policy.TargetsRoleName, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}
		if err := repo.AddPrincipalToTargets(t.Context(), signer, policy.TargetsRoleName, []tuf.Principal{newKey}, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}
		if err := repo.AddDelegation(t.Context(), signer, policy.TargetsRoleName, "rule-1", []string{newKey.ID()}, []string{"git:refs/heads/main"}, 1, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		command := New(&persistent.Options{SigningKey: keyPath, WithRSLEntry: true})
		_, _, _, err = cmd.ExecuteCommandC(command, "--rule-name", "rule-1", "--authorize", newKey.ID(), "--rule-pattern", "git:refs/heads/main", "--threshold", "2")
		assert.ErrorContains(t, err, "insufficient keys to meet threshold")
	})

	t.Run("invalid policy name (root)", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		if err := os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600); err != nil {
			t.Fatal(err)
		}

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = os.Chdir(cwd)
		}()

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}

		repo, err := gittuf.LoadRepository(".")
		if err != nil {
			t.Fatal(err)
		}
		_, err = gittuf.LoadSigner(repo, keyPath)
		if err != nil {
			t.Fatal(err)
		}

		command := New(&persistent.Options{SigningKey: keyPath})
		_, _, _, err = cmd.ExecuteCommandC(command, "--rule-name", "root", "--authorize", "principal-1", "--rule-pattern", "git:refs/heads/main")
		assert.ErrorIs(t, err, gittuf.ErrInvalidPolicyName)
	})

	t.Run("missing required rule-name flag", func(t *testing.T) {
		command := New(&persistent.Options{SigningKey: "key"})
		_, _, _, err := cmd.ExecuteCommandC(command, "--authorize", "principal-1", "--rule-pattern", "git:refs/heads/main")
		assert.ErrorContains(t, err, "required flag(s) \"rule-name\" not set")
	})

	t.Run("missing required rule-pattern flag", func(t *testing.T) {
		command := New(&persistent.Options{SigningKey: "key"})
		_, _, _, err := cmd.ExecuteCommandC(command, "--rule-name", "rule-1", "--authorize", "principal-1")
		assert.ErrorContains(t, err, "required flag(s) \"rule-pattern\" not set")
	})

	t.Run("missing required authorize / authorize-key flags", func(t *testing.T) {
		command := New(&persistent.Options{SigningKey: "key"})
		_, _, _, err := cmd.ExecuteCommandC(command, "--rule-name", "rule-1", "--rule-pattern", "git:refs/heads/main")
		assert.ErrorContains(t, err, "at least one of the flags in the group [authorize authorize-key] is required")
	})

	t.Run("missing authorize-key file", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		if err := os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600); err != nil {
			t.Fatal(err)
		}

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = os.Chdir(cwd)
		}()

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}

		repo, err := gittuf.LoadRepository(".")
		if err != nil {
			t.Fatal(err)
		}
		_, err = gittuf.LoadSigner(repo, keyPath)
		if err != nil {
			t.Fatal(err)
		}

		command := New(&persistent.Options{SigningKey: keyPath})
		_, _, _, err = cmd.ExecuteCommandC(command, "--rule-name", "rule-1", "--authorize-key", "non-existent-key.pub", "--rule-pattern", "git:refs/heads/main")
		assert.ErrorContains(t, err, "No such file or directory")
	})
}
