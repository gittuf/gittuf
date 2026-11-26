// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gittuf/gittuf/experimental/gittuf"
	rootopts "github.com/gittuf/gittuf/experimental/gittuf/options/root"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	v02 "github.com/gittuf/gittuf/internal/tuf/v02"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdditionalInformationDisplaysRootMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	gitinterface.CreateTestGitRepository(t, tmpDir, false)

	keyPath := filepath.Join(tmpDir, "test-key")
	require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600))
	require.NoError(t, os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600))

	repo, err := gittuf.LoadRepository(tmpDir)
	require.NoError(t, err)
	signer, err := gittuf.LoadSigner(repo, keyPath)
	require.NoError(t, err)

	location := "https://example.com/project"
	require.NoError(t, repo.InitializeRoot(t.Context(), signer, false,
		rootopts.WithRepositoryLocation(location),
		rootopts.WithRSLEntry(),
	))
	require.NoError(t, repo.ApplyPolicy(t.Context(), "", true, false))

	m := initialModel(t.Context(), &options{readOnly: true, targetRef: "policy"})
	m.repo = repo
	m.readOnly = true
	m.screen = screenTrustAdditionalInformation
	m.width = 80
	m.height = 24
	m.resizeLists()
	m.screen = screenTrust
	selectItemByTitle(t, &m.trustScreen.trustScreenList, "Additional Information")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	require.Equal(t, screenTrustAdditionalInformation, m.screen)

	items := m.trustAdditionalInformationScreen.operationList.Items()
	require.Len(t, items, 2)
	assert.Equal(t, item{title: "Schema Version", desc: v02.RootVersion}, items[0])
	assert.Equal(t, item{title: "Repository Location", desc: location}, items[1])

	view := m.trustAdditionalInformationScreen.View(&m)
	assert.Contains(t, view, v02.RootVersion)
	assert.Contains(t, view, location)
}

func TestAdditionalInformationMetadataError(t *testing.T) {
	m := initialModel(t.Context(), &options{readOnly: false, targetRef: "policy"})
	m.trustAdditionalInformationScreen.setMetadataError(assert.AnError)
	items := m.trustAdditionalInformationScreen.operationList.Items()
	require.Len(t, items, 2)
	for _, entry := range items {
		assert.Equal(t, assert.AnError.Error(), entry.(item).desc)
	}
	assert.Len(t, m.trustRepoNetworkScreen.operationList.Items(), 4)
}

func TestTrustInformationAndRepoNetworkNavigation(t *testing.T) {
	for _, readOnly := range []bool{true, false} {
		m := initialModel(t.Context(), &options{readOnly: readOnly, targetRef: "policy"})
		m.width, m.height = 100, 30
		m.resizeLists()
		m.screen = screenTrust
		selectItemByTitle(t, &m.trustScreen.trustScreenList, "Additional Information")
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = updated.(model)
		require.Equal(t, screenTrustAdditionalInformation, m.screen)
		assert.Contains(t, m.View(), "Home › Trust › Additional Information")
		require.Len(t, m.trustAdditionalInformationScreen.operationList.Items(), 2)
		for _, title := range []string{"Schema Version", "Repository Location"} {
			selectItemByTitle(t, &m.trustAdditionalInformationScreen.operationList, title)
			updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			m = updated.(model)
			assert.Equal(t, screenTrustAdditionalInformation, m.screen)
		}
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		m = updated.(model)
		require.Equal(t, screenTrust, m.screen)
		if readOnly {
			continue
		}
		selectItemByTitle(t, &m.trustScreen.trustScreenList, "Repo/Network")
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = updated.(model)
		require.Equal(t, screenTrustRepoNetwork, m.screen)
		assert.Contains(t, m.View(), "Home › Trust › Repo/Network")
		assert.NotContains(t, m.View(), "Additional Information")
		selectItemByTitle(t, &m.trustRepoNetworkScreen.operationList, "Set Repository Location")
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = updated.(model)
		require.Equal(t, screenTrustRepoLocationForm, m.screen)
		assert.Contains(t, m.View(), "Home › Trust › Repo/Network › Location")
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		m = updated.(model)
		require.Equal(t, screenTrustRepoNetwork, m.screen)
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		m = updated.(model)
		assert.Equal(t, screenTrust, m.screen)
	}
}
