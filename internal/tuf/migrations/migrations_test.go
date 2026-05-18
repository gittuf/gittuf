// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package migrations

import (
	"testing"
	"time"

	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	"github.com/stretchr/testify/assert"
)

var (
	rootPubKeyBytes    = artifacts.SSHRSAPublicSSH
	targetsPubKeyBytes = artifacts.SSHECDSAPublicSSH
)

func TestMigrateRootMetadataV01ToV02(t *testing.T) {
	key := tufv01.NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, rootPubKeyBytes))

	t.Run("test basic fields and roles", func(t *testing.T) {
		v01Root := tufv01.NewRootMetadata()
		v01Root.SetExpires(time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339))
		v01Root.SetRepositoryLocation("https://example.com/repo")

		err := v01Root.AddRootPrincipal(key)
		assert.Nil(t, err)

		v02Root := MigrateRootMetadataV01ToV02(v01Root)

		assert.Equal(t, "2030-01-01T00:00:00Z", v02Root.Expires)
		assert.Equal(t, "https://example.com/repo", v02Root.RepositoryLocation)
		assert.Contains(t, v02Root.Principals, key.KeyID)

		rootRole, hasRoot := v02Root.Roles[tuf.RootRoleName]
		assert.True(t, hasRoot)
		assert.True(t, rootRole.PrincipalIDs.Has(key.KeyID))
		assert.Equal(t, 1, rootRole.Threshold)
	})

	t.Run("test global rules", func(t *testing.T) {
		v01Root := tufv01.NewRootMetadata()

		globalRule := tufv01.NewGlobalRuleThreshold("test-rule", []string{"git:refs/heads/*"}, 2)
		err := v01Root.AddGlobalRule(globalRule)
		assert.Nil(t, err)

		v02Root := MigrateRootMetadataV01ToV02(v01Root)

		assert.Equal(t, 1, len(v02Root.GlobalRules))
		assert.Equal(t, "test-rule", v02Root.GlobalRules[0].GetName())
		assert.Empty(t, v02Root.Propagations)
	})

	t.Run("test github apps", func(t *testing.T) {
		v01Root := tufv01.NewRootMetadata()

		err := v01Root.AddGitHubAppPrincipal("test-app", key)
		assert.Nil(t, err)

		v02Root := MigrateRootMetadataV01ToV02(v01Root)

		assert.Contains(t, v02Root.GitHubApps, "test-app")
		assert.Equal(t, 1, v02Root.GitHubApps["test-app"].GetThreshold())
		assert.Contains(t, v02Root.GitHubApps["test-app"].GetPrincipalIDs(), key.KeyID)
		assert.Contains(t, v02Root.Principals, key.KeyID)
	})

	t.Run("test multi-repository", func(t *testing.T) {
		// nil case
		v01Root := tufv01.NewRootMetadata()
		v02Root := MigrateRootMetadataV01ToV02(v01Root)
		assert.Nil(t, v02Root.MultiRepository)

		// with controller and network repos
		v01Root = tufv01.NewRootMetadata()
		err := v01Root.AddRootPrincipal(key)
		assert.Nil(t, err)

		err = v01Root.EnableController()
		assert.Nil(t, err)

		err = v01Root.AddControllerRepository("controller-repo", "https://example.com/controller", []tuf.Principal{key})
		assert.Nil(t, err)

		err = v01Root.AddNetworkRepository("network-repo", "https://example.com/network", []tuf.Principal{key})
		assert.Nil(t, err)

		v02Root = MigrateRootMetadataV01ToV02(v01Root)

		assert.NotNil(t, v02Root.MultiRepository)
		assert.True(t, v02Root.MultiRepository.Controller)

		assert.Equal(t, 1, len(v02Root.MultiRepository.ControllerRepositories))
		assert.Equal(t, "controller-repo", v02Root.MultiRepository.ControllerRepositories[0].Name)
		assert.Equal(t, "https://example.com/controller", v02Root.MultiRepository.ControllerRepositories[0].Location)
		assert.Equal(t, 1, len(v02Root.MultiRepository.ControllerRepositories[0].InitialRootPrincipals))
		assert.Equal(t, key.KeyID, v02Root.MultiRepository.ControllerRepositories[0].InitialRootPrincipals[0].ID())

		assert.Equal(t, 1, len(v02Root.MultiRepository.NetworkRepositories))
		assert.Equal(t, "network-repo", v02Root.MultiRepository.NetworkRepositories[0].Name)
		assert.Equal(t, "https://example.com/network", v02Root.MultiRepository.NetworkRepositories[0].Location)
		assert.Equal(t, 1, len(v02Root.MultiRepository.NetworkRepositories[0].InitialRootPrincipals))
		assert.Equal(t, key.KeyID, v02Root.MultiRepository.NetworkRepositories[0].InitialRootPrincipals[0].ID())
	})

	t.Run("test hooks", func(t *testing.T) {
		v01Root := tufv01.NewRootMetadata()

		err := v01Root.AddRootPrincipal(key)
		assert.Nil(t, err)

		_, err = v01Root.AddHook([]tuf.HookStage{tuf.HookStagePreCommit}, "test-hook", []string{key.KeyID}, map[string]string{"sha256": "abc123"}, tuf.HookEnvironmentLua, 30)
		assert.Nil(t, err)

		v02Root := MigrateRootMetadataV01ToV02(v01Root)

		assert.Contains(t, v02Root.Hooks, tuf.HookStagePreCommit)
		assert.Equal(t, 1, len(v02Root.Hooks[tuf.HookStagePreCommit]))
		assert.Equal(t, "test-hook", v02Root.Hooks[tuf.HookStagePreCommit][0].ID())
	})
}

