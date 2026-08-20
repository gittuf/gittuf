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
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
)

func TestPolicyPrincipalsFormScreenInitialization(t *testing.T) {
	f := &policyPrincipalsFormScreen{}

	// Test initInputs for Add Standalone Key(s)
	f.initInputs("Add Standalone Key(s)")
	if len(f.inputs) != 1 {
		t.Fatalf("expected 1 input for standalone key, got %d", len(f.inputs))
	}
	if f.action != "Add Standalone Key(s)" {
		t.Errorf("expected action 'Add Standalone Key(s)', got %q", f.action)
	}

	// Test initInputs for Add Person
	f.initInputs("Add Person")
	if len(f.inputs) != 4 {
		t.Fatalf("expected 4 inputs for person, got %d", len(f.inputs))
	}

	// Test initInputsPrefilled for Person
	p := &tufv02.Person{
		PersonID: "alice",
		PublicKeys: map[string]*tufv02.Key{
			"key-1": {KeyID: "key-1"},
		},
		AssociatedIdentities: map[string]string{
			"github": "alice123",
		},
		Custom: map[string]string{
			"role": "admin",
		},
	}

	f.initInputsPrefilled(p)
	if f.inputs[0].Value() != "alice" {
		t.Errorf("expected personID 'alice', got %q", f.inputs[0].Value())
	}
	if !strings.Contains(f.inputs[1].Value(), "key-1") {
		t.Errorf("expected public keys to contain 'key-1', got %q", f.inputs[1].Value())
	}
	if !strings.Contains(f.inputs[2].Value(), "github::alice123") {
		t.Errorf("expected identity 'github::alice123', got %q", f.inputs[2].Value())
	}
	if !strings.Contains(f.inputs[3].Value(), "role=admin") {
		t.Errorf("expected custom metadata 'role=admin', got %q", f.inputs[3].Value())
	}
}

func TestPolicyPrincipalsFormScreenFocusCycle(t *testing.T) {
	f := &policyPrincipalsFormScreen{}
	f.initInputs("Add Person")

	if f.focusIndex != 0 {
		t.Errorf("expected focus 0, got %d", f.focusIndex)
	}

	// Cycle focus down / tab
	f.cycleFocus("tab")
	if f.focusIndex != 1 {
		t.Errorf("expected focus 1, got %d", f.focusIndex)
	}

	// Cycle focus up / shift+tab
	f.cycleFocus("shift+tab")
	if f.focusIndex != 0 {
		t.Errorf("expected focus 0, got %d", f.focusIndex)
	}

	// Cycle up (wrap to 3)
	f.cycleFocus("up")
	if f.focusIndex != 3 {
		t.Errorf("expected focus 3, got %d", f.focusIndex)
	}

	// Cycle down (wrap to 0)
	f.cycleFocus("down")
	if f.focusIndex != 0 {
		t.Errorf("expected focus 0, got %d", f.focusIndex)
	}
}

func TestPolicyPrincipalsScreenChoiceAndNavigation(t *testing.T) {
	o := &options{
		readOnly:  false,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)
	s := &m.policyPrincipalsScreen
	m.screen = screenPolicyPrincipals

	// Pressing 'a' toggles addChoice
	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}, &m)
	if !s.addChoice {
		t.Error("expected addChoice to be true after pressing 'a'")
	}

	// Test choice '1' (Add Person)
	updatedModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")}, &m)
	resModel := updatedModel.(model)
	if resModel.screen != screenPolicyPrincipalsForm {
		t.Errorf("expected screenPolicyPrincipalsForm, got %v", resModel.screen)
	}
	if resModel.policyPrincipalsFormScreen.action != "Add Person" {
		t.Errorf("expected action 'Add Person', got %q", resModel.policyPrincipalsFormScreen.action)
	}

	// Reset back to choice menu
	s.addChoice = true
	m.screen = screenPolicyPrincipals

	// Test choice '2' (Add Standalone Key)
	updatedModel, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")}, &m)
	resModel = updatedModel.(model)
	if resModel.policyPrincipalsFormScreen.action != "Add Standalone Key(s)" {
		t.Errorf("expected action 'Add Standalone Key(s)', got %q", resModel.policyPrincipalsFormScreen.action)
	}

	// Reset back and test esc
	s.addChoice = true
	s.Update(tea.KeyMsg{Type: tea.KeyEsc}, &m)
	if s.addChoice {
		t.Error("expected addChoice to be false after pressing Esc")
	}
}

