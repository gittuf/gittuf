// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type helpScreen struct {
	previousScreen screen
}

func (s *helpScreen) Update(msg tea.Msg, m *model) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "h", "esc":
			m.screen = s.previousScreen
			return *m, nil
		}
	}
	return *m, nil
}

func (s *helpScreen) View(m *model) string {
	h, v := lipgloss.NewStyle().Margin(1, 2).GetFrameSize()
	boxWidth := m.width - h - 2
	boxHeight := m.height - v - 8
	if boxWidth < 0 {
		boxWidth = 0
	}
	if boxHeight < 0 {
		boxHeight = 0
	}

	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorFocus)).Render("Help")
	helpContent := title + "\n\n" +
		"- ↑ ↓ : Navigate\n" +
		"- Enter : Select\n" +
		"- a : Add rule\n" +
		"- e : Edit\n" +
		"- d : Delete\n" +
		"- esc : Back\n" +
		"- q : Quit\n"

	centeredContent := lipgloss.Place(boxWidth, boxHeight, lipgloss.Center, lipgloss.Center, helpContent)
	return m.renderScreen("Help", centeredContent, "")
}
