// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestRepoNetworkRetainsRepositoryActions(t *testing.T) {
	m := initialModel(t.Context(), &options{readOnly: false, targetRef: "policy"})
	items := m.trustRepoNetworkScreen.operationList.Items()
	labels := make([]string, 0, len(items))
	for _, entry := range items {
		labels = append(labels, entry.(item).title)
	}

	assert.Equal(t, []string{
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
	assert.Empty(t, readOnlyItems)
}
