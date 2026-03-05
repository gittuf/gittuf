// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gittuf/gittuf/internal/tuf"
)

// Update updates the model based on the message received.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := lipgloss.NewStyle().Margin(1, 2).GetFrameSize()
		m.choiceList.SetSize(msg.Width-h, msg.Height-v)
		m.policyScreenList.SetSize(msg.Width-h, msg.Height-v)
		m.trustScreenList.SetSize(msg.Width-h, msg.Height-v)
		m.ruleList.SetSize(msg.Width-h, msg.Height-v)
		m.globalRuleList.SetSize(msg.Width-h, msg.Height-v)

	case tea.KeyMsg:
		// Delete confirmation overlay intercepts all keys
		if m.confirmDelete {
			return m.handleDeleteConfirm(msg.String())
		}

		// Global handlers (quit, back navigation)
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			// Only quit from non-form screens (avoid consuming 'q' in text inputs)
			if m.screen != screenPolicyAddRule && m.screen != screenPolicyEditRule &&
				m.screen != screenTrustAddGlobalRule && m.screen != screenTrustEditGlobalRule {
				return m, tea.Quit
			}
		case "esc":
			m.footer = ""
			switch m.screen {
			case screenPolicy, screenTrust:
				m.screen = screenChoice
			case screenPolicyRules:
				m.screen = screenPolicy
			case screenPolicyAddRule, screenPolicyEditRule:
				m.screen = screenPolicyRules
			case screenTrustGlobalRules:
				m.screen = screenTrust
			case screenTrustAddGlobalRule, screenTrustEditGlobalRule:
				m.screen = screenTrustGlobalRules
			}
			return m, nil
		}

		// Screen-specific input handling
		switch m.screen {
		case screenChoice, screenPolicy, screenTrust:
			if msg.String() == "enter" {
				return m.handleEnter()
			}
		case screenPolicyRules, screenTrustGlobalRules:
			return m.handleRulesListKey(msg)
		case screenPolicyAddRule, screenPolicyEditRule:
			if msg.String() == "enter" {
				return m.handlePolicyFormSubmit()
			}
			if msg.String() == "tab" || msg.String() == "shift+tab" || msg.String() == "up" || msg.String() == "down" {
				m.cycleFocus(msg.String())
				return m, nil
			}
		case screenTrustAddGlobalRule, screenTrustEditGlobalRule:
			if msg.String() == "enter" {
				return m.handleGlobalFormSubmit()
			}
			if msg.String() == "tab" || msg.String() == "shift+tab" || msg.String() == "up" || msg.String() == "down" {
				m.cycleFocus(msg.String())
				return m, nil
			}
		}
	}

	// Delegate to active bubbles component per screen
	switch m.screen {
	case screenChoice:
		m.choiceList, cmd = m.choiceList.Update(msg)
	case screenPolicy:
		m.policyScreenList, cmd = m.policyScreenList.Update(msg)
	case screenTrust:
		m.trustScreenList, cmd = m.trustScreenList.Update(msg)
	case screenPolicyRules:
		m.ruleList, cmd = m.ruleList.Update(msg)
	case screenTrustGlobalRules:
		m.globalRuleList, cmd = m.globalRuleList.Update(msg)
	case screenPolicyAddRule, screenPolicyEditRule, screenTrustAddGlobalRule, screenTrustEditGlobalRule:
		m.inputs[m.focusIndex], cmd = m.inputs[m.focusIndex].Update(msg)
	}

	return m, cmd
}

// handleEnter handles the enter key press on selection menu screens.
func (m model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenChoice:
		if i, ok := m.choiceList.SelectedItem().(item); ok {
			switch i.title {
			case "Policy":
				m.screen = screenPolicy
			case "Trust":
				m.screen = screenTrust
			}
		}
	case screenPolicy:
		if _, ok := m.policyScreenList.SelectedItem().(item); ok {
			m.screen = screenPolicyRules
			m.refreshRules()
		}
	case screenTrust:
		if _, ok := m.trustScreenList.SelectedItem().(item); ok {
			m.screen = screenTrustGlobalRules
			m.refreshGlobalRules()
		}
	}
	return m, nil
}

