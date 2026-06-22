// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package updateperson

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/experimental/gittuf"
	rootopts "github.com/gittuf/gittuf/experimental/gittuf/options/root"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/cmd"
	addpersoncmd "github.com/gittuf/gittuf/internal/cmd/policy/addperson"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/gittuf/gittuf/internal/policy"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestUpdatePerson(t *testing.T) {
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

		_, _, _, err = cmd.ExecuteCommandC(New(&persistent.Options{}), "--person-ID", "person-1")
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

		// First, add the person using add-person command
		addPersonCmd := addpersoncmd.New(&persistent.Options{SigningKey: keyPath, WithRSLEntry: true})
		_, _, _, err = cmd.ExecuteCommandC(addPersonCmd, "--person-ID", "person-1", "--public-key", keyPath+".pub", "--associated-identity", "github::user1", "--custom", "email=user1@test.com")
		assert.NoError(t, err)

		// Verify it was added
		state, err := policy.LoadCurrentState(t.Context(), repo.GetGitRepository(), policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}
		targetsMetadata, err := state.GetTargetsMetadata(policy.TargetsRoleName, false)
		assert.Nil(t, err)
		principals := targetsMetadata.GetPrincipals()
		assert.Contains(t, principals, "person-1")
		person := principals["person-1"].(*tufv02.Person)
		assert.Equal(t, "user1@test.com", person.Custom["email"])
		assert.Equal(t, "user1", person.AssociatedIdentities["github"])

		// Now update person
		updatePersonCmd := New(&persistent.Options{SigningKey: keyPath, WithRSLEntry: true})
		_, _, _, err = cmd.ExecuteCommandC(updatePersonCmd, "--person-ID", "person-1", "--public-key", keyPath+".pub", "--associated-identity", "github::user1-new", "--custom", "email=user1-new@test.com")
		assert.NoError(t, err)

		// Verify updated fields
		state, err = policy.LoadCurrentState(t.Context(), repo.GetGitRepository(), policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}
		targetsMetadata, err = state.GetTargetsMetadata(policy.TargetsRoleName, false)
		assert.Nil(t, err)
		principals = targetsMetadata.GetPrincipals()
		person = principals["person-1"].(*tufv02.Person)
		assert.Equal(t, "user1-new@test.com", person.Custom["email"])
		assert.Equal(t, "user1-new", person.AssociatedIdentities["github"])
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

		// First, add the person using add-person command
		addPersonCmd := addpersoncmd.New(&persistent.Options{SigningKey: keyPath, WithRSLEntry: true})
		_, _, _, err = cmd.ExecuteCommandC(addPersonCmd, "--policy-name", "custom-policy", "--person-ID", "person-1", "--public-key", keyPath+".pub", "--associated-identity", "github::user1", "--custom", "email=user1@test.com")
		assert.NoError(t, err)

		// Now update person
		updatePersonCmd := New(&persistent.Options{SigningKey: keyPath, WithRSLEntry: true})
		_, _, _, err = cmd.ExecuteCommandC(updatePersonCmd, "--policy-name", "custom-policy", "--person-ID", "person-1", "--public-key", keyPath+".pub", "--associated-identity", "github::user1-new", "--custom", "email=user1-new@test.com")
		assert.NoError(t, err)

		// Verify updated fields
		state, err := policy.LoadCurrentState(t.Context(), repo.GetGitRepository(), policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}
		targetsMetadata, err := state.GetTargetsMetadata("custom-policy", false)
		assert.Nil(t, err)
		principals := targetsMetadata.GetPrincipals()
		person := principals["person-1"].(*tufv02.Person)
		assert.Equal(t, "user1-new@test.com", person.Custom["email"])
		assert.Equal(t, "user1-new", person.AssociatedIdentities["github"])
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
		_, _, _, err = cmd.ExecuteCommandC(command, "--person-ID", "person-1")
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
		_, _, _, err = cmd.ExecuteCommandC(command, "--policy-name", "non-existent-policy", "--person-ID", "person-1")
		assert.ErrorIs(t, err, policy.ErrMetadataNotFound)
	})

	t.Run("invalid format for associated identity", func(t *testing.T) {
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

		command := New(&persistent.Options{SigningKey: keyPath})
		_, _, _, err = cmd.ExecuteCommandC(command, "--person-ID", "person-1", "--associated-identity", "invalididentity")
		assert.ErrorContains(t, err, "invalid format for associated identity")
	})

	t.Run("invalid format for custom metadata", func(t *testing.T) {
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

		command := New(&persistent.Options{SigningKey: keyPath})
		_, _, _, err = cmd.ExecuteCommandC(command, "--person-ID", "person-1", "--custom", "invalidcustom")
		assert.ErrorContains(t, err, "invalid format for custom metadata")
	})

	t.Run("missing public key file", func(t *testing.T) {
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

		command := New(&persistent.Options{SigningKey: keyPath})
		_, _, _, err = cmd.ExecuteCommandC(command, "--person-ID", "person-1", "--public-key", "non-existent-key.pub")
		assert.ErrorContains(t, err, "No such file or directory")
	})

	t.Run("person not found", func(t *testing.T) {
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

		command := New(&persistent.Options{SigningKey: keyPath, WithRSLEntry: true})
		_, _, _, err = cmd.ExecuteCommandC(command, "--person-ID", "non-existent-person")
		assert.ErrorContains(t, err, "principal not found")
	})
}
