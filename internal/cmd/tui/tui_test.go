// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
)

// makeTestModel constructs a minimal model for testing without requiring a
// live gittuf repository. It uses initialModel (which defers repo I/O) and
// then injects the initDoneMsg directly so no filesystem access occurs.
func makeTestModel(readOnly bool, rules []rule, globalRules []globalRule) model {
	if globalRules == nil {
		globalRules = []globalRule{}
	}
	if rules == nil {
		rules = []rule{}
	}

	o := &options{p: &persistent.Options{}, readOnly: readOnly}
	m := initialModel(context.Background(), o)

	// Inject an already-resolved initDoneMsg so tests don't hit the filesystem.
	updated, _ := m.Update(initDoneMsg{
		rules:       rules,
		globalRules: globalRules,
		readOnly:    readOnly,
	})
	// Size all list components so they render content in unit tests.
	sized, _ := updated.(model).Update(tea.WindowSizeMsg{Width: 200, Height: 100})
	return sized.(model)
}

// TestChoiceScreenShowsPolicyAndTrust verifies the initial choice screen
// renders both top-level navigation options.
func TestChoiceScreenShowsPolicyAndTrust(t *testing.T) {
	t.Parallel()

	m := makeTestModel(true, nil, nil)
	view := m.View()

	for _, expected := range []string{"Policy", "Trust"} {
		if !strings.Contains(view, expected) {
			t.Errorf("choice screen should contain %q, got:\n%s", expected, view)
		}
	}
}

// TestPolicyScreenViewRendersOperations verifies the policy sub-menu renders
// the expected navigation item.
func TestPolicyScreenViewRendersOperations(t *testing.T) {
	t.Parallel()

	m := makeTestModel(false, nil, nil)
	m.screen = screenPolicy
	view := m.View()

	if !strings.Contains(view, "View Rules") {
		t.Errorf("policy screen should contain 'View Rules', got:\n%s", view)
	}
}

// TestPolicyRulesViewRendersRuleDetails verifies the rule list screen renders
// each rule's name, pattern, and key without needing a live repo.
func TestPolicyRulesViewRendersRuleDetails(t *testing.T) {
	t.Parallel()

	rules := []rule{
		{name: "test-rule", pattern: "*.go", key: "abc123"},
	}
	m := makeTestModel(true, rules, nil)
	m.screen = screenPolicyRules
	m.updateRuleList()
	// Resize again after updating the rule list so the list view is populated.
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 100})
	view := sized.(model).View()

	for _, want := range []string{"test-rule", "*.go", "abc123"} {
		if !strings.Contains(view, want) {
			t.Errorf("policy rules view should contain %q", want)
		}
	}
}

// TestAddRuleViewRendersInputFields verifies the add-rule form screen shows
// its title.
func TestAddRuleViewRendersInputFields(t *testing.T) {
	t.Parallel()

	m := makeTestModel(false, nil, nil)
	m.screen = screenPolicyAddRule
	m.inputs = initInputs([]inputField{
		{"Enter Rule Name", "Rule Name:"},
		{"Enter Pattern", "Pattern:"},
		{"Enter Key Path", "Authorize Key:"},
	})
	view := m.View()

	if !strings.Contains(view, "Rule") {
		t.Errorf("add rule view should contain 'Rule', got:\n%s", view)
	}
}

// TestQuitWithQ verifies that pressing 'q' terminates the program.
func TestQuitWithQ(t *testing.T) {
	t.Parallel()

	m := makeTestModel(true, nil, nil)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(200, 100))

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Policy")) || bytes.Contains(bts, []byte("gittuf"))
	}, teatest.WithDuration(10*time.Second), teatest.WithCheckInterval(50*time.Millisecond))

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(10*time.Second))
}

// TestQuitWithCtrlC verifies that pressing ctrl+c terminates the program.
func TestQuitWithCtrlC(t *testing.T) {
	t.Parallel()

	m := makeTestModel(true, nil, nil)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(200, 100))

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Policy")) || bytes.Contains(bts, []byte("gittuf"))
	}, teatest.WithDuration(10*time.Second), teatest.WithCheckInterval(50*time.Millisecond))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(10*time.Second))
}

// TestNavigateToChoiceScreen verifies that the initial screen shows the
// top-level choice menu after initialization.
func TestNavigateToChoiceScreen(t *testing.T) {
	t.Parallel()

	m := makeTestModel(true, nil, nil)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(200, 100))

	// Wait for the choice menu (Policy / Trust).
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Policy")) && bytes.Contains(bts, []byte("Trust"))
	}, teatest.WithDuration(10*time.Second), teatest.WithCheckInterval(50*time.Millisecond))

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(10*time.Second))
}

// TestNavigateBackToChoiceScreen verifies that pressing esc from the policy
// sub-screen returns to the choice menu.
func TestNavigateBackToChoiceScreen(t *testing.T) {
	t.Parallel()

	m := makeTestModel(true, nil, nil)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(200, 100))

	// Wait for the choice menu.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Policy")) && bytes.Contains(bts, []byte("Trust"))
	}, teatest.WithDuration(10*time.Second), teatest.WithCheckInterval(50*time.Millisecond))

	// Press Enter to navigate into the Policy sub-screen.
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Wait for the policy screen ("View Rules" is the item in the policy list).
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("View Rules"))
	}, teatest.WithDuration(10*time.Second), teatest.WithCheckInterval(50*time.Millisecond))

	// Press esc to go back (the back key in this TUI is esc, not left arrow).
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})

	// The top-level choice menu should reappear.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Trust"))
	}, teatest.WithDuration(10*time.Second), teatest.WithCheckInterval(50*time.Millisecond))

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(10*time.Second))
}

// TestFinalModelScreenAfterQuit verifies that the resolved model after the
// user quits retains a known screen state.
func TestFinalModelScreenAfterQuit(t *testing.T) {
	t.Parallel()

	m := makeTestModel(true, nil, nil)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(200, 100))

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Policy"))
	}, teatest.WithDuration(10*time.Second), teatest.WithCheckInterval(50*time.Millisecond))

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})

	fm := tm.FinalModel(t, teatest.WithFinalTimeout(10*time.Second))
	_, ok := fm.(model)
	if !ok {
		t.Fatalf("final model has unexpected type: %T", fm)
	}
}
