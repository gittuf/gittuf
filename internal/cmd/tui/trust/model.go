package trust

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type Model struct {
	Threshold int
	RepoPath  textinput.Model
	IsEditing bool
}

func InitialModel(initialPath string, initialThreshold int) Model {
	ti := textinput.New()
	ti.Placeholder = "Enter repository path..."
	ti.SetValue(initialPath)

	return Model{
		Threshold: initialThreshold,
		RepoPath:  ti,
		IsEditing: false,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}
