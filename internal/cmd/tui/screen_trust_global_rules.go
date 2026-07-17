// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gittuf/gittuf/internal/tuf"
)

type trustGlobalRulesScreen struct {
	globalRules    []globalRule
	globalRuleList list.Model
	inputs         []textinput.Model
	focusIndex     int
	confirmDelete  bool
	deleteTarget   string
}

func (s *trustGlobalRulesScreen) refreshGlobalRules(ctx context.Context, o *options) {
	s.globalRules = getGlobalRules(ctx, o)
	s.updateGlobalRuleList()
}

func (s *trustGlobalRulesScreen) updateGlobalRuleList() {
	items := make([]list.Item, len(s.globalRules))
	for i, gr := range s.globalRules {
		desc := fmt.Sprintf("Type: %s\nNamespaces: %s", gr.ruleType, strings.Join(gr.rulePatterns, ", "))
		if gr.ruleType == tuf.GlobalRuleThresholdType {
			desc += fmt.Sprintf("\nThreshold: %d", gr.threshold)
		}
		items[i] = item{title: gr.ruleName, desc: desc}
	}
	s.globalRuleList.SetItems(items)
}

func (s *trustGlobalRulesScreen) initGlobalRuleInputs() {
	s.inputs = initInputs([]inputField{
		{"Enter Global Rule Name Here", "Rule Name:"},
		{"Enter Global Rule Type (threshold|block-force-pushes)", "Type:"},
		{"Enter Namespaces (comma-separated)", "Namespaces:"},
		{"Enter Threshold (if threshold type)", "Threshold:"},
	})
	s.focusIndex = 0
}

func (s *trustGlobalRulesScreen) initGlobalRuleInputsPrefilled(gr globalRule) {
	s.initGlobalRuleInputs()
	s.inputs[0].SetValue(gr.ruleName)
	s.inputs[1].SetValue(gr.ruleType)
	s.inputs[2].SetValue(strings.Join(gr.rulePatterns, ", "))
	if gr.ruleType == tuf.GlobalRuleThresholdType {
		s.inputs[3].SetValue(fmt.Sprintf("%d", gr.threshold))
	}
}

func (s *trustGlobalRulesScreen) cycleFocus(key string) {
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

func (s *trustGlobalRulesScreen) handleEsc(m *model) bool {
	switch m.screen {
	case screenTrustGlobalRules:
		if s.confirmDelete {
			s.confirmDelete = false
			s.deleteTarget = ""
			return true
		}

		m.screen = screenTrust
		return true
	case screenTrustAddGlobalRule, screenTrustEditGlobalRule:
		m.screen = screenTrustGlobalRules
		return true
	default:
		return false
	}
}

func (s *trustGlobalRulesScreen) Update(msg tea.Msg, m *model) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	if s.confirmDelete {
		return s.handleDeleteConfirm(msg, m)
	}

	switch m.screen {
	case screenTrustGlobalRules:
		if msg, ok := msg.(tea.KeyMsg); ok {
			if !m.readOnly {
				switch msg.String() {
				case "a":
					s.initGlobalRuleInputs()
					m.screen = screenTrustAddGlobalRule
					return *m, nil
				case "e":
					if sel, ok := s.globalRuleList.SelectedItem().(item); ok {
						for _, gr := range s.globalRules {
							if gr.ruleName == sel.title {
								s.initGlobalRuleInputsPrefilled(gr)
								m.screen = screenTrustEditGlobalRule
								return *m, nil
							}
						}
					}
				case "d":
					if sel, ok := s.globalRuleList.SelectedItem().(item); ok {
						s.confirmDelete = true
						s.deleteTarget = sel.title
						return *m, nil
					}
				}
			}
		}
		s.globalRuleList, cmd = s.globalRuleList.Update(msg)
		return *m, cmd

	case screenTrustAddGlobalRule, screenTrustEditGlobalRule:
		if msg, ok := msg.(tea.KeyMsg); ok {
			switch msg.String() {
			case "enter":
				return s.handleGlobalFormSubmit(m)
			case "tab", "shift+tab", "up", "down":
				s.cycleFocus(msg.String())
				m.footer = ""
				return *m, nil
			}
		}
		s.inputs[s.focusIndex], cmd = s.inputs[s.focusIndex].Update(msg)
		return *m, cmd
	}

	return *m, nil
}

