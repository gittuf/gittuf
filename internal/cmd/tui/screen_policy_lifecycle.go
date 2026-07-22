// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
)

type policyLifecycleScreen struct {
	list       list.Model
	inputs     []textinput.Model
	focusIndex int
	action     string
}

type policyLifecycleResultMsg struct {
	msg string
	err error
}

func (s *policyLifecycleScreen) initInputs(action string, m *model) {
	s.action = action
	var fields []inputField
	switch action {
	case "Initialize Policy", "Increment Version", "Sign Policy":
		defaultName := m.policyName
		if defaultName == "" {
			defaultName = "targets"
		}
		fields = []inputField{
			{placeholder: "Enter policy/role name (default: " + defaultName + ")", prompt: "Policy/Role Name: "},
		}
	case "Stage Changes", "Apply Changes":
		fields = []inputField{
			{placeholder: "Enter remote name (leave blank for local-only)", prompt: "Remote Name: "},
		}
	case "Pull Policy", "Push Policy":
		fields = []inputField{
			{placeholder: "Enter remote name (default: origin)", prompt: "Remote Name: "},
		}
	}
	s.inputs = initInputs(fields)
	s.focusIndex = 0
	switch action {
	case "Initialize Policy", "Increment Version", "Sign Policy":
		defaultName := m.policyName
		if defaultName == "" {
			defaultName = "targets"
		}
		s.inputs[0].SetValue(defaultName)
	case "Pull Policy", "Push Policy":
		s.inputs[0].SetValue("origin")
	}
}

func (s *policyLifecycleScreen) cycleFocus(key string) {
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

func (s *policyLifecycleScreen) Update(msg tea.Msg, m *model) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch m.screen {
	case screenPolicyLifecycle:
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "enter":
				if selectedItem, ok := s.list.SelectedItem().(item); ok {
					if m.readOnly {
						m.errorMsg = "cannot perform action in read-only mode"
						return *m, nil
					}
					m.errorMsg = ""
					if selectedItem.title == "Discard Changes" {
						return *m, handlePolicyLifecycleCommand(m, selectedItem.title, "", "", false)
					}
					s.initInputs(selectedItem.title, m)
					m.screen = screenPolicyLifecycleForm
					return *m, nil
				}
			case "esc", "backspace":
				m.errorMsg = ""
				m.screen = screenPolicy
				return *m, nil
			}
		}
		s.list, cmd = s.list.Update(msg)
		return *m, cmd

	case screenPolicyLifecycleForm:
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
		s.inputs[s.focusIndex], cmd = s.inputs[s.focusIndex].Update(msg)
		return *m, cmd
	}

	return *m, nil
}

func (s *policyLifecycleScreen) handleFormSubmit(m *model) (tea.Model, tea.Cmd) {
	inputValue := strings.TrimSpace(s.inputs[0].Value())

	var cmd tea.Cmd
	switch s.action {
	case "Initialize Policy":
		if inputValue == "" {
			inputValue = "targets"
		}
		cmd = handlePolicyLifecycleCommand(m, s.action, inputValue, "", false)
	case "Increment Version":
		if inputValue == "" {
			inputValue = "targets"
		}
		cmd = handlePolicyLifecycleCommand(m, s.action, inputValue, "", false)
	case "Sign Policy":
		if inputValue == "" {
			inputValue = "targets"
		}
		cmd = handlePolicyLifecycleCommand(m, s.action, inputValue, "", false)
	case "Stage Changes":
		localOnly := inputValue == ""
		remoteName := inputValue
		cmd = handlePolicyLifecycleCommand(m, s.action, "", remoteName, localOnly)
	case "Apply Changes":
		localOnly := inputValue == ""
		remoteName := inputValue
		cmd = handlePolicyLifecycleCommand(m, s.action, "", remoteName, localOnly)
	case "Pull Policy", "Push Policy":
		if inputValue == "" {
			inputValue = "origin"
		}
		cmd = handlePolicyLifecycleCommand(m, s.action, "", inputValue, false)
	}

	m.screen = screenPolicyLifecycle
	return *m, cmd
}

