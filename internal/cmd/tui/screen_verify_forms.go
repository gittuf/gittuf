// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	verifyopts "github.com/gittuf/gittuf/experimental/gittuf/options/verify"
	verifymergeableopts "github.com/gittuf/gittuf/experimental/gittuf/options/verifymergeable"
)

//
// Verify Reference Form
//

type verifyRefScreen struct {
	form formComponent
}

func (s *verifyRefScreen) reset() {
	s.form = newFormComponent([]inputField{
		{"Target Reference (e.g. refs/heads/main)", "Target Ref: "},
	})
}

func (s *verifyRefScreen) Update(msg tea.Msg, m *model) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "tab", "shift+tab", "up", "down":
			m.footer = ""
		}
	}
	cmd, isSubmit := s.form.Update(msg)
	if isSubmit {
		return s.handleSubmit(m)
	}
	return *m, cmd
}

func (s *verifyRefScreen) View(m *model) string {
	var b strings.Builder
	b.WriteString("Verify a specific reference against gittuf policies\n\n")
	b.WriteString(s.form.View())
	b.WriteString("\n\nPress Enter to submit.")
	return m.renderScreen("Home › Verify › Verify Reference", b.String(), renderActionHints(m.readOnly))
}

func (s *verifyRefScreen) handleSubmit(m *model) (tea.Model, tea.Cmd) {
	targetRef := strings.TrimSpace(s.form.inputs[0].Value())
	if targetRef == "" {
		m.errorMsg = "Target reference is required"
		return *m, nil
	}

	m.verifying = true
	m.logsBuf.Reset()
	m.logViewport.SetContent("")
	m.loadingMsg = fmt.Sprintf("Verifying reference %s...", targetRef)

	pr, pw := io.Pipe()
	logger := slog.New(slog.NewTextHandler(pw, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)

	scanner := bufio.NewScanner(pr)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	m.logCh = make(chan string, 4096)

	return *m, tea.Batch(
		m.spinner.Tick,
		collectLogsCmd(scanner, m.logCh),
		logFlushTick(m.logCh),
		func() tea.Msg {
			defer pw.Close()

			opts := []verifyopts.Option{}
			err := m.repo.VerifyRef(m.ctx, targetRef, opts...)

			// Reset logger
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

			successMsg := fmt.Sprintf("Successfully verified %s!", targetRef)
			return verifyResultMsg{err: err, successMsg: successMsg}
		},
	)
}

//
// Verify Mergeability Form
//

type verifyMergeableScreen struct {
	form formComponent
}

func (s *verifyMergeableScreen) reset() {
	s.form = newFormComponent([]inputField{
		{"Feature Branch (e.g. refs/heads/feature)", "Feature Branch: "},
		{"Base Branch (e.g. refs/heads/main)", "Base Branch: "},
	})
}

func (s *verifyMergeableScreen) Update(msg tea.Msg, m *model) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "tab", "shift+tab", "up", "down":
			m.footer = ""
		}
	}
	cmd, isSubmit := s.form.Update(msg)
	if isSubmit {
		return s.handleSubmit(m)
	}
	return *m, cmd
}

func (s *verifyMergeableScreen) View(m *model) string {
	var b strings.Builder
	b.WriteString("Verify if a feature branch can be merged into a base branch\n\n")
	b.WriteString(s.form.View())
	b.WriteString("\n\nPress Tab to advance, Enter to submit.")
	return m.renderScreen("Home › Verify › Verify Mergeability", b.String(), renderActionHints(m.readOnly))
}

func (s *verifyMergeableScreen) handleSubmit(m *model) (tea.Model, tea.Cmd) {
	featureBranch := strings.TrimSpace(s.form.inputs[0].Value())
	baseBranch := strings.TrimSpace(s.form.inputs[1].Value())

	if featureBranch == "" || baseBranch == "" {
		m.errorMsg = "Both Feature Branch and Base Branch are required"
		return *m, nil
	}

	m.verifying = true
	m.logsBuf.Reset()
	m.logViewport.SetContent("")
	m.loadingMsg = fmt.Sprintf("Verifying if %s is mergeable into %s...", featureBranch, baseBranch)

	pr, pw := io.Pipe()
	logger := slog.New(slog.NewTextHandler(pw, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)

	scanner := bufio.NewScanner(pr)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	m.logCh = make(chan string, 4096)

	return *m, tea.Batch(
		m.spinner.Tick,
		collectLogsCmd(scanner, m.logCh),
		logFlushTick(m.logCh),
		func() tea.Msg {
			defer pw.Close()
			opts := []verifymergeableopts.Option{}
			mergeable, err := m.repo.VerifyMergeable(m.ctx, baseBranch, featureBranch, opts...)

			// Reset logger
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

			if err == nil && !mergeable {
				err = fmt.Errorf("branch %s is NOT mergeable into %s", featureBranch, baseBranch)
			}

			successMsg := fmt.Sprintf("Successfully verified %s is mergeable into %s!", featureBranch, baseBranch)
			return verifyResultMsg{err: err, successMsg: successMsg}
		},
	)
}

// formComponent represents a generic form with text inputs.
type formComponent struct {
	inputs     []textinput.Model
	focusIndex int
}

// newFormComponent initializes a new form component with the given fields.
func newFormComponent(fields []inputField) formComponent {
	return formComponent{
		inputs:     initInputs(fields),
		focusIndex: 0,
	}
}

// cycleFocus changes the focused input field based on the key pressed.
func (f *formComponent) cycleFocus(key string) {
	if len(f.inputs) <= 1 {
		return
	}

	if key == "up" || key == "shift+tab" {
		if f.focusIndex > 0 {
			f.focusIndex--
		} else {
			f.focusIndex = len(f.inputs) - 1
		}
	} else {
		if f.focusIndex < len(f.inputs)-1 {
			f.focusIndex++
		} else {
			f.focusIndex = 0
		}
	}

	for i := range f.inputs {
		if i == f.focusIndex {
			f.inputs[i].Focus()
			f.inputs[i].PromptStyle = focusedStyle
			f.inputs[i].TextStyle = focusedStyle
		} else {
			f.inputs[i].Blur()
			f.inputs[i].PromptStyle = blurredStyle
			f.inputs[i].TextStyle = blurredStyle
		}
	}
}

// Update handles common key bindings for a form and updates the inputs.
// It returns a command and a boolean indicating if the form was submitted (Enter pressed).
func (f *formComponent) Update(msg tea.Msg) (tea.Cmd, bool) {
	var cmd tea.Cmd

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "enter":
			return nil, true
		case "tab", "shift+tab", "up", "down":
			f.cycleFocus(keyMsg.String())
			return nil, false
		}
	}

	if len(f.inputs) > 0 {
		f.inputs[f.focusIndex], cmd = f.inputs[f.focusIndex].Update(msg)
	}

	return cmd, false
}

// View renders the form fields.
func (f *formComponent) View() string {
	var b strings.Builder

	for i := range f.inputs {
		b.WriteString(f.inputs[i].View())
		if i < len(f.inputs)-1 {
			b.WriteRune('\n')
		}
	}

	return b.String()
}
