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
	"github.com/gittuf/gittuf/internal/tuf"
)

func TestTrustGlobalRulesScreenInitialization(t *testing.T) {
	s := &trustGlobalRulesScreen{}

	// Test initGlobalRuleInputs
	s.initGlobalRuleInputs()
	if len(s.inputs) != 4 {
		t.Fatalf("expected 4 inputs for global rule form, got %d", len(s.inputs))
	}

	// Test initGlobalRuleInputsPrefilled
	gr := globalRule{
		ruleName:     "threshold-rule",
		ruleType:     tuf.GlobalRuleThresholdType,
		rulePatterns: []string{"refs/heads/main"},
		threshold:    2,
	}

	s.initGlobalRuleInputsPrefilled(gr)
	if s.inputs[0].Value() != "threshold-rule" {
		t.Errorf("expected ruleName 'threshold-rule', got %q", s.inputs[0].Value())
	}
	if s.inputs[1].Value() != tuf.GlobalRuleThresholdType {
		t.Errorf("expected ruleType 'threshold', got %q", s.inputs[1].Value())
	}
	if s.inputs[2].Value() != "refs/heads/main" {
		t.Errorf("expected pattern 'refs/heads/main', got %q", s.inputs[2].Value())
	}
	if s.inputs[3].Value() != "2" {
		t.Errorf("expected threshold '2', got %q", s.inputs[3].Value())
	}
}

func TestTrustGlobalRulesScreenFocusCycle(t *testing.T) {
	s := &trustGlobalRulesScreen{}
	s.initGlobalRuleInputs()

	if s.focusIndex != 0 {
		t.Errorf("expected focus 0, got %d", s.focusIndex)
	}

	// Cycle tab
	s.cycleFocus("tab")
	if s.focusIndex != 1 {
		t.Errorf("expected focus 1, got %d", s.focusIndex)
	}

	// Cycle up
	s.cycleFocus("up")
	if s.focusIndex != 0 {
		t.Errorf("expected focus 0, got %d", s.focusIndex)
	}

	// Cycle up (wrap to 3)
	s.cycleFocus("up")
	if s.focusIndex != 3 {
		t.Errorf("expected focus 3, got %d", s.focusIndex)
	}
}

func TestTrustGlobalRulesScreenHandleEscNavigation(t *testing.T) {
	o := &options{
		readOnly:  false,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)
	s := &m.trustGlobalRulesScreen

	// Case 1: confirmDelete active in screenTrustGlobalRules
	m.screen = screenTrustGlobalRules
	s.confirmDelete = true
	s.deleteTarget = "test-rule"
	handled := s.handleEsc(&m)
	if !handled || s.confirmDelete {
		t.Error("expected handleEsc to clear confirmDelete and return true")
	}

	// Case 2: Esc from screenTrustGlobalRules returns to screenTrust
	handled = s.handleEsc(&m)
	if !handled || m.screen != screenTrust {
		t.Errorf("expected screenTrust on Esc, got %v", m.screen)
	}

	// Case 3: Esc from Add/Edit form returns to screenTrustGlobalRules
	m.screen = screenTrustAddGlobalRule
	handled = s.handleEsc(&m)
	if !handled || m.screen != screenTrustGlobalRules {
		t.Errorf("expected screenTrustGlobalRules on Esc, got %v", m.screen)
	}
}

func TestTrustGlobalRulesScreenKeyActions(t *testing.T) {
	o := &options{
		readOnly:  false,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)
	s := &m.trustGlobalRulesScreen
	m.screen = screenTrustGlobalRules

	s.globalRules = []globalRule{
		{ruleName: "rule-1", ruleType: tuf.GlobalRuleThresholdType, rulePatterns: []string{"refs/heads/*"}, threshold: 1},
	}
	s.globalRuleList = list.New([]list.Item{item{title: "rule-1"}}, list.NewDefaultDelegate(), 80, 20)

	// Press 'a' (Add Global Rule)
	updatedModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}, &m)
	resModel := updatedModel.(model)
	if resModel.screen != screenTrustAddGlobalRule {
		t.Errorf("expected screenTrustAddGlobalRule, got %v", resModel.screen)
	}

	// Press 'e' (Edit Global Rule)
	m.screen = screenTrustGlobalRules
	updatedModel, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")}, &m)
	resModel = updatedModel.(model)
	if resModel.screen != screenTrustEditGlobalRule {
		t.Errorf("expected screenTrustEditGlobalRule, got %v", resModel.screen)
	}

	// Press 'd' (Delete Global Rule)
	m.screen = screenTrustGlobalRules
	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")}, &m)
	if !s.confirmDelete {
		t.Error("expected confirmDelete to be true after pressing 'd'")
	}

	// Cancel delete
	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}, &m)
	if s.confirmDelete {
		t.Error("expected confirmDelete to be false after pressing 'n'")
	}
}

func TestTrustGlobalRulesScreenFormNavigationAndSubmit(t *testing.T) {
	o := &options{
		readOnly:  false,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)
	s := &m.trustGlobalRulesScreen
	m.screen = screenTrustAddGlobalRule
	s.initGlobalRuleInputs()

	// Enter on field 0 advances focus to 1
	if s.focusIndex != 0 {
		t.Fatalf("expected focusIndex 0, got %d", s.focusIndex)
	}
	s.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
	if s.focusIndex != 1 {
		t.Errorf("expected focusIndex 1 after Enter on field 0, got %d", s.focusIndex)
	}

	// Focus on last field (field 3) and submit form
	s.focusIndex = 3
	s.inputs[0].SetValue("my-global-rule")
	s.inputs[1].SetValue(tuf.GlobalRuleThresholdType)
	s.inputs[2].SetValue("refs/heads/*")
	s.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
	if m.screen != screenTrustAddGlobalRule {
		t.Errorf("expected screen to remain screenTrustAddGlobalRule on failed submission, got %v", m.screen)
	}
	if m.errorDialog == nil {
		t.Error("expected errorDialog on failed submission, got nil")
	} else if m.errorDialog.title != "Add Global Rule Failed" {
		t.Errorf("expected errorDialog title 'Add Global Rule Failed', got %q", m.errorDialog.title)
	}
}

func TestTrustGlobalRulesScreenViewRendering(t *testing.T) {
	o := &options{
		readOnly:  true,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)
	s := &m.trustGlobalRulesScreen

	// Test updateGlobalRuleList
	s.globalRules = []globalRule{
		{ruleName: "rule-1", ruleType: tuf.GlobalRuleThresholdType, rulePatterns: []string{"refs/heads/*"}, threshold: 1},
	}
	s.updateGlobalRuleList()

	// Test Main List View
	m.screen = screenTrustGlobalRules
	viewStr := s.View(&m)
	if !strings.Contains(viewStr, "Home › Trust › Global Rules") {
		t.Errorf("expected header in list view, got %q", viewStr)
	}

	// Test Add Form View
	m.screen = screenTrustAddGlobalRule
	viewStr = s.View(&m)
	if !strings.Contains(viewStr, "Add Global Rule") {
		t.Errorf("expected title in add form view, got %q", viewStr)
	}

	// Test Edit Form View
	m.screen = screenTrustEditGlobalRule
	viewStr = s.View(&m)
	if !strings.Contains(viewStr, "Edit Global Rule") {
		t.Errorf("expected title in edit form view, got %q", viewStr)
	}
}
