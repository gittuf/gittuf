// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/policy"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrustPropagationDirectivesView(t *testing.T) {
	t.Run("modal key isolation, two-stage Esc, and re-entry", func(t *testing.T) {
		m := initialModel(t.Context(), &options{readOnly: false, targetRef: "policy"})
		m.width, m.height = 100, 30
		m.resizeLists()
		m.screen = screenTrustPropagation
		m.trustPropagationScreen.showDirectives = true
		m.trustPropagationScreen.directives = []trustPropagationDirective{{name: "sync-docs"}}

		// Down and Enter must not reach the hidden operationList.
		for _, key := range []tea.KeyType{tea.KeyDown, tea.KeyEnter} {
			updated, _ := m.Update(tea.KeyMsg{Type: key})
			m = updated.(model)
			assert.Equal(t, 0, m.trustPropagationScreen.operationList.Index())
			assert.Equal(t, screenTrustPropagation, m.screen)
			assert.True(t, m.trustPropagationScreen.showDirectives)
		}

		// First Esc closes directives view and returns to Propagation menu.
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		m = updated.(model)
		assert.Equal(t, screenTrustPropagation, m.screen)
		assert.False(t, m.trustPropagationScreen.showDirectives)
		assert.Nil(t, m.trustPropagationScreen.directives)
		assert.Contains(t, m.trustPropagationScreen.View(&m), "List Directives")

		// Second Esc leaves Propagation for Trust menu.
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		m = updated.(model)
		assert.Equal(t, screenTrust, m.screen)

		// Re-entering Propagation renders the menu, not stale directives.
		m.screen = screenTrustPropagation
		view := m.trustPropagationScreen.View(&m)
		assert.Contains(t, view, "List Directives")
		assert.NotContains(t, view, "Directive:")
	})

	t.Run("Enter on List Directives refreshes from repo", func(t *testing.T) {
		t.Setenv(dev.DevModeKey, "1")
		tmpDir := t.TempDir()
		t.Chdir(tmpDir)

		gitinterface.CreateTestGitRepository(t, tmpDir, false)
		keyPath := filepath.Join(tmpDir, "root-key")
		require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600))

		repo, err := gittuf.LoadRepository(".")
		require.NoError(t, err)
		signer, err := gittuf.LoadSigner(repo, keyPath)
		require.NoError(t, err)
		require.NoError(t, repo.InitializeRoot(t.Context(), signer, false))
		require.NoError(t, repo.AddPropagationDirective(
			t.Context(), signer, "sync-docs",
			"https://example.com/upstream.git", "refs/heads/main", "docs",
			"refs/heads/main", "imported/docs", false,
		))

		m := initialModel(t.Context(), &options{readOnly: false, targetRef: policy.PolicyStagingRef})
		m.width, m.height = 100, 30
		m.resizeLists()
		m.screen = screenTrustPropagation

		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = updated.(model)
		assert.Equal(t, screenTrustPropagation, m.screen)
		assert.True(t, m.trustPropagationScreen.showDirectives)
		require.Len(t, m.trustPropagationScreen.directives, 1)
		assert.Equal(t, "sync-docs", m.trustPropagationScreen.directives[0].name)
		assert.Contains(t, m.trustPropagationScreen.View(&m), "back to menu")
	})
}
