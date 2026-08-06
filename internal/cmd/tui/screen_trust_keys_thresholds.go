// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type trustKeysAction int

const (
	trustKeysActionNone trustKeysAction = iota
	trustKeysActionAddRootKey
	trustKeysActionRemoveRootKey
	trustKeysActionAddPolicyKey
	trustKeysActionRemovePolicyKey
	trustKeysActionUpdateRootThreshold
	trustKeysActionUpdatePolicyThreshold
)

type trustKeysThresholdsScreen struct {
	operationList  list.Model
	inputs         []textinput.Model
	focusIndex     int
	selectedAction trustKeysAction
}

func (s *trustKeysThresholdsScreen) Update(msg tea.Msg, m *model) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch m.screen {
	case screenTrustKeysThresholds:
		if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" {
			if sel, ok := s.operationList.SelectedItem().(item); ok {
				s.selectAction(sel.title, m)
				return *m, nil
			}
		}

		s.operationList, cmd = s.operationList.Update(msg)
		return *m, cmd

	case screenTrustKeyForm, screenTrustThresholdForm:
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

		if len(s.inputs) == 0 {
			return *m, nil
		}

		s.inputs[s.focusIndex], cmd = s.inputs[s.focusIndex].Update(msg)
		return *m, cmd
	}

	return *m, nil
}

func (s *trustKeysThresholdsScreen) View(m *model) string {
	switch m.screen {
	case screenTrustKeysThresholds:
		return m.renderScreen("Home › Trust › Keys & Thresholds", s.operationList.View(), renderActionHints(m.readOnly))
	case screenTrustKeyForm:
		return s.renderFormScreen(m, "Trust Key Form", "Home › Trust › Keys & Thresholds › Key Form")
	case screenTrustThresholdForm:
		return s.renderFormScreen(m, "Threshold Form", "Home › Trust › Keys & Thresholds › Threshold Form")
	default:
		return ""
	}
}

func (s *trustKeysThresholdsScreen) selectAction(title string, m *model) {
	if m.readOnly {
		m.footer = "Read-only mode: action unavailable."
		return
	}

	switch title {
	case "Add Root Key":
		s.selectedAction = trustKeysActionAddRootKey
		s.initKeyInputs("Public key path or key material")
		m.screen = screenTrustKeyForm
	case "Remove Root Key":
		s.selectedAction = trustKeysActionRemoveRootKey
		s.initKeyInputs("Root key ID")
		m.screen = screenTrustKeyForm
	case "Add Policy Key":
		s.selectedAction = trustKeysActionAddPolicyKey
		s.initKeyInputs("Public key path or key material")
		m.screen = screenTrustKeyForm
	case "Remove Policy Key":
		s.selectedAction = trustKeysActionRemovePolicyKey
		s.initKeyInputs("Policy key ID")
		m.screen = screenTrustKeyForm
	case "Update Root Threshold":
		s.selectedAction = trustKeysActionUpdateRootThreshold
		s.initThresholdInputs()
		m.screen = screenTrustThresholdForm
	case "Update Policy Threshold":
		s.selectedAction = trustKeysActionUpdatePolicyThreshold
		s.initThresholdInputs()
		m.screen = screenTrustThresholdForm
	}
}

func (s *trustKeysThresholdsScreen) initKeyInputs(placeholder string) {
	s.inputs = initInputs([]inputField{
		{placeholder, "Value:"},
	})
	s.focusIndex = 0
}

func (s *trustKeysThresholdsScreen) initThresholdInputs() {
	s.inputs = initInputs([]inputField{
		{"Enter threshold", "Threshold:"},
	})
	s.focusIndex = 0
}

