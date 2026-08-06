// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gittuf/gittuf/internal/tuf"
)

type trustGitHubAppAction int

const (
	trustGitHubAppActionNone trustGitHubAppAction = iota
	trustGitHubAppActionAdd
	trustGitHubAppActionRemove
	trustGitHubAppActionEnableApprovals
	trustGitHubAppActionDisableApprovals
)

type trustGitHubAppScreen struct {
	operationList  list.Model
	selectedAction trustGitHubAppAction
	inputs         []textinput.Model
	focusIndex     int
}

func (s *trustGitHubAppScreen) Update(msg tea.Msg, m *model) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch m.screen {
	case screenTrustGitHubApp:
		if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" {
			if sel, ok := s.operationList.SelectedItem().(item); ok {
				s.selectAction(sel.title, m)
				return *m, nil
			}
		}

		s.operationList, cmd = s.operationList.Update(msg)
		return *m, cmd

	case screenTrustAddGitHubAppForm, screenTrustGitHubAppActionForm:
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "enter":
				return s.handleFormSubmit(m)
			case "tab", "shift+tab", "up", "down":
				s.cycleFocus(keyMsg.String())
				m.footer = ""
				return *m, nil
			}
		}

		if len(s.inputs) > 0 {
			s.inputs[s.focusIndex], cmd = s.inputs[s.focusIndex].Update(msg)
		}
		return *m, cmd
	}

	return *m, nil
}

func (s *trustGitHubAppScreen) View(m *model) string {
	switch m.screen {
	case screenTrustGitHubApp:
		return m.renderScreen("Home › Trust › GitHub App", s.operationList.View(), renderActionHints(m.readOnly))
	case screenTrustAddGitHubAppForm:
		return m.renderScreen("Home › Trust › GitHub App › Add", s.renderAddForm(), renderActionHints(m.readOnly))
	case screenTrustGitHubAppActionForm:
		return m.renderScreen("Home › Trust › GitHub App › Action", s.renderActionForm(), renderActionHints(m.readOnly))
	default:
		return ""
	}
}

func (s *trustGitHubAppScreen) selectAction(title string, m *model) {
	switch title {
	case "Add GitHub App":
		s.selectedAction = trustGitHubAppActionAdd
		s.inputs = initInputs([]inputField{
			{tuf.GitHubAppRoleName, "App Name: "},
			{"Path to public key or key ref", "App Key: "},
		})
		s.focusIndex = 0
		m.screen = screenTrustAddGitHubAppForm
	case "Remove GitHub App":
		s.selectedAction = trustGitHubAppActionRemove
		s.inputs = initInputs([]inputField{
			{tuf.GitHubAppRoleName, "App Name: "},
		})
		s.focusIndex = 0
		m.screen = screenTrustGitHubAppActionForm
	case "Enable App Approvals":
		s.selectedAction = trustGitHubAppActionEnableApprovals
		s.inputs = initInputs([]inputField{
			{tuf.GitHubAppRoleName, "App Name: "},
		})
		s.focusIndex = 0
		m.screen = screenTrustGitHubAppActionForm
	case "Disable App Approvals":
		s.selectedAction = trustGitHubAppActionDisableApprovals
		s.inputs = initInputs([]inputField{
			{tuf.GitHubAppRoleName, "App Name: "},
		})
		s.focusIndex = 0
		m.screen = screenTrustGitHubAppActionForm
	default:
		s.selectedAction = trustGitHubAppActionNone
	}
}

func (s *trustGitHubAppScreen) cycleFocus(key string) {
	if len(s.inputs) == 0 {
		return
	}

	if key == "up" || key == "shift+tab" {
		if s.focusIndex > 0 {
			s.focusIndex--
		} else {
			s.focusIndex = len(s.inputs) - 1
		}
	} else {
		if s.focusIndex < len(s.inputs)-1 {
			s.focusIndex++
		} else {
			s.focusIndex = 0
		}
	}

	for i := range s.inputs {
		if i == s.focusIndex {
			s.inputs[i].Focus()
			s.inputs[i].PromptStyle = focusedStyle
			s.inputs[i].TextStyle = focusedStyle
		} else {
			s.inputs[i].Blur()
			s.inputs[i].PromptStyle = blurredStyle
			s.inputs[i].TextStyle = blurredStyle
		}
	}
}

func (s *trustGitHubAppScreen) renderAddForm() string {
	return strings.Join([]string{
		"Add a trusted GitHub App",
		"",
		s.inputs[0].View(),
		s.inputs[1].View(),
		"",
		"Press Enter to submit.",
	}, "\n")
}

func (s *trustGitHubAppScreen) renderActionForm() string {
	return strings.Join([]string{
		s.actionLabel(),
		"",
		s.inputs[0].View(),
		"",
		"Press Enter to submit.",
	}, "\n")
}

func (s *trustGitHubAppScreen) actionLabel() string {
	switch s.selectedAction {
	case trustGitHubAppActionAdd:
		return "Add a trusted GitHub App"
	case trustGitHubAppActionRemove:
		return "Remove a trusted GitHub App"
	case trustGitHubAppActionEnableApprovals:
		return "Enable GitHub App approvals"
	case trustGitHubAppActionDisableApprovals:
		return "Disable GitHub App approvals"
	default:
		return "GitHub App action"
	}
}

func (s *trustGitHubAppScreen) handleFormSubmit(m *model) (tea.Model, tea.Cmd) {
	name := tuf.GitHubAppRoleName
	if len(s.inputs) > 0 {
		if value := strings.TrimSpace(s.inputs[0].Value()); value != "" {
			name = value
		}
	}

	var err error
	switch s.selectedAction {
	case trustGitHubAppActionAdd:
		appKey := strings.TrimSpace(s.inputs[1].Value())
		if appKey == "" {
			m.errorMsg = "App key is required"
			return *m, nil
		}
		err = repoAddGitHubApp(m.ctx, m.options, name, appKey)
		m.footer = "GitHub App added!"
	case trustGitHubAppActionRemove:
		err = repoRemoveGitHubApp(m.ctx, m.options, name)
		m.footer = "GitHub App removed!"
	case trustGitHubAppActionEnableApprovals:
		err = repoTrustGitHubApp(m.ctx, m.options, name)
		m.footer = "GitHub App approvals enabled!"
	case trustGitHubAppActionDisableApprovals:
		err = repoUntrustGitHubApp(m.ctx, m.options, name)
		m.footer = "GitHub App approvals disabled!"
	default:
		m.footer = "No GitHub App action selected."
		return *m, nil
	}

	if err != nil {
		m.errorMsg = fmt.Sprintf("Error updating GitHub App settings: %v", err)
		return *m, nil
	}

	m.errorMsg = ""
	s.inputs = nil
	s.focusIndex = 0
	s.selectedAction = trustGitHubAppActionNone
	m.screen = screenTrustGitHubApp
	return *m, nil
}
