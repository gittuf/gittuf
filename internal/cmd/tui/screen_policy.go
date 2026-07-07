// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type policyScreen struct {
	policyScreenList list.Model
}

func (s *policyScreen) Update(msg tea.Msg, m *model) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	if msg, ok := msg.(tea.KeyMsg); ok {
		if msg.String() == "enter" {
			if _, ok := s.policyScreenList.SelectedItem().(item); ok {
				m.screen = screenPolicyRules
				m.policyRulesScreen.refreshRules(m.ctx, m.options)
			}
			return *m, nil
		}
	}
	s.policyScreenList, cmd = s.policyScreenList.Update(msg)
	return *m, cmd
}

func (s *policyScreen) View(m *model) string {
	return m.renderScreen("Home › Policy", s.policyScreenList.View(), renderActionHints(m.readOnly))
}
