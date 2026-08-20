// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
)

func TestPolicyRulesScreenInitialization(t *testing.T) {
	s := &policyRulesScreen{}

	// Test initRuleInputs
	s.initRuleInputs()
	if len(s.inputs) != 4 {
		t.Fatalf("expected 4 input fields, got %d", len(s.inputs))
	}
	if s.focusIndex != 0 {
		t.Errorf("expected focusIndex 0, got %d", s.focusIndex)
	}

	// Test initRuleInputsPrefilled
	r := rule{
		name:      "rule-1",
		pattern:   "refs/heads/main",
		key:       "key-1",
		threshold: 2,
	}
	s.initRuleInputsPrefilled(r)
	if s.inputs[0].Value() != "rule-1" {
		t.Errorf("expected rule name 'rule-1', got %q", s.inputs[0].Value())
	}
	if s.inputs[1].Value() != "refs/heads/main" {
		t.Errorf("expected pattern 'refs/heads/main', got %q", s.inputs[1].Value())
	}
	if s.inputs[2].Value() != "key-1" {
		t.Errorf("expected key 'key-1', got %q", s.inputs[2].Value())
	}
	if s.inputs[3].Value() != "2" {
		t.Errorf("expected threshold '2', got %q", s.inputs[3].Value())
	}
}

func TestPolicyRulesScreenCycleFocus(t *testing.T) {
	s := &policyRulesScreen{}
	s.initRuleInputs()

	// Initial focus index 0
	if s.focusIndex != 0 {
		t.Errorf("expected initial focus 0, got %d", s.focusIndex)
	}

	// Tab down
	s.cycleFocus("tab")
	if s.focusIndex != 1 {
		t.Errorf("expected focus 1 after tab, got %d", s.focusIndex)
	}

	// Tab up / shift+tab
	s.cycleFocus("shift+tab")
	if s.focusIndex != 0 {
		t.Errorf("expected focus 0 after shift+tab, got %d", s.focusIndex)
	}

	// Up key (wrap to last index 3)
	s.cycleFocus("up")
	if s.focusIndex != 3 {
		t.Errorf("expected focus 3 after up, got %d", s.focusIndex)
	}

	// Down key (wrap to index 0)
	s.cycleFocus("down")
	if s.focusIndex != 0 {
		t.Errorf("expected focus 0 after down, got %d", s.focusIndex)
	}
}

func TestPolicyRulesScreenKeyHandling(t *testing.T) {
	o := &options{
		readOnly:  false,
		targetRef: "policy",
	}

	m := initialModel(context.Background(), o)
	m.screen = screenPolicyRules
	s := &m.policyRulesScreen

	// Setup dummy rules
	s.rules = []rule{
		{name: "rule-a", pattern: "refs/heads/a", key: "key-a", threshold: 1},
		{name: "rule-b", pattern: "refs/heads/b", key: "key-b", threshold: 1},
	}
	s.updateRuleList()

	// Test 'a' key (Add Rule)
	updatedModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}, &m)
	resModel := updatedModel.(model)
	if resModel.screen != screenPolicyAddRule {
		t.Errorf("expected screenPolicyAddRule after pressing 'a', got %v", resModel.screen)
	}

	// Reset screen back
	m.screen = screenPolicyRules

	// Test 'e' key (Edit Rule)
	updatedModel, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")}, &m)
	resModel = updatedModel.(model)
	if resModel.screen != screenPolicyEditRule {
		t.Errorf("expected screenPolicyEditRule after pressing 'e', got %v", resModel.screen)
	}

	// Reset screen back
	m.screen = screenPolicyRules

	// Test 'd' key (Delete Rule trigger)
	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")}, &m)
	if !s.confirmDelete {
		t.Error("expected confirmDelete to be true after pressing 'd'")
	}
	if s.deleteTarget != "rule-a" {
		t.Errorf("expected deleteTarget 'rule-a', got %q", s.deleteTarget)
	}

	// Test cancelling delete confirm (pressing any key except 'y')
	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}, &m)
	if s.confirmDelete {
		t.Error("expected confirmDelete to be false after pressing 'n'")
	}
}

