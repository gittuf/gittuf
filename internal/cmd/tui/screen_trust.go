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
			if _, ok := s.trustScreenList.SelectedItem().(item); ok {
				m.screen = screenTrustGlobalRules
				m.refreshGlobalRules()
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
