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

type trustPropagationAction int

const (
	trustPropagationActionNone trustPropagationAction = iota
	trustListDirectivesAction
	trustAddDirectiveAction
	trustUpdateDirectiveAction
	trustRemoveDirectiveAction
)

type trustPropagationScreen struct {
	operationList  list.Model
	selectedAction trustPropagationAction
	directives     []trustPropagationDirective
	inputs         []textinput.Model
	focusIndex     int
	showDirectives bool
}

func (s *trustPropagationScreen) Update(msg tea.Msg, m *model) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch m.screen {
	case screenTrustPropagation:
		if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" {
			if sel, ok := s.operationList.SelectedItem().(item); ok {
				s.selectAction(sel.title, m)
				if s.selectedAction == trustListDirectivesAction {
					runSelectedPropagationAction(s, m)
				}
				return *m, nil
			}
		}

		s.operationList, cmd = s.operationList.Update(msg)
		return *m, cmd

	case screenTrustAddPropagationForm, screenTrustUpdatePropagationForm, screenTrustRemovePropagationForm:
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "enter":
				return s.handlePropagationFormSubmit(m)
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

func (s *trustPropagationScreen) View(m *model) string {
	switch m.screen {
	case screenTrustPropagation:
		if s.showDirectives {
			return m.renderScreen("Home › Trust › Propagation", s.renderDirectives(), renderActionHints(m.readOnly))
		}
		return m.renderScreen("Home › Trust › Propagation", s.operationList.View(), renderActionHints(m.readOnly))
	case screenTrustAddPropagationForm:
		return m.renderScreen("Home › Trust › Propagation › Add Directive", s.renderDirectiveForm(), renderActionHints(m.readOnly))
	case screenTrustUpdatePropagationForm:
		return m.renderScreen("Home › Trust › Propagation › Update Directive", s.renderDirectiveForm(), renderActionHints(m.readOnly))
	case screenTrustRemovePropagationForm:
		return m.renderScreen("Home › Trust › Propagation › Remove Directive", s.renderRemoveDirectiveForm(), renderActionHints(m.readOnly))
	default:
		return ""
	}
}

func (s *trustPropagationScreen) selectAction(title string, m *model) {
	switch title {
	case "List Directives":
		s.selectedAction = trustListDirectivesAction
	case "Add Directive":
		s.selectedAction = trustAddDirectiveAction
		s.inputs = initInputs([]inputField{
			{"Directive name", "Name: "},
			{"Upstream repository", "From Repository: "},
			{"Upstream reference", "From Reference: "},
			{"Upstream path", "From Path: "},
			{"Downstream reference", "Into Reference: "},
			{"Downstream path", "Into Path: "},
		})
		s.focusIndex = 0
		s.showDirectives = false
		m.screen = screenTrustAddPropagationForm
	case "Update Directive":
		s.selectedAction = trustUpdateDirectiveAction
		s.inputs = initInputs([]inputField{
			{"Directive name", "Name: "},
			{"Upstream repository", "From Repository: "},
			{"Upstream reference", "From Reference: "},
			{"Upstream path", "From Path: "},
			{"Downstream reference", "Into Reference: "},
			{"Downstream path", "Into Path: "},
		})
		s.focusIndex = 0
		s.showDirectives = false
		m.screen = screenTrustUpdatePropagationForm
	case "Remove Directive":
		s.selectedAction = trustRemoveDirectiveAction
		s.inputs = initInputs([]inputField{
			{"Directive name", "Name: "},
		})
		s.focusIndex = 0
		s.showDirectives = false
		m.screen = screenTrustRemovePropagationForm
	default:
		s.selectedAction = trustPropagationActionNone
	}
}

func (s *trustPropagationScreen) cycleFocus(key string) {
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

// runSelectedPropagationAction executes the selected propagation menu action.
func runSelectedPropagationAction(s *trustPropagationScreen, m *model) {
	switch s.selectedAction {
	case trustListDirectivesAction:
		directives, err := repoListPropagationDirectives(m.ctx, m.options)
		if err != nil {
			m.errorMsg = fmt.Sprintf("Error listing directives: %v", err)
			return
		}

		m.errorMsg = ""
		m.footer = "Propagation directives listed!"
		s.directives = directives
		s.showDirectives = true
		s.selectedAction = trustPropagationActionNone
	default:
		m.footer = "No propagation action selected."
	}
}

func (s *trustPropagationScreen) renderDirectiveForm() string {
	return strings.Join([]string{
		"Configure a propagation directive",
		"",
		s.inputs[0].View(),
		s.inputs[1].View(),
		s.inputs[2].View(),
		s.inputs[3].View(),
		s.inputs[4].View(),
		s.inputs[5].View(),
		"",
		"Press Enter to submit.",
	}, "\n")
}

func (s *trustPropagationScreen) renderRemoveDirectiveForm() string {
	return strings.Join([]string{
		"Remove a propagation directive",
		"",
		s.inputs[0].View(),
		"",
		"Press Enter to submit.",
	}, "\n")
}

func (s *trustPropagationScreen) handlePropagationFormSubmit(m *model) (tea.Model, tea.Cmd) {
	switch s.selectedAction {
	case trustAddDirectiveAction, trustUpdateDirectiveAction:
		name := strings.TrimSpace(s.inputs[0].Value())
		upstreamRepository := strings.TrimSpace(s.inputs[1].Value())
		upstreamReference := strings.TrimSpace(s.inputs[2].Value())
		upstreamPath := strings.TrimSpace(s.inputs[3].Value())
		downstreamReference := strings.TrimSpace(s.inputs[4].Value())
		downstreamPath := strings.TrimSpace(s.inputs[5].Value())
		if name == "" || upstreamRepository == "" || upstreamReference == "" || downstreamReference == "" || downstreamPath == "" {
			m.errorMsg = "Name, from repository, from reference, into reference, and into path are required"
			return *m, nil
		}

		var err error
		if s.selectedAction == trustAddDirectiveAction {
			err = repoAddPropagationDirective(m.ctx, m.options, name, upstreamRepository, upstreamReference, upstreamPath, downstreamReference, downstreamPath)
		} else {
			err = repoUpdatePropagationDirective(m.ctx, m.options, name, upstreamRepository, upstreamReference, upstreamPath, downstreamReference, downstreamPath)
		}
		if err != nil {
			m.errorMsg = fmt.Sprintf("Error saving directive: %v", err)
			return *m, nil
		}
		if s.selectedAction == trustAddDirectiveAction {
			m.footer = "Propagation directive added!"
		} else {
			m.footer = "Propagation directive updated!"
		}
	case trustRemoveDirectiveAction:
		name := strings.TrimSpace(s.inputs[0].Value())
		if name == "" {
			m.errorMsg = "Directive name is required"
			return *m, nil
		}
		if err := repoRemovePropagationDirective(m.ctx, m.options, name); err != nil {
			m.errorMsg = fmt.Sprintf("Error removing directive: %v", err)
			return *m, nil
		}
		m.footer = "Propagation directive removed!"
	default:
		m.footer = "No propagation action selected."
		return *m, nil
	}

	m.errorMsg = ""
	s.inputs = nil
	s.focusIndex = 0
	s.directives = nil
	s.showDirectives = false
	s.selectedAction = trustPropagationActionNone
	m.screen = screenTrustPropagation
	return *m, nil
}
