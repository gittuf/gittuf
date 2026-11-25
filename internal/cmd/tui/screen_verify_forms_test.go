// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	verifyopts "github.com/gittuf/gittuf/experimental/gittuf/options/verify"
)

func TestVerifyRefScreenValidation(t *testing.T) {
	o := &options{
		readOnly:  true,
		targetRef: "policy",
	}

	m := initialModel(context.Background(), o)
	m.screen = screenVerifyRefForm
	m.verifyRefScreen.reset()

	// Initially no error message
	if m.errorMsg != "" {
		t.Fatalf("expected empty errorMsg, got %q", m.errorMsg)
	}

	// Submit empty form (press Enter)
	updatedModel, _ := m.verifyRefScreen.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
	resModel := updatedModel.(model)

	if resModel.errorMsg != "Target reference is required" {
		t.Errorf("expected errorMsg %q, got %q", "Target reference is required", resModel.errorMsg)
	}
}

func TestVerifyMergeableScreenValidation(t *testing.T) {
	t.Run("Empty Submission", func(t *testing.T) {
		o := &options{
			readOnly:  true,
			targetRef: "policy",
		}

		m := initialModel(context.Background(), o)
		m.screen = screenVerifyMergeableForm
		m.verifyMergeableScreen.reset()

		// Submit empty form (press Enter)
		updatedModel, _ := m.verifyMergeableScreen.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
		resModel := updatedModel.(model)

		if resModel.errorMsg != "Both Feature Branch and Base Branch are required" {
			t.Errorf("expected errorMsg %q, got %q", "Both Feature Branch and Base Branch are required", resModel.errorMsg)
		}
	})

	t.Run("Partial Submission", func(t *testing.T) {
		o := &options{
			readOnly:  true,
			targetRef: "policy",
		}

		m := initialModel(context.Background(), o)
		m.screen = screenVerifyMergeableForm
		m.verifyMergeableScreen.reset()

		// Fill only feature branch field
		m.verifyMergeableScreen.form.inputs[0].SetValue("feature-branch")

		// Submit partial form (press Enter)
		updatedModel, _ := m.verifyMergeableScreen.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
		resModel := updatedModel.(model)

		if resModel.errorMsg != "Both Feature Branch and Base Branch are required" {
			t.Errorf("expected errorMsg %q, got %q", "Both Feature Branch and Base Branch are required", resModel.errorMsg)
		}
	})
}

func TestVerifyRefScreenViewAndKeyHandling(t *testing.T) {
	o := &options{
		readOnly:  true,
		targetRef: "policy",
	}

	m := initialModel(context.Background(), o)
	m.screen = screenVerifyRefForm
	m.verifyRefScreen.reset()

	// Test View rendering
	viewStr := m.verifyRefScreen.View(&m)
	if !strings.Contains(viewStr, "Verify a specific reference against gittuf policies") {
		t.Errorf("expected view to contain header, got %q", viewStr)
	}

	// Test Update key handling (tab clears footer)
	m.footer = "some footer"
	m.verifyRefScreen.Update(tea.KeyMsg{Type: tea.KeyTab}, &m)
	if m.footer != "" {
		t.Errorf("expected footer to be cleared on tab, got %q", m.footer)
	}
}

func TestVerifyMergeableScreenViewAndKeyHandling(t *testing.T) {
	o := &options{
		readOnly:  true,
		targetRef: "policy",
	}

	m := initialModel(context.Background(), o)
	m.screen = screenVerifyMergeableForm
	m.verifyMergeableScreen.reset()

	// Test View rendering
	viewStr := m.verifyMergeableScreen.View(&m)
	if !strings.Contains(viewStr, "Verify if a feature branch can be merged") {
		t.Errorf("expected view to contain header, got %q", viewStr)
	}

	// Test Update key handling (down clears footer and cycles focus)
	m.footer = "some footer"
	m.verifyMergeableScreen.Update(tea.KeyMsg{Type: tea.KeyDown}, &m)
	if m.footer != "" {
		t.Errorf("expected footer to be cleared on down, got %q", m.footer)
	}
}

