// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
