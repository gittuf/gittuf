// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package removerule

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

func TestRemoveRule(t *testing.T) {
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

		_, _, _, err = cmd.ExecuteCommandC(New(&persistent.Options{}), "--rule-name", "dummy-rule")
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

		if err := repo.AddDelegation(t.Context(), signer, policy.TargetsRoleName, "protect-main", []string{newKey.ID()}, []string{"git:refs/heads/main"}, 1, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		command := New(&persistent.Options{SigningKey: keyPath, WithRSLEntry: true})
		_, _, _, err = cmd.ExecuteCommandC(command, "--rule-name", "protect-main")
		assert.NoError(t, err)

		// Verification
		state, err := policy.LoadCurrentState(t.Context(), repo.GetGitRepository(), policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		targetsMetadata, err := state.GetTargetsMetadata(policy.TargetsRoleName, false)
		assert.Nil(t, err)

		rules := targetsMetadata.GetRules()
		for _, rule := range rules {
			assert.NotEqual(t, "protect-main", rule.ID())
		}
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

		if err := repo.AddDelegation(t.Context(), signer, "custom-policy", "protect-main", []string{newKey.ID()}, []string{"git:refs/heads/main"}, 1, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		command := New(&persistent.Options{SigningKey: keyPath, WithRSLEntry: true})
		_, _, _, err = cmd.ExecuteCommandC(command, "--policy-name", "custom-policy", "--rule-name", "protect-main")
		assert.NoError(t, err)

		// Verification
		state, err := policy.LoadCurrentState(t.Context(), repo.GetGitRepository(), policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		targetsMetadata, err := state.GetTargetsMetadata("custom-policy", false)
		assert.Nil(t, err)

		rules := targetsMetadata.GetRules()
		for _, rule := range rules {
			assert.NotEqual(t, "protect-main", rule.ID())
		}
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
		_, _, _, err = cmd.ExecuteCommandC(command, "--rule-name", "dummy-rule")
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

		// Run removerule for policy that is not initialized
		command := New(&persistent.Options{SigningKey: keyPath, WithRSLEntry: true})
		_, _, _, err = cmd.ExecuteCommandC(command, "--policy-name", "non-existent-policy", "--rule-name", "dummy-rule")
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

		// Run removerule for rule with gittuf prefix
		command := New(&persistent.Options{SigningKey: keyPath, WithRSLEntry: true})
		_, _, _, err = cmd.ExecuteCommandC(command, "--rule-name", "gittuf-some-rule")
		assert.ErrorContains(t, err, "cannot add or change rules whose names have the 'gittuf-' prefix")
	})

	t.Run("missing required rule-name flag", func(t *testing.T) {
		command := New(&persistent.Options{SigningKey: "key"})
		_, _, _, err := cmd.ExecuteCommandC(command)
		assert.ErrorContains(t, err, "required flag(s) \"rule-name\" not set")
	})
}
