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
	"github.com/gittuf/gittuf/internal/tuf/v02"
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
	m.screen = screenTrustRepoNetwork
	m.width = 80
	m.height = 24
	m.resizeLists()
	m.trustRepoNetworkScreen.refreshRootMetadata(&m)

	items := m.trustRepoNetworkScreen.operationList.Items()
	require.Len(t, items, 2)
	assert.Equal(t, item{title: "Schema Version", desc: v02.RootVersion}, items[0])
	assert.Equal(t, item{title: "Repository Location", desc: location}, items[1])

	view := m.trustRepoNetworkScreen.View(&m)
	assert.Contains(t, view, v02.RootVersion)
	assert.Contains(t, view, location)
}

func TestAdditionalInformationRetainsRepositoryActions(t *testing.T) {
	m := initialModel(t.Context(), &options{readOnly: false, targetRef: "policy"})
	items := m.trustRepoNetworkScreen.operationList.Items()
	labels := make([]string, 0, len(items))
	for _, entry := range items {
		labels = append(labels, entry.(item).title)
	}

	assert.Equal(t, []string{
		"Schema Version",
		"Repository Location",
		"Add Controller Repository",
		"Add Network Repository",
		"Set Repository Location",
		"Make Controller",
	}, labels)

	for _, title := range []string{"Add Controller Repository", "Add Network Repository"} {
		m.screen = screenTrustRepoNetwork
		selectItemByTitle(t, &m.trustRepoNetworkScreen.operationList, title)
		updated, _ := m.trustRepoNetworkScreen.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
		assert.Equal(t, screenTrustRepoForm, updated.(model).screen)
	}

	m.screen = screenTrustRepoNetwork
	selectItemByTitle(t, &m.trustRepoNetworkScreen.operationList, "Set Repository Location")
	updated, _ := m.trustRepoNetworkScreen.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
	assert.Equal(t, screenTrustRepoLocationForm, updated.(model).screen)

	readOnly := initialModel(t.Context(), &options{readOnly: true, targetRef: "policy"})
	readOnlyItems := readOnly.trustRepoNetworkScreen.operationList.Items()
	assert.Len(t, readOnlyItems, 2)
}

func TestAdditionalInformationMetadataRefreshPreservesActions(t *testing.T) {
	m := initialModel(t.Context(), &options{readOnly: false, targetRef: "policy"})

	// A failed load still shows the existing operations alongside the error.
	m.trustRepoNetworkScreen.setMetadataError(assert.AnError, false)
	items := m.trustRepoNetworkScreen.operationList.Items()
	assert.Len(t, items, 6)
	assert.Equal(t, assert.AnError.Error(), items[0].(item).desc)
	assert.Equal(t, "Add Controller Repository", items[2].(item).title)
}
