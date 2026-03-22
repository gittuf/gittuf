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
	case metadataDoneMsg:
		if msg.err != nil {
			m.errorMsg = fmt.Sprintf("error: %v", msg.err)
			return m, nil
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			m.footer = ""
			switch m.screen {
			case screenMaintainerSelections, screenTransport:
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
						m.screen = screenMaintainerSelections
						return m, nil
					case "Contributor":
						exists, err := checkTransportExists(m.repo)
						m.transportExists = exists
						if err != nil {
							m.errorMsg = err.Error()
						}
						if !m.transportExists {
							m.screen = screenTransportConfirm
							return m, nil
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

		// root screen-specific input handling
		switch m.screen {
		case screenTransportConfirm:
			switch msg.String() {
			case "y", "Y":
				m.transportRunning = true
				m.screen = screenTransport
				return m, tea.Batch(runTransportSetup(m.repo), m.spinner.Tick)
			case "n", "N":
				m.screen = screenConclusion
				return m, nil
			}

		case screenMaintainerSelections:
			switch msg.String() {
			case "up", "k":
				if m.rootCursor > 0 {
					m.rootCursor--
				}
			case "down", "j":
				if m.rootCursor < len(m.rootChoices) {
					m.rootCursor++
				}
			case "space", "enter":
				if m.rootCursor == len(m.rootChoices) {
					// submit response
					m.screen = screenTransportConfirm
					return m, tea.Batch(setupMaintainerChoices(m.ctx, m.repo, m.signer, m.rootSelected, m.rootExists))
				}
				_, ok := m.rootSelected[m.rootCursor]
				if ok {
					if m.rootCursor != 0 || m.rootExists {
						// "Add key to root" cannot be deselected when gittuf has not been initialized
						delete(m.rootSelected, m.rootCursor)
					}
				} else {
					m.rootSelected[m.rootCursor] = false
				}
			}
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