func (s *trustKeysThresholdsScreen) cycleFocus(key string) {
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

func (s *trustKeysThresholdsScreen) handleFormSubmit(m *model) (tea.Model, tea.Cmd) {
	var err error
	switch s.selectedAction {
	case trustKeysActionAddRootKey:
		value := strings.TrimSpace(s.inputs[0].Value())
		if value == "" {
			m.errorMsg = "Key input is required"
			return *m, nil
		}

		err = repoAddRootKey(m.ctx, m.options, value)
		if err != nil {
			m.errorMsg = fmt.Sprintf("Error adding root key: %v", err)
			return *m, nil
		}

		m.errorMsg = ""
		m.footer = "Root key added!"
		s.inputs = nil
		s.selectedAction = trustKeysActionNone
		m.screen = screenTrustKeysThresholds
		return *m, nil

	case trustKeysActionRemoveRootKey:
		value := strings.TrimSpace(s.inputs[0].Value())
		if value == "" {
			m.errorMsg = "Key input is required"
			return *m, nil
		}

		err = repoRemoveRootKey(m.ctx, m.options, value)
		if err != nil {
			m.errorMsg = fmt.Sprintf("Error removing root key: %v", err)
			return *m, nil
		}

		m.errorMsg = ""
		m.footer = "Root key removed!"
		s.inputs = nil
		s.selectedAction = trustKeysActionNone
		m.screen = screenTrustKeysThresholds
		return *m, nil

	case trustKeysActionAddPolicyKey:
		value := strings.TrimSpace(s.inputs[0].Value())
		if value == "" {
			m.errorMsg = "Key input is required"
			return *m, nil
		}

		err = repoAddPolicyKey(m.ctx, m.options, value)
		if err != nil {
			m.errorMsg = fmt.Sprintf("Error adding policy key: %v", err)
			return *m, nil
		}

		m.errorMsg = ""
		m.footer = "Policy key added!"
		s.inputs = nil
		s.selectedAction = trustKeysActionNone
		m.screen = screenTrustKeysThresholds
		return *m, nil

	case trustKeysActionRemovePolicyKey:
		value := strings.TrimSpace(s.inputs[0].Value())
		if value == "" {
			m.errorMsg = "Key input is required"
			return *m, nil
		}

		err = repoRemovePolicyKey(m.ctx, m.options, value)
		if err != nil {
			m.errorMsg = fmt.Sprintf("Error removing policy key: %v", err)
			return *m, nil
		}

		m.errorMsg = ""
		m.footer = "Policy key removed!"
		s.inputs = nil
		s.selectedAction = trustKeysActionNone
		m.screen = screenTrustKeysThresholds
		return *m, nil
	case trustKeysActionUpdateRootThreshold:
		value := strings.TrimSpace(s.inputs[0].Value())
		if value == "" {
			m.errorMsg = "Threshold input is required"
			return *m, nil
		}

		threshold, err := strconv.Atoi(value)
		if err != nil || threshold <= 0 {
			m.errorMsg = "Threshold must be a positive integer"
			return *m, nil
		}
		err = repoUpdateRootThreshold(m.ctx, m.options, threshold)
		if err != nil {
			m.errorMsg = fmt.Sprintf("Error updating root threshold: %v", err)
			return *m, nil
		}

		m.errorMsg = ""
		m.footer = "Root threshold updated!"
		s.inputs = nil
		s.selectedAction = trustKeysActionNone
		m.screen = screenTrustKeysThresholds
		return *m, nil

	case trustKeysActionUpdatePolicyThreshold:
		value := strings.TrimSpace(s.inputs[0].Value())
		if value == "" {
			m.errorMsg = "Threshold input is required"
			return *m, nil
		}

		threshold, err := strconv.Atoi(value)
		if err != nil || threshold <= 0 {
			m.errorMsg = "Threshold must be a positive integer"
			return *m, nil
		}
		err = repoUpdatePolicyThreshold(m.ctx, m.options, threshold)
		if err != nil {
			m.errorMsg = fmt.Sprintf("Error updating policy threshold: %v", err)
			return *m, nil
		}

		m.errorMsg = ""
		m.footer = "Policy threshold updated!"
		s.inputs = nil
		s.selectedAction = trustKeysActionNone
		m.screen = screenTrustKeysThresholds
		return *m, nil

	default:
		m.footer = fmt.Sprintf("TODO: implement %s", s.actionLabel())
		m.screen = screenTrustKeysThresholds
		return *m, nil
	}
}

func (s *trustKeysThresholdsScreen) actionLabel() string {
	switch s.selectedAction {
	case trustKeysActionAddRootKey:
		return "add root key"
	case trustKeysActionRemoveRootKey:
		return "remove root key"
	case trustKeysActionAddPolicyKey:
		return "add policy key"
	case trustKeysActionRemovePolicyKey:
		return "remove policy key"
	case trustKeysActionUpdateRootThreshold:
		return "update root threshold"
	case trustKeysActionUpdatePolicyThreshold:
		return "update policy threshold"
	default:
		return "keys & thresholds action"
	}
}

func (s *trustKeysThresholdsScreen) renderFormScreen(m *model, formTitle string, breadcrumb string) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(formTitle))
	b.WriteString("\n\n")

	for _, input := range s.inputs {
		b.WriteString(input.View())
		b.WriteString("\n")
	}

	b.WriteString("\nPress Tab to advance, Enter to submit, and Esc to go back.\n")
	return lipgloss.JoinVertical(
		lipgloss.Left,
		renderStatusBar(breadcrumb, m.readOnly, m.width),
		renderWithMargin(b.String()+renderFooterBox(*m)+renderErrorMsg(m.errorMsg)),
	)
}
