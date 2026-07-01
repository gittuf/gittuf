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
	"github.com/gittuf/gittuf/experimental/gittuf"
)

type rule struct {
	name      string
	pattern   string
	key       string
	threshold int
}

type policyRulesScreen struct {
	rules         []rule
	ruleList      list.Model
	inputs        []textinput.Model
	focusIndex    int
	confirmDelete bool
	deleteTarget  string
}

func (s *policyRulesScreen) refreshRules(ctx context.Context, o *options) {
	s.rules = getCurrRules(ctx, o)
	s.updateRuleList()
}

func (s *policyRulesScreen) updateRuleList() {
	items := make([]list.Item, len(s.rules))
	for i, r := range s.rules {
		items[i] = item{title: r.name, desc: fmt.Sprintf("Pattern: %s, Key: %s, Threshold: %d", r.pattern, r.key, r.threshold)}
	}
	s.ruleList.SetItems(items)
}

func (s *policyRulesScreen) initRuleInputs() {
	s.inputs = initInputs([]inputField{
		{"Enter Rule Name Here", "Rule Name:"},
		{"Enter Rule Pattern Here", "Rule Pattern:"},
		{"Enter Principal IDs Here (comma-separated)", "Authorized Principals:"},
		{"Enter Threshold", "Threshold:"},
	})
	s.focusIndex = 0
}

func (s *policyRulesScreen) initRuleInputsPrefilled(r rule) {
	s.initRuleInputs()
	s.inputs[0].SetValue(r.name)
	s.inputs[1].SetValue(r.pattern)
	s.inputs[2].SetValue(r.key)
	s.inputs[3].SetValue(fmt.Sprintf("%d", r.threshold))
}