// handleRulesListKey handles keybindings on rule list screens (rules and global rules).
// For unhandled keys (including up/down arrows), it delegates to the active list for navigation.
func (m model) handleRulesListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if !m.readOnly {
		switch msg.String() {
		case "a":
			if m.screen == screenPolicyRules {
				m.initRuleInputs()
				m.screen = screenPolicyAddRule
			} else {
				m.initGlobalRuleInputs()
				m.screen = screenTrustAddGlobalRule
			}
			return m, nil
		case "e":
			if m.screen == screenPolicyRules {
				if sel, ok := m.ruleList.SelectedItem().(item); ok {
					for _, r := range m.rules {
						if r.name == sel.title {
							m.initRuleInputsPrefilled(r)
							m.screen = screenPolicyEditRule
							return m, nil
						}
					}
				}
			} else {
				if sel, ok := m.globalRuleList.SelectedItem().(item); ok {
					for _, gr := range m.globalRules {
						if gr.ruleName == sel.title {
							m.initGlobalRuleInputsPrefilled(gr)
							m.screen = screenTrustEditGlobalRule
							return m, nil
						}
					}
				}
			}
		case "d":
			var sel item
			var ok bool
			if m.screen == screenPolicyRules {
				sel, ok = m.ruleList.SelectedItem().(item)
			} else {
				sel, ok = m.globalRuleList.SelectedItem().(item)
			}
			if ok {
				m.confirmDelete = true
				m.deleteTarget = sel.title
				return m, nil
			}
		case "u", "k":
			if m.screen == screenPolicyRules {
				return m.handleReorderUp()
			}
		case "j":
			if m.screen == screenPolicyRules {
				return m.handleReorderDown()
			}
		}
	}

	// Delegate unhandled keys to the active list for navigation (up/down arrows, etc.)
	var cmd tea.Cmd
	if m.screen == screenPolicyRules {
		m.ruleList, cmd = m.ruleList.Update(msg)
	} else {
		m.globalRuleList, cmd = m.globalRuleList.Update(msg)
	}
	return m, cmd
}

// handleDeleteConfirm handles the delete confirmation overlay.
func (m model) handleDeleteConfirm(key string) (tea.Model, tea.Cmd) {
	if key == "y" {
		switch m.screen {
		case screenPolicyRules:
			if err := repoRemoveRule(m.options, rule{name: m.deleteTarget}); err != nil {
				m.footer = fmt.Sprintf("Error removing rule: %v", err)
			} else {
				m.footer = "Rule removed successfully!"
				m.refreshRules()
			}
		case screenTrustGlobalRules:
			if err := repoRemoveGlobalRule(m.options, globalRule{ruleName: m.deleteTarget}); err != nil {
				m.footer = fmt.Sprintf("Error removing global rule: %v", err)
			} else {
				m.footer = "Global rule removed!"
				m.refreshGlobalRules()
			}
		}
	}
	m.confirmDelete = false
	m.deleteTarget = ""
	return m, nil
}

// handlePolicyFormSubmit handles enter on policy add/edit form screens.
func (m model) handlePolicyFormSubmit() (tea.Model, tea.Cmd) {
	if m.focusIndex < len(m.inputs)-1 {
		// Not on last field yet - just advance focus
		m.cycleFocus("tab")
		return m, nil
	}

	r := rule{
		name:    m.inputs[0].Value(),
		pattern: m.inputs[1].Value(),
		key:     m.inputs[2].Value(),
	}
	authorizedKeys := splitAndTrim(m.inputs[2].Value())

	var err error
	switch m.screen {
	case screenPolicyAddRule:
		err = repoAddRule(m.options, r, authorizedKeys)
	case screenPolicyEditRule:
		err = repoUpdateRule(m.options, r, authorizedKeys)
	}

	if err != nil {
		m.footer = fmt.Sprintf("Error: %v", err)
		return m, nil
	}

	m.refreshRules()
	if m.screen == screenPolicyAddRule {
		m.footer = "Rule added successfully!"
	} else {
		m.footer = "Rule updated successfully!"
	}
	m.screen = screenPolicyRules
	return m, nil
}

