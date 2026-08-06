// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type trustRepoNetworkAction int

const (
	trustRepoNetworkActionNone trustRepoNetworkAction = iota
	trustRepoNetworkActionAddControllerRepository
	trustRepoNetworkActionAddNetworkRepository
	trustRepoNetworkActionSetRepositoryLocation
	trustRepoNetworkActionMakeController
)

type trustRepoNetworkScreen struct {
	operationList  list.Model
	selectedAction trustRepoNetworkAction
	inputs         []textinput.Model
	focusIndex     int
}

func (s *trustRepoNetworkScreen) Update(msg tea.Msg, m *model) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch m.screen {
	case screenTrustRepoNetwork:
		if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" {
			if sel, ok := s.operationList.SelectedItem().(item); ok {
				s.selectAction(sel.title, m)
				if s.selectedAction == trustRepoNetworkActionMakeController {
					return s.handleFormSubmit(m)
				}
				return *m, nil
			}
		}

		s.operationList, cmd = s.operationList.Update(msg)
		return *m, cmd

	case screenTrustRepoForm, screenTrustRepoLocationForm:
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

func (s *trustRepoNetworkScreen) View(m *model) string {
	switch m.screen {
	case screenTrustRepoNetwork:
		return m.renderScreen("Home › Trust › Repo/Network", s.operationList.View(), renderActionHints(m.readOnly))
	case screenTrustRepoForm:
		return m.renderScreen("Home › Trust › Repo/Network › Repository", s.renderRepoForm(), renderActionHints(m.readOnly))
	case screenTrustRepoLocationForm:
		return m.renderScreen("Home › Trust › Repo/Network › Location", s.renderLocationForm(), renderActionHints(m.readOnly))
	default:
		return ""
	}
}

func (s *trustRepoNetworkScreen) selectAction(title string, m *model) {
	switch title {
	case "Add Controller Repository":
		s.selectedAction = trustRepoNetworkActionAddControllerRepository
		s.inputs = initInputs([]inputField{
			{"Repository name", "Name: "},
			{"Repository location", "Location: "},
			{"Initial root principals (comma-separated)", "Initial Root Principals: "},
		})
		s.focusIndex = 0
		m.screen = screenTrustRepoForm
	case "Add Network Repository":
		s.selectedAction = trustRepoNetworkActionAddNetworkRepository
		s.inputs = initInputs([]inputField{
			{"Repository name", "Name: "},
			{"Repository location", "Location: "},
			{"Initial root principals (comma-separated)", "Initial Root Principals: "},
		})
		s.focusIndex = 0
		m.screen = screenTrustRepoForm
	case "Set Repository Location":
		s.selectedAction = trustRepoNetworkActionSetRepositoryLocation
		s.inputs = initInputs([]inputField{
			{"Repository location", "Location: "},
		})
		s.focusIndex = 0
		m.screen = screenTrustRepoLocationForm
	case "Make Controller":
		s.selectedAction = trustRepoNetworkActionMakeController
	default:
		s.selectedAction = trustRepoNetworkActionNone
	}
}

func (s *trustRepoNetworkScreen) cycleFocus(key string) {
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

func (s *trustRepoNetworkScreen) renderRepoForm() string {
	return strings.Join([]string{
		s.actionLabel(),
		"",
		s.inputs[0].View(),
		s.inputs[1].View(),
		s.inputs[2].View(),
		"",
		"Press Enter to submit.",
	}, "\n")
}

func (s *trustRepoNetworkScreen) renderLocationForm() string {
	return strings.Join([]string{
		s.actionLabel(),
		"",
		s.inputs[0].View(),
		"",
		"Press Enter to submit.",
	}, "\n")
}

func (s *trustRepoNetworkScreen) actionLabel() string {
	switch s.selectedAction {
	case trustRepoNetworkActionAddControllerRepository:
		return "Add a controller repository"
	case trustRepoNetworkActionAddNetworkRepository:
		return "Add a network repository"
	case trustRepoNetworkActionSetRepositoryLocation:
		return "Set repository location"
	case trustRepoNetworkActionMakeController:
		return "Make this repository a controller"
	default:
		return "Repo/Network action"
	}
}

func (s *trustRepoNetworkScreen) handleFormSubmit(m *model) (tea.Model, tea.Cmd) {
	var err error

	switch s.selectedAction {
	case trustRepoNetworkActionAddControllerRepository, trustRepoNetworkActionAddNetworkRepository:
		name := strings.TrimSpace(s.inputs[0].Value())
		location := strings.TrimSpace(s.inputs[1].Value())
		principals := splitAndTrim(s.inputs[2].Value())
		if name == "" || location == "" || len(principals) == 0 || (len(principals) == 1 && principals[0] == "") {
			m.errorMsg = "Name, location, and at least one initial root principal are required"
			return *m, nil
		}
		if s.selectedAction == trustRepoNetworkActionAddControllerRepository {
			err = repoAddControllerRepository(m.ctx, m.options, name, location, principals)
			m.footer = "Controller repository added!"
		} else {
			err = repoAddNetworkRepository(m.ctx, m.options, name, location, principals)
			m.footer = "Network repository added!"
		}
	case trustRepoNetworkActionSetRepositoryLocation:
		location := strings.TrimSpace(s.inputs[0].Value())
		if location == "" {
			m.errorMsg = "Location is required"
			return *m, nil
		}
		err = repoSetRepositoryLocation(m.ctx, m.options, location)
		m.footer = "Repository location set!"
	case trustRepoNetworkActionMakeController:
		err = repoMakeController(m.ctx, m.options)
		m.footer = "Repository marked as controller!"
	default:
		m.footer = "No Repo/Network action selected."
		return *m, nil
	}

	if err != nil {
		m.errorMsg = fmt.Sprintf("Error updating repo/network settings: %v", err)
		return *m, nil
	}

	m.errorMsg = ""
	s.inputs = nil
	s.focusIndex = 0
	s.selectedAction = trustRepoNetworkActionNone
	m.screen = screenTrustRepoNetwork
	return *m, nil
}
