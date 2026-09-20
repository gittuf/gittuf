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

	// Test small screen dimensions (boxWidth/boxHeight < 0 branch)
	m.width = 1
	m.height = 1
	s.View(&m)
}
