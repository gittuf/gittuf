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

func TestVerifyUINavigation(t *testing.T) {
	o := &options{
		readOnly:  true,
		targetRef: "policy",
	}

	m := initialModel(context.Background(), o)

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))

	// Wait for home screen
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return strings.Contains(string(out), "Policy")
	}, teatest.WithCheckInterval(time.Millisecond*50), teatest.WithDuration(time.Second*10))

	// Send Down arrow to move to Trust
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	time.Sleep(time.Millisecond * 50)

	// Send Down arrow to move to Verify
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	time.Sleep(time.Millisecond * 50)

	// Enter Verify screen
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Wait for Verify menu screen
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return strings.Contains(string(out), "Home › Verify")
	}, teatest.WithCheckInterval(time.Millisecond*50), teatest.WithDuration(time.Second*10))

	// Enter Verify Reference form
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Wait for Verify Reference form screen
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		s := string(out)
		return strings.Contains(s, "Verify Reference") || strings.Contains(s, "Target Ref")
	}, teatest.WithCheckInterval(time.Millisecond*50), teatest.WithDuration(time.Second*10))

	// Send ctrl+c to quit (since 'q' is captured by form text inputs)
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second*10))
}
