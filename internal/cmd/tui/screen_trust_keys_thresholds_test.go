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
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
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

		selectItemByTitle(t, &s.operationList, "List Keys")
		updatedModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
		resModel := updatedModel.(model)

		require.Nil(t, resModel.errorDialog)
		assert.True(t, resModel.trustKeysScreen.showKeys)
		require.NotNil(t, resModel.trustKeysScreen.keys)
		assert.Len(t, resModel.trustKeysScreen.keys.rootKeys, 1)
		assert.Equal(t, 1, resModel.trustKeysScreen.keys.rootThreshold)
		assert.Empty(t, resModel.trustKeysScreen.keys.policyKeys)

		viewStr := resModel.trustKeysScreen.View(&resModel)
		assert.Contains(t, viewStr, "Home › Trust › Keys & Thresholds › List Keys")
		assert.Contains(t, viewStr, "Root Keys (threshold: 1)")
		assert.Contains(t, viewStr, resModel.trustKeysScreen.keys.rootKeys[0].ID())
		assert.Contains(t, viewStr, "Top-Level Policy Keys")
		assert.Contains(t, viewStr, "None configured.")
	})

	t.Run("lists root and top-level policy keys", func(t *testing.T) {
		keyPath, repo := setup(t)

		signer, err := gittuf.LoadSigner(repo, keyPath)
		require.NoError(t, err)
		policyKey, err := gittuf.LoadPublicKey(keyPath + ".pub")
		require.NoError(t, err)
		require.NoError(t, repo.AddTopLevelTargetsKey(t.Context(), signer, policyKey, false))
		require.NoError(t, repo.StagePolicy(t.Context(), "", true, false))
		require.NoError(t, repo.ApplyPolicy(t.Context(), "", true, false))

		m := newTestModel(keyPath)
		s := &m.trustKeysScreen

		selectItemByTitle(t, &s.operationList, "List Keys")
		updatedModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
		resModel := updatedModel.(model)

		require.Nil(t, resModel.errorDialog)
		require.NotNil(t, resModel.trustKeysScreen.keys)
		require.Len(t, resModel.trustKeysScreen.keys.policyKeys, 1)
		assert.Equal(t, policyKey.ID(), resModel.trustKeysScreen.keys.policyKeys[0].ID())

		viewStr := resModel.trustKeysScreen.View(&resModel)
		assert.Contains(t, viewStr, "Top-Level Policy Keys (threshold: 1)")
		assert.Equal(t, 2, strings.Count(viewStr, policyKey.ID()))
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

		selectItemByTitle(t, &s.operationList, "List Keys")
		updatedModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
		resModel := updatedModel.(model)

		require.NotNil(t, resModel.errorDialog)
		assert.Equal(t, "List Keys Failed", resModel.errorDialog.title)
		assert.False(t, resModel.trustKeysScreen.showKeys)
	})
}