func TestMigrateTargetsMetadataV01ToV02(t *testing.T) {
	t.Run("test basic fields", func(t *testing.T) {
		v01Targets := tufv01.NewTargetsMetadata()
		v01Targets.SetExpires(time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339))

		v02Targets := MigrateTargetsMetadataV01ToV02(v01Targets)

		assert.Equal(t, "2030-01-01T00:00:00Z", v02Targets.Expires)
	})

	t.Run("test empty delegations", func(t *testing.T) {
		v01Targets := tufv01.NewTargetsMetadata()

		v02Targets := MigrateTargetsMetadataV01ToV02(v01Targets)

		assert.NotNil(t, v02Targets.Delegations)
		assert.Empty(t, v02Targets.Delegations.Principals)
		assert.Equal(t, 1, len(v02Targets.Delegations.Roles))
		assert.Equal(t, tuf.AllowRuleName, v02Targets.Delegations.Roles[0].Name)
	})

	t.Run("test delegations with keys and rules", func(t *testing.T) {
		v01Targets := tufv01.NewTargetsMetadata()

		key1 := tufv01.NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, rootPubKeyBytes))
		key2 := tufv01.NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targetsPubKeyBytes))

		err := v01Targets.AddPrincipal(key1)
		assert.Nil(t, err)
		err = v01Targets.AddPrincipal(key2)
		assert.Nil(t, err)

		err = v01Targets.AddRule("test-rule", []string{key1.KeyID, key2.KeyID}, []string{"git:refs/heads/*"}, 2)
		assert.Nil(t, err)

		v02Targets := MigrateTargetsMetadataV01ToV02(v01Targets)

		assert.NotNil(t, v02Targets.Delegations)
		assert.Equal(t, 2, len(v02Targets.Delegations.Principals))
		assert.Contains(t, v02Targets.Delegations.Principals, key1.KeyID)
		assert.Contains(t, v02Targets.Delegations.Principals, key2.KeyID)

		assert.Equal(t, 2, len(v02Targets.Delegations.Roles))

		rule := v02Targets.Delegations.Roles[0]
		assert.Equal(t, "test-rule", rule.Name)
		assert.Equal(t, []string{"git:refs/heads/*"}, rule.Paths)
		assert.True(t, rule.PrincipalIDs.Has(key1.KeyID))
		assert.True(t, rule.PrincipalIDs.Has(key2.KeyID))
		assert.Equal(t, 2, rule.Threshold)
		assert.False(t, rule.Terminating)

		assert.Equal(t, tuf.AllowRuleName, v02Targets.Delegations.Roles[1].Name)
		assert.True(t, v02Targets.Delegations.Roles[1].Terminating)
	})
}
