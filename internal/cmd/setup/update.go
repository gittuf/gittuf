// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

// This file handles TUI input handling.

package setup

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Update updates the model based on the message received
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		h, v := lipgloss.NewStyle().Margin(1, 2).GetFrameSize()
		m.choiceList.SetSize(msg.Width-h, msg.Height-v)

	case transportDoneMsg:
		m.transportRunning = false
		if m.screen != screenTransport {
			return m, nil // user escapes screen before done
		}
		if msg.err != nil {
			m.errorMsg = fmt.Sprintf("error: %v", msg.err)
			return m, nil
		}
		m.transportSteps = msg.steps
		m.footer = "Setup complete!" + "\n" + "Please see https://gittuf.dev/ for further documentation."
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			m.footer = ""
			switch m.screen {
			case screenRoot, screenTransport:
				m.screen = screenChoice
			}
			return m, nil
		case "enter":
			switch m.screen {
			case screenChoice:
				i, ok := m.choiceList.SelectedItem().(item)
				if ok {
					switch i.title {
					case "Maintainer":
						m.rootExists = checkRootExists(m.repo)
						m.screen = screenRoot
					case "Contributor":
						exists, err := checkTransportExists(m.repo)
						m.transportExists = exists
						if err != nil {
							m.errorMsg = err.Error()
						}
						if !m.transportExists {
							m.transportRunning = true
							m.screen = screenTransport
							return m, tea.Batch(runTransportSetup(m.repo), m.spinner.Tick)
						}
						m.screen = screenAbort
					}
				}
			case screenTransport:
				if !m.transportRunning {
					return m, tea.Quit
				}

			case screenAbort, screenConclusion:
				return m, tea.Quit
			}
		case "tab", "shift+tab", "up", "down":
		}
	}

	switch m.screen {
	case screenChoice:
		m.choiceList, cmd = m.choiceList.Update(msg)
	case screenTransport:
		if m.transportRunning {
			m.spinner, cmd = m.spinner.Update(msg)
		}
	}
	return m, cmd
}
