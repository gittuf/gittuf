package trust

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type Model struct {
	Threshold  int
	RepoPath   textinput.Model
	Principals []string
	IsEditing  bool

	OriginalThreshold int
	OriginalRepoPath  string
}

func InitialModel(initialPath string, initialThreshold int, principals []string) Model {
	ti := textinput.New()
	ti.Placeholder = "Enter repository path..."
	ti.SetValue(initialPath)

	return Model{
		Threshold:         initialThreshold,
		RepoPath:          ti,
		Principals:        principals,
		IsEditing:         false,
		OriginalThreshold: initialThreshold,
		OriginalRepoPath:  initialPath,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}
