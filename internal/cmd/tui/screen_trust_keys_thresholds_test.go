// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/gittuf/gittuf/internal/policy"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrustKeysListKeys(t *testing.T) {
	setup := func(t *testing.T) (string, *gittuf.Repository) {
		t.Helper()

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
		require.NoError(t, repo.InitializeRoot(t.Context(), signer, false))
		require.NoError(t, repo.StagePolicy(t.Context(), "", true, false))
		require.NoError(t, repo.ApplyPolicy(t.Context(), "", true, false))

		return keyPath, repo
	}

	newTestModel := func(keyPath string) model {
		o := &options{
			targetRef: "policy",
			p:         &persistent.Options{SigningKey: keyPath},
		}
		m := initialModel(context.Background(), o)
		m.screen = screenTrustKeysThresholds
		return m
	}

	t.Run("lists root keys without top-level policy keys", func(t *testing.T) {
		keyPath, _ := setup(t)
		m := newTestModel(keyPath)
		s := &m.trustKeysScreen

		selectItemByTitle(t, &s.operationList, "List Root Keys")
		updatedModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
		resModel := updatedModel.(model)

		require.Nil(t, resModel.errorDialog)
		assert.True(t, resModel.trustKeysScreen.showKeys)
		require.NotNil(t, resModel.trustKeysScreen.keys)
		assert.Len(t, resModel.trustKeysScreen.keys.rootKeys, 1)
		assert.Equal(t, 1, resModel.trustKeysScreen.keys.rootThreshold)
		assert.Empty(t, resModel.trustKeysScreen.keys.policyKeys)

		viewStr := resModel.trustKeysScreen.View(&resModel)
		assert.Contains(t, viewStr, "Home › Trust › Keys & Thresholds › List Root Keys")
		assert.Contains(t, viewStr, "Root Keys (threshold: 1)")
		assert.Contains(t, viewStr, resModel.trustKeysScreen.keys.rootKeys[0].ID())
		assert.NotContains(t, viewStr, "Top-Level Policy Keys")
	})

	t.Run("lists policy keys when none are configured", func(t *testing.T) {
		keyPath, _ := setup(t)
		m := newTestModel(keyPath)
		s := &m.trustKeysScreen

		selectItemByTitle(t, &s.operationList, "List Policy Keys")
		updatedModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
		resModel := updatedModel.(model)

		require.Nil(t, resModel.errorDialog)
		viewStr := resModel.trustKeysScreen.View(&resModel)
		assert.Contains(t, viewStr, "Top-Level Policy Keys (threshold: not configured)")
		assert.Contains(t, viewStr, "None configured.")
		assert.NotContains(t, viewStr, "Root Keys")
	})

	t.Run("lists root and top-level policy keys", func(t *testing.T) {
		keyPath, repo := setup(t)

		signer, err := gittuf.LoadSigner(repo, keyPath)
		require.NoError(t, err)
		policyKey, err := gittuf.LoadPublicKey(repo, keyPath+".pub")
		require.NoError(t, err)
		require.NoError(t, repo.AddTopLevelTargetsKey(t.Context(), signer, policyKey, false))
		require.NoError(t, repo.StagePolicy(t.Context(), "", true, false))
		require.NoError(t, repo.ApplyPolicy(t.Context(), "", true, false))

		m := newTestModel(keyPath)
		s := &m.trustKeysScreen

		selectItemByTitle(t, &s.operationList, "List Policy Keys")
		updatedModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
		resModel := updatedModel.(model)

		require.Nil(t, resModel.errorDialog)
		require.NotNil(t, resModel.trustKeysScreen.keys)
		require.Len(t, resModel.trustKeysScreen.keys.policyKeys, 1)
		assert.Equal(t, policyKey.ID(), resModel.trustKeysScreen.keys.policyKeys[0].ID())

		viewStr := resModel.trustKeysScreen.View(&resModel)
		assert.Contains(t, viewStr, "Home › Trust › Keys & Thresholds › List Policy Keys")
		assert.Contains(t, viewStr, "Top-Level Policy Keys (threshold: 1)")
		assert.Equal(t, 1, strings.Count(viewStr, policyKey.ID()))
		assert.NotContains(t, viewStr, "Root Keys")
	})

	t.Run("lists all unstaged policy keys with policy-staging target ref", func(t *testing.T) {
		keyPath, repo := setup(t)

		signer, err := gittuf.LoadSigner(repo, keyPath)
		require.NoError(t, err)

		firstKeyPath := filepath.Join(t.TempDir(), "first.pub")
		require.NoError(t, os.WriteFile(firstKeyPath, artifacts.SSHECDSAPublicSSH, 0o600))
		firstKey, err := gittuf.LoadPublicKey(repo, firstKeyPath)
		require.NoError(t, err)

		secondKeyPath := filepath.Join(t.TempDir(), "second.pub")
		require.NoError(t, os.WriteFile(secondKeyPath, artifacts.SSHRSAPublicSSH, 0o600))
		secondKey, err := gittuf.LoadPublicKey(repo, secondKeyPath)
		require.NoError(t, err)

		// Add keys the way the TUI does, without staging or applying
		require.NoError(t, repo.AddTopLevelTargetsKey(t.Context(), signer, firstKey, false))
		require.NoError(t, repo.AddTopLevelTargetsKey(t.Context(), signer, secondKey, false))

		m := newTestModel(keyPath)
		m.options.targetRef = "policy-staging"
		s := &m.trustKeysScreen

		selectItemByTitle(t, &s.operationList, "List Policy Keys")
		updatedModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
		resModel := updatedModel.(model)

		require.Nil(t, resModel.errorDialog)
		require.NotNil(t, resModel.trustKeysScreen.keys)
		require.Len(t, resModel.trustKeysScreen.keys.policyKeys, 2)

		viewStr := resModel.trustKeysScreen.View(&resModel)
		assert.Contains(t, viewStr, "Top-Level Policy Keys (threshold: 1)")
		assert.Contains(t, viewStr, firstKey.ID())
		assert.Contains(t, viewStr, secondKey.ID())
		assert.NotContains(t, viewStr, "threshold: not configured")
	})

	t.Run("lists principal keys from the top-level policy", func(t *testing.T) {
		keyPath, repo := setup(t)

		signer, err := gittuf.LoadSigner(repo, keyPath)
		require.NoError(t, err)
		policyKey, err := gittuf.LoadPublicKey(repo, keyPath+".pub")
		require.NoError(t, err)
		require.NoError(t, repo.AddTopLevelTargetsKey(t.Context(), signer, policyKey, false))
		require.NoError(t, repo.InitializeTargets(t.Context(), signer, policy.TargetsRoleName, false))

		firstKeyPath := filepath.Join(t.TempDir(), "first.pub")
		require.NoError(t, os.WriteFile(firstKeyPath, artifacts.SSHECDSAPublicSSH, 0o600))
		firstKey, err := gittuf.LoadPublicKey(repo, firstKeyPath)
		require.NoError(t, err)

		secondKeyPath := filepath.Join(t.TempDir(), "second.pub")
		require.NoError(t, os.WriteFile(secondKeyPath, artifacts.SSHRSAPublicSSH, 0o600))
		secondKey, err := gittuf.LoadPublicKey(repo, secondKeyPath)
		require.NoError(t, err)

		require.NoError(t, repo.AddPrincipalToTargets(t.Context(), signer, policy.TargetsRoleName, []tuf.Principal{firstKey, secondKey}, false))
		require.NoError(t, repo.StagePolicy(t.Context(), "", true, false))
		require.NoError(t, repo.ApplyPolicy(t.Context(), "", true, false))

		m := newTestModel(keyPath)
		s := &m.trustKeysScreen

		selectItemByTitle(t, &s.operationList, "List Policy Keys")
		updatedModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
		resModel := updatedModel.(model)

		require.Nil(t, resModel.errorDialog)
		require.NotNil(t, resModel.trustKeysScreen.keys)
		require.Len(t, resModel.trustKeysScreen.keys.policyKeys, 1)
		require.Len(t, resModel.trustKeysScreen.keys.policyPrincipals, 2)

		viewStr := resModel.trustKeysScreen.View(&resModel)
		assert.Contains(t, viewStr, "Top-Level Policy Keys (threshold: 1)")
		assert.Contains(t, viewStr, policyKey.ID())
		assert.Contains(t, viewStr, "Top-Level Policy Principal Keys")
		// Each principal is listed once, followed by its key
		assert.Equal(t, 2, strings.Count(viewStr, firstKey.ID()))
		assert.Equal(t, 2, strings.Count(viewStr, secondKey.ID()))
		assert.NotContains(t, viewStr, "None configured.")
	})

	t.Run("esc closes key list before leaving screen", func(t *testing.T) {
		keyPath, _ := setup(t)
		m := newTestModel(keyPath)
		m.trustKeysScreen.showKeys = true
		m.trustKeysScreen.keys = &trustKeys{}

		updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		resModel := updatedModel.(model)
		assert.Equal(t, screenTrustKeysThresholds, resModel.screen)
		assert.False(t, resModel.trustKeysScreen.showKeys)

		updatedModel, _ = resModel.Update(tea.KeyMsg{Type: tea.KeyEsc})
		resModel = updatedModel.(model)
		assert.Equal(t, screenTrust, resModel.screen)
	})

	t.Run("shows error dialog when policy is missing", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Chdir(tmpDir)
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		m := newTestModel("")
		s := &m.trustKeysScreen

		selectItemByTitle(t, &s.operationList, "List Root Keys")
		updatedModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
		resModel := updatedModel.(model)

		require.NotNil(t, resModel.errorDialog)
		assert.Equal(t, "List Root Keys Failed", resModel.errorDialog.title)
		assert.False(t, resModel.trustKeysScreen.showKeys)
	})
}

func TestTrustKeysMenuSeparatesRootAndPolicyKeys(t *testing.T) {
	items := trustKeysMenuItems(false)
	titles := make([]string, 0, len(items))
	descriptions := make([]string, 0, len(items))
	for _, menuItem := range items {
		item := menuItem.(item)
		titles = append(titles, item.title)
		descriptions = append(descriptions, item.desc)
	}

	assert.Contains(t, titles, "List Root Keys")
	assert.Contains(t, titles, "List Policy Keys")
	assert.Contains(t, strings.Join(descriptions, "\n"), "root keys and their signature threshold")
	assert.Contains(t, strings.Join(descriptions, "\n"), "policy keys and their signature threshold")
	assert.NotContains(t, titles, "List Keys")
}