func TestPolicyPrincipalsScreenDeleteConfirm(t *testing.T) {
	o := &options{
		readOnly:  false,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)
	s := &m.policyPrincipalsScreen
	m.screen = screenPolicyPrincipals

	p := &tufv02.Person{PersonID: "alice"}
	s.principals = []tuf.Principal{p}
	s.list = list.New([]list.Item{item{title: "alice"}}, list.NewDefaultDelegate(), 80, 20)

	// Pressing 'd' triggers confirm delete
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

func TestPolicyPrincipalsFormValidationErrors(t *testing.T) {
	o := &options{
		readOnly:  false,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)
	f := &m.policyPrincipalsFormScreen
	m.screen = screenPolicyPrincipalsForm

	// 1. Missing Person ID error
	f.initInputs("Add Person")
	f.focusIndex = 3
	f.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
	if !strings.Contains(m.errorMsg, "Principal ID is required") {
		t.Errorf("expected error for missing principal ID, got %q", m.errorMsg)
	}

	// 2. Invalid Associated Identity format
	m.errorMsg = ""
	f.initInputs("Add Person")
	f.inputs[0].SetValue("alice")
	f.inputs[2].SetValue("invalididentityformat")
	f.focusIndex = 3
	f.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
	if !strings.Contains(m.errorMsg, "invalid format for associated identity") {
		t.Errorf("expected error for invalid identity, got %q", m.errorMsg)
	}

	// 3. Invalid Custom Metadata format
	m.errorMsg = ""
	f.initInputs("Add Person")
	f.inputs[0].SetValue("alice")
	f.inputs[3].SetValue("invalidmetadataformat")
	f.focusIndex = 3
	f.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
	if !strings.Contains(m.errorMsg, "invalid format for custom metadata") {
		t.Errorf("expected error for invalid custom metadata, got %q", m.errorMsg)
	}

	// 4. Missing Standalone Public Keys error
	m.errorMsg = ""
	f.initInputs("Add Standalone Key(s)")
	f.focusIndex = 0
	f.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
	if !strings.Contains(m.errorMsg, "At least one public key is required") {
		t.Errorf("expected error for missing standalone key, got %q", m.errorMsg)
	}
}

func TestPolicyPrincipalsScreenViewRendering(t *testing.T) {
	o := &options{
		readOnly:  true,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)
	s := &m.policyPrincipalsScreen
	f := &m.policyPrincipalsFormScreen
	m.screen = screenPolicyPrincipals

	// Test main screen view with empty list
	viewStr := s.View(&m)
	if !strings.Contains(viewStr, "Home › Policy › Principals") {
		t.Errorf("expected header in view, got %q", viewStr)
	}

	// Test choice popup rendering
	s.addChoice = true
	viewStr = s.View(&m)
	if !strings.Contains(viewStr, "Add Principal") {
		t.Errorf("expected choice popup in view, got %q", viewStr)
	}
	s.addChoice = false

	// Test updatePrincipalsList
	p := &tufv02.Person{
		PersonID: "alice",
		PublicKeys: map[string]*tufv02.Key{
			"key-1": {KeyID: "key-1"},
		},
		AssociatedIdentities: map[string]string{
			"github": "alice123",
		},
		Custom: map[string]string{
			"role": "admin",
		},
	}
	s.principals = []tuf.Principal{p}
	s.updatePrincipalsList()

	// Test delete confirm overlay
	s.confirmDelete = true
	s.deleteTarget = "alice"
	viewStr = s.View(&m)
	if !strings.Contains(viewStr, "Delete rule \"alice\"?") {
		t.Errorf("expected delete overlay in view, got %q", viewStr)
	}

	// Test Form Screen view
	f.initInputs("Add Person")
	viewStr = f.View(&m)
	if !strings.Contains(viewStr, "Home › Policy › Principals › Add Person") {
		t.Errorf("expected form header in view, got %q", viewStr)
	}
}

func TestPolicyPrincipalsScreenReadOnlyAddGuard(t *testing.T) {
	o := &options{
		readOnly:  true,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)
	s := &m.policyPrincipalsScreen
	m.screen = screenPolicyPrincipals

	// Pressing 'a' in read-only mode should not open addChoice popup
	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}, &m)
	if s.addChoice {
		t.Error("expected addChoice to remain false in read-only mode")
	}
}

func TestPolicyPrincipalsScreenEditStandaloneKey(t *testing.T) {
	o := &options{
		readOnly:  false,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)
	s := &m.policyPrincipalsScreen
	m.screen = screenPolicyPrincipals

	key := &tufv02.Key{KeyID: "key-1"}
	s.principals = []tuf.Principal{key}
	s.updatePrincipalsList()

	// Pressing 'e' on a standalone key should show error and stay on screenPolicyPrincipals
	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")}, &m)
	if m.screen != screenPolicyPrincipals {
		t.Errorf("expected screen to remain screenPolicyPrincipals, got %v", m.screen)
	}
	if !strings.Contains(m.errorMsg, "Standalone keys cannot be edited") {
		t.Errorf("expected error message for standalone key edit, got %q", m.errorMsg)
	}

	// Pressing 'a' should clear the stale errorMsg and open addChoice popup
	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}, &m)
	if !s.addChoice {
		t.Error("expected addChoice to be true after pressing 'a'")
	}
	if m.errorMsg != "" {
		t.Errorf("expected errorMsg to be cleared after pressing 'a', got %q", m.errorMsg)
	}
}