func TestPolicyRulesScreenReorder(t *testing.T) {
	o := &options{
		readOnly:  false,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)
	m.screen = screenPolicyRules
	s := &m.policyRulesScreen

	s.rules = []rule{
		{name: "rule-a", pattern: "refs/heads/a", key: "key-a", threshold: 1},
		{name: "rule-b", pattern: "refs/heads/b", key: "key-b", threshold: 1},
	}
	s.updateRuleList()

	// Test handleReorderDown when at top (index 0)
	s.handleReorderDown(&m)
	if s.rules[0].name != "rule-b" || s.rules[1].name != "rule-a" {
		t.Errorf("expected [rule-b, rule-a] after reorder down, got [%s, %s]", s.rules[0].name, s.rules[1].name)
	}

	// Test handleReorderUp when at index 1
	s.ruleList.Select(1)
	s.handleReorderUp(&m)
	if s.rules[0].name != "rule-a" || s.rules[1].name != "rule-b" {
		t.Errorf("expected [rule-a, rule-b] after reorder up, got [%s, %s]", s.rules[0].name, s.rules[1].name)
	}
}

func TestPolicyRulesScreenFormNavigationAndSubmit(t *testing.T) {
	o := &options{
		readOnly:  false,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)
	m.screen = screenPolicyAddRule
	s := &m.policyRulesScreen
	s.initRuleInputs()

	// Pressing Enter on field 0 should advance focus to field 1
	if s.focusIndex != 0 {
		t.Fatalf("expected focusIndex 0, got %d", s.focusIndex)
	}

	s.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
	if s.focusIndex != 1 {
		t.Errorf("expected focusIndex 1 after Enter on field 0, got %d", s.focusIndex)
	}

	// Move focus to last field (field 3)
	s.focusIndex = 3

	// Pressing Enter on last field triggers handlePolicyFormSubmit
	s.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
	// Since repoAddRule fails without git repo, error dialog should open
	if m.errorDialog == nil {
		t.Error("expected errorDialog to open when repoAddRule fails without git repo")
	}
}

func TestPolicyRulesScreenView(t *testing.T) {
	o := &options{
		readOnly:  true,
		targetRef: "policy",
	}

	m := initialModel(context.Background(), o)
	s := &m.policyRulesScreen

	// Test View on screenPolicyRules with no rules
	m.screen = screenPolicyRules
	viewStr := s.View(&m)
	if !strings.Contains(viewStr, "Home › Policy › Rules") {
		t.Errorf("expected view to contain header, got %q", viewStr)
	}

	// Test View on screenPolicyRules with delete overlay confirm
	s.confirmDelete = true
	s.deleteTarget = "test-rule"
	viewStr = s.View(&m)
	if !strings.Contains(viewStr, "Delete rule \"test-rule\"?") {
		t.Errorf("expected view to contain delete overlay, got %q", viewStr)
	}
	s.confirmDelete = false

	// Test View on screenPolicyAddRule
	m.screen = screenPolicyAddRule
	s.initRuleInputs()
	viewStr = s.View(&m)
	if !strings.Contains(viewStr, "Add Rule") {
		t.Errorf("expected view to contain 'Add Rule', got %q", viewStr)
	}

	// Test View on screenPolicyEditRule
	m.screen = screenPolicyEditRule
	viewStr = s.View(&m)
	if !strings.Contains(viewStr, "Edit Rule") {
		t.Errorf("expected view to contain 'Edit Rule', got %q", viewStr)
	}
}

func TestPolicyRulesScreenReadOnlyKeyIgnore(t *testing.T) {
	o := &options{
		readOnly:  true,
		targetRef: "policy",
	}

	m := initialModel(context.Background(), o)
	m.readOnly = true
	m.screen = screenPolicyRules
	s := &m.policyRulesScreen
	s.rules = []rule{{name: "rule-a", pattern: "refs/heads/a", key: "key-a", threshold: 1}}
	s.ruleList = list.New([]list.Item{item{title: "rule-a"}}, list.NewDefaultDelegate(), 80, 20)

	// Pressing 'a' in read-only mode should be ignored
	updatedModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}, &m)
	resModel := updatedModel.(model)
	if resModel.screen != screenPolicyRules {
		t.Errorf("expected screen to stay screenPolicyRules in readOnly mode, got %v", resModel.screen)
	}
}
