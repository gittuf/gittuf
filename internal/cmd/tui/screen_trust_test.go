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
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
)

func TestTrustScreenMenuSelectionAndRendering(t *testing.T) {
	o := &options{
		readOnly:  true,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)
	m.screen = screenTrust
	s := &m.trustScreen

	// Test View rendering
	viewStr := s.View(&m)
	if !strings.Contains(viewStr, "Home › Trust") {
		t.Errorf("expected view to contain header Home › Trust, got %q", viewStr)
	}

	// Test pressing Enter to select View Global Rules
	updatedModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
	resModel := updatedModel.(model)
	if resModel.screen != screenTrustGlobalRules {
		t.Errorf("expected screenTrustGlobalRules, got %v", resModel.screen)
	}
}

func TestTrustScreenInteractiveNavigation(t *testing.T) {
	o := &options{
		readOnly:  true,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))

	// Wait for home screen Policy option
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return strings.Contains(string(out), "Policy")
	}, teatest.WithCheckInterval(time.Millisecond*50), teatest.WithDuration(time.Second*10))

	// Move down to Trust option and press Enter
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	time.Sleep(time.Millisecond * 50)
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Wait for Trust screen
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return strings.Contains(string(out), "Home › Trust")
	}, teatest.WithCheckInterval(time.Millisecond*50), teatest.WithDuration(time.Second*10))

	// Quit with 'q' from non-form screen
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second*10))
}
