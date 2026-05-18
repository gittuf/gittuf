// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestTUI(t *testing.T) {
	tmpDir := t.TempDir()
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(currentDir) //nolint:errcheck

	gitinterface.CreateTestGitRepository(t, tmpDir, false)

	t.Run("Start and Quit", func(t *testing.T) {
		o := &options{
			readOnly:  true,
			targetRef: "policy",
		}

		m := initialModel(context.Background(), o)

		tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))
		// Wait for main menu to render so startup initialization has completed
		teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
			return strings.Contains(string(out), "Policy")
		}, teatest.WithCheckInterval(time.Millisecond*100), teatest.WithDuration(time.Second*15))

		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
		tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second*15))
	})

	t.Run("Policy UI Navigation", func(t *testing.T) {
		o := &options{
			readOnly:  true,
			targetRef: "policy",
		}

		m := initialModel(context.Background(), o)

		tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))

		// Wait for main menu to render
		teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
			return strings.Contains(string(out), "Policy")
		}, teatest.WithCheckInterval(time.Millisecond*100), teatest.WithDuration(time.Second*15))

		// Select "Policy" (already selected by default, so just press enter)
		tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

		// Now we should be on the Policy Operations screen.
		teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
			return strings.Contains(string(out), "gittuf Policy Operations")
		}, teatest.WithCheckInterval(time.Millisecond*100), teatest.WithDuration(time.Second*15))

		// Select "View Rules" (already selected by default)
		tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

		// Now we should be on the Policy Rules screen.
		// We check for the "Policy Rules" title OR the screen-specific empty-state message.
		teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
			content := string(out)
			return strings.Contains(content, "Policy Rules") || strings.Contains(content, "No rules configured. Press 'A' to add one.")
		}, teatest.WithCheckInterval(time.Millisecond*100), teatest.WithDuration(time.Second*15))

		// Send "q" to quit
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
		tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second*15))
	})
}

func TestSplitAndTrim(t *testing.T) {
	tests := map[string]struct {
		input    string
		expected []string
	}{
		"comma separated values": {
			input:    "a, b, c",
			expected: []string{"a", "b", "c"},
		},
		"single value": {
			input:    "a",
			expected: []string{"a"},
		},
		"values with extra spaces": {
			input:    " a ,b, c ",
			expected: []string{"a", "b", "c"},
		},
		"empty string": {
			input:    "",
			expected: []string{""},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			result := splitAndTrim(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
