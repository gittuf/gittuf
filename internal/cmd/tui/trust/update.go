package trust

import (
	tea "github.com/charmbracelet/bubbletea"
)

type UpdatedTrustMsg struct {
	Threshold int
	RepoPath  string
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "e":
			m.IsEditing = !m.IsEditing
			if m.IsEditing {
				return m, m.RepoPath.Focus()
			}
			m.RepoPath.Blur()
			// Return a message to notify the parent of the save
			return m, func() tea.Msg {
				return UpdatedTrustMsg{Threshold: m.Threshold, RepoPath: m.RepoPath.Value()}
			}

		case "up", "down":
			if !m.IsEditing {
				if msg.String() == "up" {
					m.Threshold++
				}
				if msg.String() == "down" && m.Threshold > 1 {
					m.Threshold--
				}
			}
		}
	}

	if m.IsEditing {
		m.RepoPath, cmd = m.RepoPath.Update(msg)
	}
	return m, cmd
}
