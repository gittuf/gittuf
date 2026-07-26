// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type trustHookAction int

const (
	trustHookActionNone trustHookAction = iota
	trustListHooksAction
	trustAddHookAction
	trustUpdateHookAction
	trustRemoveHookAction
)

type trustHookScreen struct {
	operationList  list.Model
	selectedAction trustHookAction
	hooks          []trustHook
	inputs         []textinput.Model
	focusIndex     int
	isPreCommit    bool
	isPrePush      bool
	showHooks      bool
}

func (s *trustHookScreen) Update(msg tea.Msg, m *model) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch m.screen {
	case screenTrustHooks:
		if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" {
			if sel, ok := s.operationList.SelectedItem().(item); ok {
				s.selectAction(sel.title, m)
				if sel.title == "List Hooks" {
					runSelectedHookAction(s, m)
				}
				return *m, nil
			}
		}

		s.operationList, cmd = s.operationList.Update(msg)
		return *m, cmd

	case screenTrustAddHookForm, screenTrustUpdateHookForm:
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "enter":
				return s.handleHookFormSubmit(m)
			case "up", "down", "left", "right", "tab":
				s.cycleFocus(keyMsg.String())
				return *m, nil
			}
		}

		if len(s.inputs) > 0 && s.focusIndex < len(s.inputs) {
			s.inputs[s.focusIndex], cmd = s.inputs[s.focusIndex].Update(msg)
		}
		return *m, cmd

	case screenTrustRemoveHookForm:
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "enter":
				return s.handleRemoveHookSubmit(m)
			case "up", "down", "left", "right", "tab":
				s.cycleFocus(keyMsg.String())
				return *m, nil
			}
		}

		if len(s.inputs) > 0 && s.focusIndex < len(s.inputs) {
			s.inputs[s.focusIndex], cmd = s.inputs[s.focusIndex].Update(msg)
		}
		return *m, cmd
	}
	return *m, nil
}

func (s *trustHookScreen) View(m *model) string {
	switch m.screen {
	case screenTrustHooks:
		if s.showHooks {
			return m.renderScreen("Home › Trust › Hooks", s.renderHooks(), renderActionHints(m.readOnly))
		}
		return m.renderScreen("Home › Trust › Hooks", s.operationList.View(), renderActionHints(m.readOnly))
	case screenTrustRemoveHookForm:
		return m.renderScreen("Home › Trust › Hooks › Remove Hook", s.renderRemoveHookForm(), renderActionHints(m.readOnly))
	case screenTrustAddHookForm:
		return m.renderScreen("Home › Trust › Hooks › Add Hook", s.renderHookForm(), renderActionHints(m.readOnly))
	case screenTrustUpdateHookForm:
		return m.renderScreen("Home › Trust › Hooks › Update Hook", s.renderHookForm(), renderActionHints(m.readOnly))
	default:
		return ""
	}
}

func (s *trustHookScreen) selectAction(title string, m *model) {
	switch title {
	case "List Hooks":
		s.selectedAction = trustListHooksAction
	case "Add Hook":
		s.selectedAction = trustAddHookAction
		s.inputs = initInputs([]inputField{
			{"Hook name", "Hook Name: "},
			{"Path to hook script", "Script Path: "},
			{"Environment", "Environment (lua): "},
			{"Principal IDs (comma-separated)", "Principal IDs: "},
			{"Timeout (seconds)", "Timeout: "},
		})
		s.isPreCommit = true
		s.isPrePush = false
		s.focusIndex = 0
		m.screen = screenTrustAddHookForm
	case "Update Hook":
		s.selectedAction = trustUpdateHookAction
		s.inputs = initInputs([]inputField{
			{"Hook name", "Hook Name: "},
			{"Path to hook script", "Script Path: "},
			{"Environment", "Environment (lua): "},
			{"Principal IDs (comma-separated)", "Principal IDs: "},
			{"Timeout (seconds)", "Timeout: "},
		})
		s.isPreCommit = true
		s.isPrePush = false
		s.focusIndex = 0
		m.screen = screenTrustUpdateHookForm
	case "Remove Hook":
		s.selectedAction = trustRemoveHookAction
		s.inputs = initInputs([]inputField{
			{"Hook name", "Hook Name: "},
		})
		s.isPreCommit = true
		s.isPrePush = false
		s.focusIndex = 0
		m.screen = screenTrustRemoveHookForm
	default:
		s.selectedAction = trustHookActionNone
	}
}

func (s *trustHookScreen) cycleFocus(key string) {
	if len(s.inputs) == 0 {
		return
	}

	stageIndex := len(s.inputs)

	if key == "up" || key == "left" {
		if s.focusIndex > 0 {
			s.focusIndex--
		} else {
			s.focusIndex = stageIndex
		}
	} else {
		if s.focusIndex < stageIndex {
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
			continue
		}

		s.inputs[i].Blur()
		s.inputs[i].PromptStyle = blurredStyle
		s.inputs[i].TextStyle = blurredStyle
	}

	if s.focusIndex == stageIndex {
		s.isPreCommit = !s.isPreCommit
		s.isPrePush = !s.isPrePush
	}
}

func runSelectedHookAction(s *trustHookScreen, m *model) {
	switch s.selectedAction {
	case trustListHooksAction:
		trustHooks, err := repoListHooks(m.ctx, m.options)
		if err != nil {
			m.errorMsg = fmt.Sprintf("Error listing hooks: %v", err)
			return
		}

		m.errorMsg = ""
		m.footer = "Hooks listed!"
		s.hooks = trustHooks
		s.showHooks = true
		s.selectedAction = trustHookActionNone
		m.screen = screenTrustHooks
	default:
		m.footer = "No hook action selected."
	}
}