func TestFormComponentCycleFocus(t *testing.T) {
	fields := []inputField{
		{"Field 1", "P1: "},
		{"Field 2", "P2: "},
	}

	form := newFormComponent(fields)

	if form.focusIndex != 0 {
		t.Errorf("expected initial focusIndex 0, got %d", form.focusIndex)
	}

	// Cycle down / tab
	form.cycleFocus("tab")
	if form.focusIndex != 1 {
		t.Errorf("expected focusIndex 1 after tab, got %d", form.focusIndex)
	}

	// Cycle down again (should wrap to 0)
	form.cycleFocus("tab")
	if form.focusIndex != 0 {
		t.Errorf("expected focusIndex 0 after wrapping tab, got %d", form.focusIndex)
	}

	// Cycle up / shift+tab (should wrap to 1)
	form.cycleFocus("shift+tab")
	if form.focusIndex != 1 {
		t.Errorf("expected focusIndex 1 after shift+tab, got %d", form.focusIndex)
	}

	// Cycle up again (should go to 0)
	form.cycleFocus("up")
	if form.focusIndex != 0 {
		t.Errorf("expected focusIndex 0 after up, got %d", form.focusIndex)
	}

	// Test form View() rendering
	formView := form.View()
	if formView == "" {
		t.Error("expected non-empty form View()")
	}
}

func TestBuildVerifyRefOptions(t *testing.T) {
	apply := func(remoteRefName string, latestOnly bool) verifyopts.Options {
		var o verifyopts.Options
		for _, opt := range buildVerifyRefOptions(remoteRefName, latestOnly) {
			opt(&o)
		}
		return o
	}

	set := apply("refs/heads/upstream-main", true)
	if set.RefNameOverride != "refs/heads/upstream-main" {
		t.Errorf("expected ref name override %q, got %q", "refs/heads/upstream-main", set.RefNameOverride)
	}
	if !set.LatestOnly {
		t.Errorf("expected LatestOnly to be true")
	}

	empty := apply("", false)
	if empty.RefNameOverride != "" {
		t.Errorf("expected no ref name override for empty input, got %q", empty.RefNameOverride)
	}
	if empty.LatestOnly {
		t.Errorf("expected LatestOnly to be false")
	}
}

func TestVerifyFormsValidSubmission(t *testing.T) {
	defaultLogger := slog.Default()
	t.Cleanup(func() { slog.SetDefault(defaultLogger) })

	o := &options{
		readOnly:  true,
		targetRef: "policy",
	}

	t.Run("Verify Ref", func(t *testing.T) {
		m := initialModel(context.Background(), o)
		m.screen = screenVerifyRefForm
		m.verifyRefScreen.reset()
		m.verifyRefScreen.form.inputs[0].SetValue("refs/heads/main")
		m.verifyRefScreen.form.inputs[1].SetValue("refs/heads/upstream-main")
		m.verifyRefScreen.form.inputs[2].SetValue("y")

		updatedModel, _ := m.verifyRefScreen.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
		resModel := updatedModel.(model)

		if !resModel.verifying {
			t.Errorf("expected verifying to be true")
		}
		if resModel.errorMsg != "" {
			t.Errorf("expected empty errorMsg, got %q", resModel.errorMsg)
		}
	})

	t.Run("Verify Mergeable", func(t *testing.T) {
		m := initialModel(context.Background(), o)
		m.screen = screenVerifyMergeableForm
		m.verifyMergeableScreen.reset()
		m.verifyMergeableScreen.form.inputs[0].SetValue("refs/heads/feature")
		m.verifyMergeableScreen.form.inputs[1].SetValue("refs/heads/main")
		m.verifyMergeableScreen.form.inputs[2].SetValue("y")

		updatedModel, _ := m.verifyMergeableScreen.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
		resModel := updatedModel.(model)

		if !resModel.verifying {
			t.Errorf("expected verifying to be true")
		}
		if resModel.errorMsg != "" {
			t.Errorf("expected empty errorMsg, got %q", resModel.errorMsg)
		}
	})
}
