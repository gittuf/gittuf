// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"strings"

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

func (s *helpScreen) helpEntries(readOnly bool) [][2]string {
	entries := [][2]string{
		{"↑ ↓", "Navigate"},
	}

	switch s.previousScreen {
	case screenPolicyRules:
		if !readOnly {
			entries = append(entries,
				[2]string{"a", "Add rule"},
				[2]string{"e", "Edit rule"},
				[2]string{"d", "Delete rule"},
				[2]string{"j / k", "Reorder rules"},
			)
		}
	case screenPolicyPrincipals:
		if !readOnly {
			entries = append(entries,
				[2]string{"a", "Add principal"},
				[2]string{"e", "Edit principal"},
				[2]string{"d", "Delete principal"},
			)
		}
	case screenTrustGlobalRules:
		if !readOnly {
			entries = append(entries,
				[2]string{"a", "Add global rule"},
				[2]string{"e", "Edit global rule"},
				[2]string{"d", "Delete global rule"},
			)
		}
	case screenLoading:
		entries = append(entries, [2]string{"Enter", "Select"})
		if !readOnly {
			entries = append(entries,
				[2]string{"a", "Add"},
				[2]string{"e", "Edit"},
				[2]string{"d", "Delete"},
			)
		}
	default:
		entries = append(entries, [2]string{"Enter", "Select"})
	}

	entries = append(entries,
		[2]string{"v", "Review staged diff"},
		[2]string{"h", "Toggle help"},
		[2]string{"esc", "Back"},
		[2]string{"q", "Quit"},
	)
	return entries
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
	var b strings.Builder
	b.WriteString(title + "\n\n")
	for _, entry := range s.helpEntries(m.readOnly) {
		fmt.Fprintf(&b, "- %s : %s\n", entry[0], entry[1])
	}

	centeredContent := lipgloss.Place(boxWidth, boxHeight, lipgloss.Center, lipgloss.Center, b.String())
	return m.renderScreen("Help", centeredContent, "")
}