func (s *policyRulesScreen) cycleFocus(key string) {
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

func (s *policyRulesScreen) Update(msg tea.Msg, m *model) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	if s.confirmDelete {
		return s.handleDeleteConfirm(msg, m)
	}

	switch m.screen {
	case screenPolicyRules:
		if msg, ok := msg.(tea.KeyMsg); ok {
			if !m.readOnly {
				switch msg.String() {
				case "a":
					s.initRuleInputs()
					m.screen = screenPolicyAddRule
					return *m, nil
				case "e":
					if sel, ok := s.ruleList.SelectedItem().(item); ok {
						for _, r := range s.rules {
							if r.name == sel.title {
								s.initRuleInputsPrefilled(r)
								m.screen = screenPolicyEditRule
								return *m, nil
							}
						}
					}
				case "d":
					if sel, ok := s.ruleList.SelectedItem().(item); ok {
						s.confirmDelete = true
						s.deleteTarget = sel.title
						return *m, nil
					}
				case "k":
					return s.handleReorderUp(m)
				case "j":
					return s.handleReorderDown(m)
				}
			}
		}
		s.ruleList, cmd = s.ruleList.Update(msg)
		return *m, cmd

	case screenPolicyAddRule, screenPolicyEditRule:
		if msg, ok := msg.(tea.KeyMsg); ok {
			switch msg.String() {
			case "enter":
				return s.handlePolicyFormSubmit(m)
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

func (s *policyRulesScreen) handleDeleteConfirm(msg tea.Msg, m *model) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.String() == "y" {
			if err := repoRemoveRule(m.ctx, m.options, rule{name: s.deleteTarget}); err != nil {
				m.errorMsg = fmt.Sprintf("Error removing rule: %v", err)
			} else {
				m.footer = "Rule removed successfully!"
				s.refreshRules(m.ctx, m.options)
			}
		}
		s.confirmDelete = false
		s.deleteTarget = ""
	}
	return *m, nil
}

func (s *policyRulesScreen) handleReorderUp(m *model) (tea.Model, tea.Cmd) {
	if i := s.ruleList.Index(); i > 0 {
		s.rules[i], s.rules[i-1] = s.rules[i-1], s.rules[i]
		if err := repoReorderRules(m.ctx, m.options, s.rules); err != nil {
			m.errorMsg = fmt.Sprintf("Error reordering rules: %v", err)
			return *m, nil
		}
		s.updateRuleList()
		m.footer = "Rules reordered successfully!"
	}
	return *m, nil
}

func (s *policyRulesScreen) handleReorderDown(m *model) (tea.Model, tea.Cmd) {
	if i := s.ruleList.Index(); i < len(s.rules)-1 {
		s.rules[i], s.rules[i+1] = s.rules[i+1], s.rules[i]
		if err := repoReorderRules(m.ctx, m.options, s.rules); err != nil {
			m.errorMsg = fmt.Sprintf("Error reordering rules: %v", err)
			return *m, nil
		}
		s.updateRuleList()
		m.footer = "Rules reordered successfully!"
	}
	return *m, nil
}

func (s *policyRulesScreen) handlePolicyFormSubmit(m *model) (tea.Model, tea.Cmd) {
	if s.focusIndex < len(s.inputs)-1 {
		s.cycleFocus("tab")
		return *m, nil
	}

	thr, _ := strconv.Atoi(s.inputs[3].Value())
	r := rule{
		name:      s.inputs[0].Value(),
		pattern:   s.inputs[1].Value(),
		key:       s.inputs[2].Value(),
		threshold: thr,
	}
	authorizedKeys := splitAndTrim(s.inputs[2].Value())
	protectedNamespaces := splitAndTrim(s.inputs[1].Value())

	var err error
	switch m.screen {
	case screenPolicyAddRule:
		err = repoAddRule(m.ctx, m.options, r, authorizedKeys, protectedNamespaces)
	case screenPolicyEditRule:
		err = repoUpdateRule(m.ctx, m.options, r, authorizedKeys, protectedNamespaces)
	}

	if err != nil {
		m.errorMsg = fmt.Sprintf("Error: %v", err)
		return *m, nil
	}

	s.refreshRules(m.ctx, m.options)
	if m.screen == screenPolicyAddRule {
		m.footer = "Rule added successfully!"
	} else {
		m.footer = "Rule updated successfully!"
	}
	m.screen = screenPolicyRules
	return *m, nil
}

func (s *policyRulesScreen) View(m *model) string {
	switch m.screen {
	case screenPolicyRules:
		overlay := ""
		if s.confirmDelete {
			overlay = "\n" + renderDeleteOverlay(s.deleteTarget) + "\n"
		}
		hint := ""
		if !m.readOnly {
			hint = "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color(colorSubtext)).Render(
				"Run `gittuf policy apply` to apply staged changes to the selected policy file.",
			)
		}

		listView := m.renderListOrEmpty(s.ruleList, len(s.rules), "No rules configured")
		overlays := overlay + renderActionHints(m.readOnly) + hint

		return m.renderScreen("Home › Policy › Rules", listView, overlays)

	case screenPolicyAddRule:
		return s.renderFormScreen(m, "Add Rule", "Home › Policy › Rules › Add")

	case screenPolicyEditRule:
		return s.renderFormScreen(m, "Edit Rule", "Home › Policy › Rules › Edit")
	}
	return ""
}

func (s *policyRulesScreen) renderFormScreen(m *model, formTitle string, breadcrumb string) string {
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

// getCurrRules returns the current rules from the policy file.
func getCurrRules(ctx context.Context, o *options) []rule {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return nil
	}

	rules, err := repo.ListRules(ctx, o.targetRef)
	if err != nil {
		return nil
	}

	var currRules = make([]rule, len(rules))
	for i, r := range rules {
		currRules[i] = rule{
			name:      r.Delegation.ID(),
			pattern:   strings.Join(r.Delegation.GetProtectedNamespaces(), ", "),
			key:       strings.Join(r.Delegation.GetPrincipalIDs().Contents(), ", "),
			threshold: r.Delegation.GetThreshold(),
		}
	}
	return currRules
}

// repoAddRule adds a rule to the policy file.
func repoAddRule(ctx context.Context, o *options, rule rule, authorizedPrincipalIDs []string, protectedNamespaces []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	return repo.AddDelegation(ctx, signer, o.policyName, rule.name, authorizedPrincipalIDs, protectedNamespaces, rule.threshold, true)
}

// repoUpdateRule updates an existing rule in the policy file.
func repoUpdateRule(ctx context.Context, o *options, r rule, authorizedPrincipalIDs []string, protectedNamespaces []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	return repo.UpdateDelegation(ctx, signer, o.policyName, r.name, authorizedPrincipalIDs, protectedNamespaces, r.threshold, true)
}

// repoRemoveRule removes a rule from the policy file.
func repoRemoveRule(ctx context.Context, o *options, rule rule) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}
	return repo.RemoveDelegation(ctx, signer, o.policyName, rule.name, true)
}

// repoReorderRules reorders the rules in the policy file.
func repoReorderRules(ctx context.Context, o *options, rules []rule) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	ruleNames := make([]string, len(rules))
	for i, r := range rules {
		ruleNames[i] = r.name
	}

	return repo.ReorderDelegations(ctx, signer, o.policyName, ruleNames, true)
}
