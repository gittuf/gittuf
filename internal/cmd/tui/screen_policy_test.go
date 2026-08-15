// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

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

	// Test selecting "View Rules" (first item, index 0)
	updatedModel, _ := m.policyScreen.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
	resModel := updatedModel.(model)
	if resModel.screen != screenPolicyRules {
		t.Errorf("expected screenPolicyRules, got %v", resModel.screen)
	}

	// Test selecting "Manage Principals" (second item, index 1)
	m.policyScreen.policyScreenList.Select(1)
	updatedModel, _ = m.policyScreen.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
	resModel = updatedModel.(model)
	if resModel.screen != screenPolicyPrincipals {
		t.Errorf("expected screenPolicyPrincipals, got %v", resModel.screen)
	}

	// Test selecting "Manage Lifecycle" (third item, index 2)
	m.policyScreen.policyScreenList.Select(2)
	updatedModel, _ = m.policyScreen.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
	resModel = updatedModel.(model)
	if resModel.screen != screenPolicyLifecycle {
		t.Errorf("expected screenPolicyLifecycle, got %v", resModel.screen)
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