func (s *trustGlobalRulesScreen) handleDeleteConfirm(msg tea.Msg, m *model) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.String() == "y" {
			if err := repoRemoveGlobalRule(m.ctx, m.options, globalRule{ruleName: s.deleteTarget}); err != nil {
				m.errorMsg = fmt.Sprintf("Error removing global rule: %v", err)
			} else {
				m.footer = "Global rule removed!"
				s.refreshGlobalRules(m.ctx, m.options)
			}
		}
		s.confirmDelete = false
		s.deleteTarget = ""
	}
	return *m, nil
}

func (s *trustGlobalRulesScreen) handleGlobalFormSubmit(m *model) (tea.Model, tea.Cmd) {
	if s.focusIndex < len(s.inputs)-1 {
		s.cycleFocus("tab")
		return *m, nil
	}

	parts := splitAndTrim(s.inputs[2].Value())
	thr := 0
	if s.inputs[1].Value() == tuf.GlobalRuleThresholdType {
		thr, _ = strconv.Atoi(s.inputs[3].Value())
	}
	gr := globalRule{
		ruleName:     s.inputs[0].Value(),
		ruleType:     s.inputs[1].Value(),
		rulePatterns: parts,
		threshold:    thr,
	}

	var err error
	switch m.screen {
	case screenTrustAddGlobalRule:
		err = repoAddGlobalRule(m.ctx, m.options, gr)
	case screenTrustEditGlobalRule:
		err = repoUpdateGlobalRule(m.ctx, m.options, gr)
	}

	if err != nil {
		m.errorMsg = fmt.Sprintf("Error: %v", err)
		return *m, nil
	}

	s.refreshGlobalRules(m.ctx, m.options)
	if m.screen == screenTrustAddGlobalRule {
		m.footer = "Global rule added!"
	} else {
		m.footer = "Global rule updated!"
	}
	m.screen = screenTrustGlobalRules
	return *m, nil
}

func (s *trustGlobalRulesScreen) View(m *model) string {
	switch m.screen {
	case screenTrustGlobalRules:
		overlay := ""
		if s.confirmDelete {
			overlay = "\n" + renderDeleteOverlay(s.deleteTarget) + "\n"
		}
		hint := ""
		if !m.readOnly {
			hint = "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color(colorSubtext)).Render(
				"Run `gittuf trust apply` to apply staged changes to the selected policy file.",
			)
		}

		listView := m.renderListOrEmpty(s.globalRuleList, len(s.globalRules), "No global rules configured")
		overlays := overlay + renderActionHints(m.readOnly) + hint
		return m.renderScreen("Home › Trust › Global Rules", listView, overlays)

	case screenTrustAddGlobalRule:
		return s.renderFormScreen(m, "Add Global Rule", "Home › Trust › Global Rules › Add")

	case screenTrustEditGlobalRule:
		return s.renderFormScreen(m, "Edit Global Rule", "Home › Trust › Global Rules › Edit")
	}

	return ""
}

func (s *trustGlobalRulesScreen) renderFormScreen(m *model, formTitle string, breadcrumb string) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(formTitle) + "\n\n")
	for _, input := range s.inputs {
		b.WriteString(input.View() + "\n")
	}
	b.WriteString("\n" + "Press Tab to advance, Enter to advance/submit, and Esc to go back." + "\n")
	b.WriteString(renderFooterBox(*m))
	b.WriteString(renderErrorMsg(m.errorMsg))
	return lipgloss.JoinVertical(lipgloss.Left,
		renderStatusBar(breadcrumb, m.readOnly, m.width),
		renderWithMargin(b.String()),
	)
}
