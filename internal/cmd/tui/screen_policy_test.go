// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

func selectItemByTitle(t *testing.T, l *list.Model, title string) {
	t.Helper()
	for i, it := range l.Items() {
		if item, ok := it.(item); ok && item.title == title {
			l.Select(i)
			return
		}
	}
	t.Fatalf("menu item %q not found", title)
}

func TestPolicyUINavigationMenu(t *testing.T) {
	o := &options{
		readOnly:  true,
		targetRef: "policy",
	}

	m := initialModel(context.Background(), o)
	m.screen = screenPolicy

	// Test View rendering
	viewStr := m.policyScreen.View(&m)
	if !strings.Contains(viewStr, "Home › Policy") {
		t.Errorf("expected view to contain header Home › Policy, got %q", viewStr)
	}

	// Test selecting "View Rules"
	m.screen = screenPolicy
	selectItemByTitle(t, &m.policyScreen.policyScreenList, "View Rules")
	updatedModel, _ := m.policyScreen.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
	resModel := updatedModel.(model)
	if resModel.screen != screenPolicyRules {
		t.Errorf("expected screenPolicyRules (%v), got %v", screenPolicyRules, resModel.screen)
	}

	// Test selecting "Manage Principals"
	m.screen = screenPolicy
	selectItemByTitle(t, &m.policyScreen.policyScreenList, "Manage Principals")
	updatedModel, _ = m.policyScreen.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
	resModel = updatedModel.(model)
	if resModel.screen != screenPolicyPrincipals {
		t.Errorf("expected screenPolicyPrincipals (%v), got %v", screenPolicyPrincipals, resModel.screen)
	}

	// Test selecting "Manage Lifecycle"
	m.screen = screenPolicy
	selectItemByTitle(t, &m.policyScreen.policyScreenList, "Manage Lifecycle")
	updatedModel, _ = m.policyScreen.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
	resModel = updatedModel.(model)
	if resModel.screen != screenPolicyLifecycle {
		t.Errorf("expected screenPolicyLifecycle (%v), got %v", screenPolicyLifecycle, resModel.screen)
	}
}

func TestPolicyUINavigationInteractive(t *testing.T) {
	o := &options{
		readOnly:  true,
		targetRef: "policy",
	}

	m := initialModel(context.Background(), o)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))

	// Wait for home screen Policy option
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return strings.Contains(string(out), "Policy")
	}, teatest.WithCheckInterval(time.Millisecond*50), teatest.WithDuration(time.Second*10))

	// Press Enter to navigate to Policy screen (Policy is 1st item)
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Wait for Policy screen
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return strings.Contains(string(out), "Home › Policy")
	}, teatest.WithCheckInterval(time.Millisecond*50), teatest.WithDuration(time.Second*10))

	// Quit with 'q' from non-form screen
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second*10))
}
