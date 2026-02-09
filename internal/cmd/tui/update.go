// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

// This file handles TUI input handling.

package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gittuf/gittuf/internal/tuf"
)

// Update updates the model based on the message received
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := lipgloss.NewStyle().Margin(1, 2).GetFrameSize()
		m.choiceList.SetSize(msg.Width-h, msg.Height-v)
		m.policyScreenList.SetSize(msg.Width-h, msg.Height-v)
		m.trustScreenList.SetSize(msg.Width-h, msg.Height-v)
		m.ruleList.SetSize(msg.Width-h, msg.Height-v)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "left":
			m.footer = ""
			switch m.screen {
			case screenPolicy, screenTrust:
				m.screen = screenChoice
			case screenAddRule, screenRemoveRule, screenListRules, screenReorderRules:
				m.screen = screenPolicy
			case screenAddGlobalRule, screenRemoveGlobalRule, screenUpdateGlobalRule, screenListGlobalRules:
				m.screen = screenTrust
			}
			return m, nil
		case "enter":
			switch m.screen {
			case screenChoice:
				i, ok := m.choiceList.SelectedItem().(item)
				if ok {
					switch i.title {
					case "Policy":
						m.screen = screenPolicy
					case "Trust":
						m.screen = screenTrust
					}
				}
			case screenPolicy:
				i, ok := m.policyScreenList.SelectedItem().(item)
				if ok {
					switch i.title {
					case "Add Rule":
						m.screen = screenAddRule
						m.focusIndex = 0
						m.inputs[0].Focus()
					case "Remove Rule":
						m.screen = screenRemoveRule
						m.updateRuleList()
					case "List Rules":
						m.screen = screenListRules
					case "Reorder Rules":
						m.screen = screenReorderRules
						m.updateRuleList()
					}
				}
			case screenTrust:
				i, ok := m.trustScreenList.SelectedItem().(item)
				if ok {
					switch i.title {
					case "Add Global Rule":
						m.screen = screenAddGlobalRule
						m.initGlobalInputs()
					case "Remove Global Rule":
						m.screen = screenRemoveGlobalRule
						m.updateGlobalRuleList()
					case "Update Global Rule":
						m.screen = screenUpdateGlobalRule
						m.initGlobalInputs()
					case "List Global Rules":
						m.screen = screenListGlobalRules
						m.updateGlobalRuleList()
					}
				}
			case screenAddRule:
				if m.focusIndex == len(m.inputs)-1 {
					newRule := rule{
						name:    m.inputs[0].Value(),
						pattern: m.inputs[1].Value(),
						key:     m.inputs[2].Value(),
					}
					authorizedKeys := []string{m.inputs[2].Value()}
					err := repoAddRule(m.options, newRule, authorizedKeys)
					if err != nil {
						m.footer = fmt.Sprintf("Error adding rule: %v", err)
						return m, nil
					}
					m.rules = append(m.rules, newRule)
					m.updateRuleList()
					m.footer = "Rule added successfully!"
					m.screen = screenPolicy
				}
			case screenRemoveRule:
				if i, ok := m.ruleList.SelectedItem().(item); ok {
					err := repoRemoveRule(m.options, rule{name: i.title})
					if err != nil {
						m.footer = fmt.Sprintf("Error removing rule: %v", err)
						return m, nil
					}
					for idx, rule := range m.rules {
						if rule.name == i.title {
							m.rules = append(m.rules[:idx], m.rules[idx+1:]...)
							break
						}
					}
					m.updateRuleList()
					m.footer = "Rule removed successfully!"
					m.screen = screenPolicy
				}
			case screenAddGlobalRule:
				// parse comma-separated input into []string
				if m.focusIndex == len(m.inputs)-1 {
					raw := m.inputs[2].Value()
					parts := strings.Split(raw, ",")
					for i := range parts {
						parts[i] = strings.TrimSpace(parts[i])
					}
					// parse threshold only if that type
					thr := 0
					if m.inputs[1].Value() == tuf.GlobalRuleThresholdType {
						thr, _ = strconv.Atoi(m.inputs[3].Value())
					}
					newGR := globalRule{
						ruleName:     m.inputs[0].Value(),
						ruleType:     m.inputs[1].Value(),
						rulePatterns: parts,
						threshold:    thr,
					}
					if err := repoAddGlobalRule(m.options, newGR); err != nil {
						m.footer = fmt.Sprintf("Error: %v", err)
						return m, nil
					}
					m.globalRules = append(m.globalRules, newGR)
					m.updateGlobalRuleList()
					m.footer = "Global rule added!"
					m.screen = screenTrust
				}
			case screenRemoveGlobalRule:
				if sel, ok := m.globalRuleList.SelectedItem().(item); ok {
					err := repoRemoveGlobalRule(m.options, globalRule{ruleName: sel.title})
					if err != nil {
						m.footer = fmt.Sprintf("Error removing global rule: %v", err)
						return m, nil
					}
					for idx, gr := range m.globalRules {
						if gr.ruleName == sel.title {
							m.globalRules = append(m.globalRules[:idx], m.globalRules[idx+1:]...)
							break
						}
					}
					m.updateGlobalRuleList()
					m.footer = "Global rule removed!"
					m.screen = screenTrust
				}
			case screenUpdateGlobalRule:
				if m.focusIndex == len(m.inputs)-1 {
					// parse namespaces (split + TrimSpace)
					parts := strings.Split(m.inputs[2].Value(), ",")
					for i := range parts {
						parts[i] = strings.TrimSpace(parts[i])
					}
					// parse threshold if needed
					thr := 0
					if m.inputs[1].Value() == tuf.GlobalRuleThresholdType {
						thr, _ = strconv.Atoi(m.inputs[3].Value())
					}
					updated := globalRule{
						ruleName:     m.inputs[0].Value(),
						ruleType:     m.inputs[1].Value(),
						rulePatterns: parts,
						threshold:    thr,
					}
					if err := repoUpdateGlobalRule(m.options, updated); err != nil {
						m.footer = fmt.Sprintf("Error updating global rule: %v", err)
						return m, nil
					}
					for idx, gr := range m.globalRules {
						if gr.ruleName == updated.ruleName {
							m.globalRules[idx] = updated
							break
						}
					}
					m.updateGlobalRuleList()
					m.footer = "Global rule updated!"
					m.screen = screenTrust
				}
			}
		case "u":
			if m.screen == screenReorderRules {
				if i := m.ruleList.Index(); i > 0 {
					m.rules[i], m.rules[i-1] = m.rules[i-1], m.rules[i]
					if err := repoReorderRules(m.options, m.rules); err != nil {
						m.footer = fmt.Sprintf("Error reordering rules: %v", err)
						return m, nil
					}
					m.updateRuleList()
					m.footer = "Rules reordered successfully!"
				}
			}
		case "d":
			if m.screen == screenReorderRules {
				if i := m.ruleList.Index(); i < len(m.rules)-1 {
					m.rules[i], m.rules[i+1] = m.rules[i+1], m.rules[i]
					if err := repoReorderRules(m.options, m.rules); err != nil {
						m.footer = fmt.Sprintf("Error reordering rules: %v", err)
						return m, nil
					}
					m.updateRuleList()
					m.footer = "Rules reordered successfully!"
				}
			}
		case "tab", "shift+tab", "up", "down":
			if m.screen == screenAddRule || m.screen == screenAddGlobalRule || m.screen == screenUpdateGlobalRule {
				s := msg.String()
				if s == "up" || s == "shift+tab" {
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

				for i := 0; i <= len(m.inputs)-1; i++ {
					if i == m.focusIndex {
						m.inputs[i].Focus()
						m.inputs[i].PromptStyle = focusedStyle
						m.inputs[i].TextStyle = focusedStyle
						continue
					}
					m.inputs[i].Blur()
					m.inputs[i].PromptStyle = blurredStyle
					m.inputs[i].TextStyle = blurredStyle
				}
				return m, nil
			}
		}
	}

	switch m.screen {
	case screenChoice:
		m.choiceList, cmd = m.choiceList.Update(msg)
	case screenPolicy:
		m.policyScreenList, cmd = m.policyScreenList.Update(msg)
	case screenTrust:
		m.trustScreenList, cmd = m.trustScreenList.Update(msg)
	case screenAddRule:
		m.inputs[m.focusIndex], cmd = m.inputs[m.focusIndex].Update(msg)
	case screenRemoveRule, screenReorderRules:
		m.ruleList, cmd = m.ruleList.Update(msg)
	case screenAddGlobalRule, screenUpdateGlobalRule:
		m.inputs[m.focusIndex], cmd = m.inputs[m.focusIndex].Update(msg)
	case screenListGlobalRules, screenRemoveGlobalRule:
		m.globalRuleList, cmd = m.globalRuleList.Update(msg)
	}

	return m, cmd
}
