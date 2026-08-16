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
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
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

	// Test selecting "View Global Rules"
	selectItemByTitle(t, &s.trustScreenList, "View Global Rules")
	updatedModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
	resModel := updatedModel.(model)
	if resModel.screen != screenTrustGlobalRules {
		t.Errorf("expected screenTrustGlobalRules (%v), got %v", screenTrustGlobalRules, resModel.screen)
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

	// Dynamically calculate number of Down presses to reach "Trust"
	var trustIndex int
	for i, it := range m.homeScreen.choiceList.Items() {
		if it.(item).title == "Trust" {
			trustIndex = i
			break
		}
	}

	for i := 0; i < trustIndex; i++ {
		tm.Send(tea.KeyMsg{Type: tea.KeyDown})
		time.Sleep(time.Millisecond * 50)
	}

	// Press Enter to enter Trust screen
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Wait for Trust screen
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return strings.Contains(string(out), "Home › Trust")
	}, teatest.WithCheckInterval(time.Millisecond*50), teatest.WithDuration(time.Second*10))

	// Quit with 'q' from non-form screen
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second*10))
}
