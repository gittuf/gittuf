// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
)

func TestHelpScreenUpdateAndNavigation(t *testing.T) {
	o := &options{
		readOnly:  false,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)
	s := &m.helpScreen

	// Store previous screen as screenPolicy
	s.previousScreen = screenPolicy
	m.screen = screenHelp

	// Test pressing 'h' to return to previous screen
	updatedModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")}, &m)
	resModel := updatedModel.(model)
	if resModel.screen != screenPolicy {
		t.Errorf("expected screen to return to screenPolicy on 'h', got %v", resModel.screen)
	}

	// Test pressing 'esc' to return to previous screen
	s.previousScreen = screenChoice
	m.screen = screenHelp
	updatedModel, _ = s.Update(tea.KeyMsg{Type: tea.KeyEsc}, &m)
	resModel = updatedModel.(model)
	if resModel.screen != screenChoice {
		t.Errorf("expected screen to return to screenChoice on 'esc', got %v", resModel.screen)
	}
}

func TestHelpScreenViewRendering(t *testing.T) {
	o := &options{
		readOnly:  false,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)
	m.width = 80
	m.height = 24
	s := &m.helpScreen

	viewStr := s.View(&m)
	if !strings.Contains(viewStr, "Help") {
		t.Errorf("expected view to contain 'Help', got %q", viewStr)
	}
	if !strings.Contains(viewStr, "Navigate") {
		t.Errorf("expected view to contain navigation help, got %q", viewStr)
	}
	for _, want := range []string{"- a : Add", "- e : Edit", "- d : Delete", "- v : Review staged diff", "- h : Toggle help"} {
		if !strings.Contains(viewStr, want) {
			t.Errorf("expected default edit-mode help to contain %q, got %q", want, viewStr)
		}
	}

	// Test small screen dimensions (boxWidth/boxHeight < 0 branch)
	m.width = 1
	m.height = 1
	s.View(&m)
}

func TestHelpScreenHidesWriteShortcutsInReadOnlyMode(t *testing.T) {
	m := initialModel(context.Background(), &options{
		readOnly:  true,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	})
	m.readOnly = true
	m.width = 90
	m.height = 28

	for _, prev := range []screen{
		screenLoading,
		screenChoice,
		screenPolicy,
		screenPolicyRules,
		screenPolicyPrincipals,
		screenTrustGlobalRules,
	} {
		m.helpScreen.previousScreen = prev
		viewStr := m.helpScreen.View(&m)

		for _, forbidden := range []string{"- a :", "- e :", "- d :", "Reorder rules"} {
			if strings.Contains(viewStr, forbidden) {
				t.Errorf("read-only help (prev=%v) must not contain %q, got %q", prev, forbidden, viewStr)
			}
		}
		for _, want := range []string{"- ↑ ↓ : Navigate", "- v : Review staged diff", "- h : Toggle help", "- esc : Back", "- q : Quit"} {
			if !strings.Contains(viewStr, want) {
				t.Errorf("read-only help (prev=%v) missing %q, got %q", prev, want, viewStr)
			}
		}
	}
}

func TestHelpScreenContextAwareEditMode(t *testing.T) {
	m := initialModel(context.Background(), &options{
		readOnly:  false,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	})
	m.readOnly = false
	m.width = 90
	m.height = 28

	tests := []struct {
		name     string
		prev     screen
		mustHave []string
		mustOmit []string
	}{
		{
			name:     "policy rules screen",
			prev:     screenPolicyRules,
			mustHave: []string{"- a : Add rule", "- e : Edit rule", "- d : Delete rule", "- j / k : Reorder rules"},
			mustOmit: []string{"Add principal", "Add global rule"},
		},
		{
			name:     "policy principals screen",
			prev:     screenPolicyPrincipals,
			mustHave: []string{"- a : Add principal", "- e : Edit principal", "- d : Delete principal"},
			mustOmit: []string{"Add rule", "Reorder rules"},
		},
		{
			name:     "trust global rules screen",
			prev:     screenTrustGlobalRules,
			mustHave: []string{"- a : Add global rule", "- e : Edit global rule", "- d : Delete global rule"},
			mustOmit: []string{"- a : Add rule", "Reorder rules"},
		},
		{
			name:     "menu screen omits CRUD keys",
			prev:     screenPolicy,
			mustHave: []string{"- Enter : Select", "- v : Review staged diff", "- h : Toggle help"},
			mustOmit: []string{"- a :", "- e :", "- d :", "Reorder rules"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m.helpScreen.previousScreen = tc.prev
			viewStr := m.helpScreen.View(&m)
			for _, want := range tc.mustHave {
				if !strings.Contains(viewStr, want) {
					t.Errorf("expected %q in help view, got %q", want, viewStr)
				}
			}
			for _, omit := range tc.mustOmit {
				if strings.Contains(viewStr, omit) {
					t.Errorf("did not expect %q in help view, got %q", omit, viewStr)
				}
			}
		})
	}
}
