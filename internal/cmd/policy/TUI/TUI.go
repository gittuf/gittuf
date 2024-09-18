// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/spf13/cobra"
)

type model struct {
	options []string
	cursor  int
	choice  string
}

func (m model) Init() tea.Cmd {
	return nil
}

func initialModel() model {
	return model{
		options: []string{"Add Rule", "Remove Rule", "List Rules"},
		cursor:  0,
		choice:  "",
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch msg.String() {

		case "ctrl+c", "q":
			return m, tea.Quit

		case "up":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}

		case "enter":
			m.choice = m.options[m.cursor]
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m model) View() string {
	var s string

	if m.choice == "" {
		s = "gittuf policy operations: \n\n"

		for i, option := range m.options {
			if m.cursor == i {
				s += fmt.Sprintf("> %s\n", option)
			} else {
				s += fmt.Sprintf("  %s\n", option)
			}
		}
	} else {
		s = ""
		switch m.choice {
		case "Add Rule":
			s += "adding a rule..."
		case "Remove Rule":
			s += "removing a rule..."
		case "List Rules":
			s += "listing rules..."
		default:
			s += "Not implemented yet."
		}
	}

	s += "\nPress q to quit.\n"
	return s
}

func startTUI() error {
	p := tea.NewProgram(initialModel())
	_, err := p.Run()
	return err
}

type options struct {
	p *persistent.Options
}

func (o *options) AddFlags(cmd *cobra.Command) {
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	return startTUI()
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "TUI",
		Short:             "Start the TUI for managing policies",
		PreRunE:           common.CheckIfSigningViableWithFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
