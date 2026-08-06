// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type trustScreen struct {
	trustScreenList list.Model
}

func (s *trustScreen) Update(msg tea.Msg, m *model) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	if msg, ok := msg.(tea.KeyMsg); ok {
		if msg.String() == "enter" {
			if sel, ok := s.trustScreenList.SelectedItem().(item); ok {
				switch sel.title {
				case "View Global Rules":
					m.screen = screenTrustGlobalRules
					m.trustGlobalRulesScreen.refreshGlobalRules(m.ctx, m.options)
				case "Keys & Thresholds":
					m.screen = screenTrustKeysThresholds
				case "Propagation":
					m.screen = screenTrustPropagation
				case "GitHub App":
					m.screen = screenTrustGitHubApp
				case "Lifecycle":
					m.screen = screenTrustLifecycle
				case "Repo/Network":
					m.screen = screenTrustRepoNetwork
				}
			}
			return *m, nil
		}
	}
	s.trustScreenList, cmd = s.trustScreenList.Update(msg)
	return *m, cmd
}

func (s *trustScreen) View(m *model) string {
	return m.renderScreen("Home › Trust", s.trustScreenList.View(), renderActionHints(m.readOnly))
}
