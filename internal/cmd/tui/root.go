package tui

import (
	"github.com/gittuf/gittuf/internal/cmd/tui/trust"

	tea "github.com/charmbracelet/bubbletea"
)

type MainModel struct {
	trustSubModel trust.Model
	ready         bool
}

func (m MainModel) Init() tea.Cmd {
	return m.trustSubModel.Init()
}

func (m MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}
	case trust.UpdatedTrustMsg:
		// Handle global state updates or persist to gittuf config here
	}

	// Delegate to the trust sub-module
	var cmd tea.Cmd
	m.trustSubModel, cmd = m.trustSubModel.Update(msg)
	return m, cmd
}

func (m MainModel) View() string {
	return m.trustSubModel.View()
}
