package trust

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	styleTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5"))
	styleHint  = lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("240"))
)

func (m Model) View() string {
	s := styleTitle.Render("Root of Trust Configuration") + "\n\n"

	s += fmt.Sprintf("Threshold: %d (Use ↑/↓ to change)\n", m.Threshold)
	s += fmt.Sprintf("Repository: %s\n\n", m.RepoPath.View())

	s += "Root Principals:\n"
	if len(m.Principals) == 0 {
		s += "  None configured\n\n"
	} else {
		for _, principalID := range m.Principals {
			s += fmt.Sprintf("  %s\n", principalID)
		}
		s += "\n"
	}

	if m.IsEditing {
		s += styleHint.Render("(Press 'e' or 's' to save)")
	} else {
		s += styleHint.Render("(Press 'e' to edit path, 's' to save changes, 'q' to quit)")
	}

	return s
}