// handleGlobalFormSubmit handles enter on global add/edit form screens.
func (m model) handleGlobalFormSubmit() (tea.Model, tea.Cmd) {
	if m.focusIndex < len(m.inputs)-1 {
		m.cycleFocus("tab")
		return m, nil
	}

	parts := splitAndTrim(m.inputs[2].Value())
	thr := 0
	if m.inputs[1].Value() == tuf.GlobalRuleThresholdType {
		thr, _ = strconv.Atoi(m.inputs[3].Value())
	}
	gr := globalRule{
		ruleName:     m.inputs[0].Value(),
		ruleType:     m.inputs[1].Value(),
		rulePatterns: parts,
		threshold:    thr,
	}

	var err error
	switch m.screen {
	case screenTrustAddGlobalRule:
		err = repoAddGlobalRule(m.options, gr)
	case screenTrustEditGlobalRule:
		err = repoUpdateGlobalRule(m.options, gr)
	}

	if err != nil {
		m.footer = fmt.Sprintf("Error: %v", err)
		return m, nil
	}

	m.refreshGlobalRules()
	if m.screen == screenTrustAddGlobalRule {
		m.footer = "Global rule added!"
	} else {
		m.footer = "Global rule updated!"
	}
	m.screen = screenTrustGlobalRules
	return m, nil
}

// handleReorderUp moves the selected rule up in the list.
func (m model) handleReorderUp() (tea.Model, tea.Cmd) {
	if i := m.ruleList.Index(); i > 0 {
		m.rules[i], m.rules[i-1] = m.rules[i-1], m.rules[i]
		if err := repoReorderRules(m.options, m.rules); err != nil {
			m.footer = fmt.Sprintf("Error reordering rules: %v", err)
			return m, nil
		}
		m.updateRuleList()
		m.footer = "Rules reordered successfully!"
	}
	return m, nil
}

// handleReorderDown moves the selected rule down in the list.
func (m model) handleReorderDown() (tea.Model, tea.Cmd) {
	if i := m.ruleList.Index(); i < len(m.rules)-1 {
		m.rules[i], m.rules[i+1] = m.rules[i+1], m.rules[i]
		if err := repoReorderRules(m.options, m.rules); err != nil {
			m.footer = fmt.Sprintf("Error reordering rules: %v", err)
			return m, nil
		}
		m.updateRuleList()
		m.footer = "Rules reordered successfully!"
	}
	return m, nil
}

// cycleFocus moves focus (the cursor) between input fields in form screens.
func (m *model) cycleFocus(key string) {
	if key == "up" || key == "shift+tab" {
		if m.focusIndex > 0 {
			m.focusIndex--
			m.footer = ""
		} else {
			m.focusIndex = len(m.inputs) - 1
		}
	} else {
		if m.focusIndex < len(m.inputs)-1 {
			m.focusIndex++
		} else {
			m.focusIndex = 0
		}
	}

	for i := range m.inputs {
		if i == m.focusIndex {
			m.inputs[i].Focus()
			m.inputs[i].PromptStyle = focusedStyle
			m.inputs[i].TextStyle = focusedStyle
		} else {
			m.inputs[i].Blur()
			m.inputs[i].PromptStyle = blurredStyle
			m.inputs[i].TextStyle = blurredStyle
		}
	}
}

// splitAndTrim splits a comma-separated string and trims whitespace.
func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}
