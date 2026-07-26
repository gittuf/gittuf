// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/list"
)

type trustLifecycleAction int

const (
	trustLifecycleActionNone trustLifecycleAction = iota
	trustLifecycleActionStage
	trustLifecycleActionSign
	trustLifecycleActionApply
)

type trustLifecycleScreen struct {
	operationList  list.Model
	selectedAction trustLifecycleAction
}

func (s *trustLifecycleScreen) Update(msg tea.Msg, m *model) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch m.screen {
	case screenTrustLifecycle:
		if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" {
			if sel, ok := s.operationList.SelectedItem().(item); ok {
				s.selectAction(sel.title, m)
				runSelectedLifecycleAction(s, m)
				return *m, nil
			}
		}

		s.operationList, cmd = s.operationList.Update(msg)
		return *m, cmd
	}
	return *m, nil
}

func (s *trustLifecycleScreen) View(m *model) string {
	switch m.screen {
	case screenTrustLifecycle:
		return m.renderScreen("Home › Trust › Lifecycle", s.operationList.View(), renderActionHints(m.readOnly))
	default:
		return ""
	}
}

func (s *trustLifecycleScreen) selectAction(title string, m *model) {
	switch title {
	case "Stage Trust Changes":
		s.selectedAction = trustLifecycleActionStage
	case "Sign Trust Changes":
		s.selectedAction = trustLifecycleActionSign
	case "Apply Trust Changes":
		s.selectedAction = trustLifecycleActionApply
	default:
		s.selectedAction = trustLifecycleActionNone
	}
}


func (s *trustLifecycleScreen) actionLabel() string {
	switch s.selectedAction {
	case trustLifecycleActionStage:
		return "Stage Trust Change"
	case trustLifecycleActionSign:
		return "Sign Trust Change"
	case trustLifecycleActionApply:
		return "Apply Trust Change"
	default:
		return "lifecycle action"
	}
}

func runSelectedLifecycleAction(s *trustLifecycleScreen, m *model) {
	var err error
	switch s.selectedAction {
	case trustLifecycleActionStage:
		err = repoStageTrustChanges(m.ctx, m.options)
		if err != nil {
			m.errorMsg = fmt.Sprintf("Error staging trust changes: %v", err)
			return
		}

		m.errorMsg = ""
		m.footer = "Trust changes staged!"
		s.selectedAction = trustLifecycleActionNone
		m.screen = screenTrustLifecycle
	case trustLifecycleActionSign:
		err = repoSignTrustChanges(m.ctx, m.options)
		if err != nil {
			m.errorMsg = fmt.Sprintf("Error signing trust changes: %v", err)
			return
		}

		m.errorMsg = ""
		m.footer = "Trust changes signed!"
		s.selectedAction = trustLifecycleActionNone
		m.screen = screenTrustLifecycle
	case trustLifecycleActionApply:
		err = repoApplyTrustChanges(m.ctx, m.options)
		if err != nil {
			m.errorMsg = fmt.Sprintf("Error applying trust changes: %v", err)
			return
		}

		m.errorMsg = ""
		m.footer = "Trust changes applied!"
		s.selectedAction = trustLifecycleActionNone
		m.screen = screenTrustLifecycle
	default:
		m.footer = "No lifecycle action selected."
	}
}
