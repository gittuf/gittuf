// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type homeScreen struct {
	choiceList list.Model
}

func (s *homeScreen) Update(msg tea.Msg, m *model) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.String() == "enter" {
			if sel, ok := s.choiceList.SelectedItem().(item); ok {
				switch sel.title {
				case "Policy":
					m.screen = screenPolicy
				case "Trust":
					m.screen = screenTrust
				}
			}
			return *m, nil
		}
	}

	s.choiceList, cmd = s.choiceList.Update(msg)
	return *m, cmd
}

func (s *homeScreen) View(m *model) string {
	return m.renderScreen("Home", s.choiceList.View(), renderActionHints(m.readOnly))
}
