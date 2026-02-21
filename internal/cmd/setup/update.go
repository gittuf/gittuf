// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

// This file handles TUI input handling.

package setup

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Update updates the model based on the message received
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := lipgloss.NewStyle().Margin(1, 2).GetFrameSize()
		m.choiceList.SetSize(msg.Width-h, msg.Height-v)
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "left":
			m.footer = ""
			switch m.screen {
			}
			return m, nil
		case "enter":
			switch m.screen {
			case screenChoice:
				i, ok := m.choiceList.SelectedItem().(item)
				if ok {
					switch i.title {
					case "Maintainer":
						m.screen = screenRoot
					case "Contributer":
						m.screen = screenTransport
					}
				}
			}
		case "tab", "shift+tab", "up", "down":
		}
	}

	switch m.screen {
	case screenChoice:
		m.choiceList, cmd = m.choiceList.Update(msg)
	}
	return m, cmd
}