func (s *policyLifecycleScreen) View(m *model) string {
	switch m.screen {
	case screenPolicyLifecycle:
		hints := "\n" + renderStyledHelp([][2]string{
			{"enter", "select"},
			{"h", "help"},
			{"esc", "back"},
			{"q", "quit"},
		})
		return m.renderScreen("Home › Policy › Lifecycle", s.list.View(), hints)

	case screenPolicyLifecycleForm:
		breadcrumb := fmt.Sprintf("Home › Policy › Lifecycle › %s", s.action)
		var b strings.Builder
		b.WriteString(titleStyle.Render(s.action))
		b.WriteString("\n\n")

		if s.action == "Increment Version" {
			warningStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorErrorMsg)).
				Bold(true)
			b.WriteString(warningStyle.Render("Warning: This is an advanced operation rarely needed under normal workflows.") + "\n\n")
		}

		for _, input := range s.inputs {
			b.WriteString(input.View())
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString("Press Tab to advance, Enter to advance/submit and Esc to go back.")
		b.WriteString("\n")
		b.WriteString(renderFooterBox(*m))
		if m.errorMsg != "" {
			b.WriteString("\n")
			b.WriteString(renderErrorMsg(m.errorMsg))
		}
		return lipgloss.JoinVertical(lipgloss.Left,
			renderStatusBar(breadcrumb, m.readOnly, m.width),
			renderWithMargin(b.String()),
		)
	}

	return ""
}

func handlePolicyLifecycleCommand(m *model, action string, policyName string, remote string, localOnly bool) tea.Cmd {
	return func() tea.Msg {
		if m.readOnly {
			return policyLifecycleResultMsg{
				msg: "",
				err: fmt.Errorf("cannot perform action in read-only mode"),
			}
		}

		var err error
		var successMsg string

		opts := []trustpolicyopts.Option{}
		if m.options.p != nil && m.options.p.WithRSLEntry {
			opts = append(opts, trustpolicyopts.WithRSLEntry())
		}

		switch action {
		case "Initialize Policy":
			err = m.repo.InitializeTargets(m.ctx, m.signer, policyName, true, opts...)
			successMsg = fmt.Sprintf("Successfully initialized policy %q.", policyName)
		case "Increment Version":
			err = m.repo.IncrementTargetsVersion(m.ctx, m.signer, policyName, true, opts...)
			successMsg = fmt.Sprintf("Successfully incremented policy version for %q.", policyName)
		case "Sign Policy":
			err = m.repo.SignTargets(m.ctx, m.signer, policyName, true, opts...)
			successMsg = fmt.Sprintf("Successfully signed policy %q.", policyName)
		case "Stage Changes":
			err = m.repo.StagePolicy(m.ctx, remote, localOnly, true)
			if localOnly {
				successMsg = "Successfully staged policy changes locally."
			} else {
				successMsg = fmt.Sprintf("Successfully staged policy changes to remote %q.", remote)
			}
		case "Apply Changes":
			err = m.repo.ApplyPolicy(m.ctx, remote, localOnly, true)
			if localOnly {
				successMsg = "Successfully applied policy changes locally."
			} else {
				successMsg = fmt.Sprintf("Successfully applied policy changes to remote %q.", remote)
			}
		case "Discard Changes":
			err = m.repo.DiscardPolicy()
			successMsg = "Successfully discarded policy changes."
		case "Pull Policy":
			err = m.repo.PullPolicy(remote)
			successMsg = fmt.Sprintf("Successfully pulled policy from remote %q.", remote)
		case "Push Policy":
			err = m.repo.PushPolicy(remote)
			successMsg = fmt.Sprintf("Successfully pushed policy to remote %q.", remote)
		default:
			err = fmt.Errorf("unknown action: %s", action)
		}

		return policyLifecycleResultMsg{
			msg: successMsg,
			err: err,
		}
	}
}
