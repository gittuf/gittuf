// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
)

func TestViewHelperFunctions(t *testing.T) {
	// Test renderWithMargin
	marginStr := renderWithMargin("content")
	if !strings.Contains(marginStr, "content") {
		t.Errorf("expected margin content, got %q", marginStr)
	}

	// Test renderFooter
	footerStr := renderFooter("footer-text")
	if !strings.Contains(footerStr, "footer-text") {
		t.Errorf("expected footer text, got %q", footerStr)
	}

	// Test renderErrorMsg
	if renderErrorMsg("") != "" {
		t.Error("expected empty string for empty errorMsg")
	}
	errMsg := renderErrorMsg("some error")
	if !strings.Contains(errMsg, "some error") {
		t.Errorf("expected error message formatted, got %q", errMsg)
	}

	// Test renderDeleteOverlay
	delOverlay := renderDeleteOverlay("test-target")
	if !strings.Contains(delOverlay, "Delete rule \"test-target\"?") {
		t.Errorf("expected delete overlay string, got %q", delOverlay)
	}

	// Test renderActionHints for readOnly vs edit mode
	hintsReadOnly := renderActionHints(true)
	if !strings.Contains(hintsReadOnly, "help") || strings.Contains(hintsReadOnly, "add") {
		t.Errorf("unexpected readOnly action hints: %q", hintsReadOnly)
	}

	hintsEdit := renderActionHints(false)
	if !strings.Contains(hintsEdit, "add") || !strings.Contains(hintsEdit, "edit") {
		t.Errorf("unexpected edit mode action hints: %q", hintsEdit)
	}
}

func TestRenderFooterBoxVariants(t *testing.T) {
	o := &options{
		readOnly:  true,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)
	m.readOnly = true
	m.showHelp = true
	m.signerError = "signer unavailable"

	// Read-only mode with showHelp and signerError
	boxStr := renderFooterBox(m)
	if !strings.Contains(boxStr, "Read-only mode") || !strings.Contains(boxStr, "signer unavailable") {
		t.Errorf("expected read-only help box with signer error, got %q", boxStr)
	}

	// Read-only mode without showHelp but with signerError
	m.showHelp = false
	boxStr = renderFooterBox(m)
	if !strings.Contains(boxStr, "Notice: signer") {
		t.Errorf("expected signer notice in footer, got %q", boxStr)
	}
}

func TestRenderErrorAndPopupDialog(t *testing.T) {
	o := &options{
		readOnly:  false,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)
	m.width = 100

	// When no error dialog present
	if renderErrorDialog(m) != "" || renderPopupDialog(m) != "" {
		t.Error("expected empty string when no error dialog is present")
	}

	// Open error dialog
	m.openErrorDialog("Fatal Error", "Something went wrong")

	dialogStr := renderPopupDialog(m)
	if !strings.Contains(dialogStr, "Fatal Error") || !strings.Contains(dialogStr, "Something went wrong") {
		t.Errorf("expected dialog with title and message, got %q", dialogStr)
	}

	// Test small width error dialog boundary
	m.width = 10
	dialogSmallStr := renderErrorDialog(m)
	if !strings.Contains(dialogSmallStr, "Fatal Error") {
		t.Errorf("expected small dialog, got %q", dialogSmallStr)
	}
}

func TestRenderStatusBar(t *testing.T) {
	// Width 0 fallback (defaults to 80)
	barStr := renderStatusBar("TestScreen", false, 0)
	if !strings.Contains(barStr, "TestScreen") || !strings.Contains(barStr, "Edit Mode") {
		t.Errorf("expected status bar with Edit Mode, got %q", barStr)
	}

	// Read-only status bar
	barReadOnly := renderStatusBar("TestScreen", true, 100)
	if !strings.Contains(barReadOnly, "Read-only") {
		t.Errorf("expected status bar with Read-only, got %q", barReadOnly)
	}
}

func TestRenderListOrEmpty(t *testing.T) {
	o := &options{
		readOnly:  true,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)
	m.readOnly = true
	m.signerError = "key error"
	m.width = 80
	m.height = 24

	l := list.New([]list.Item{item{title: "item-1"}}, list.NewDefaultDelegate(), 80, 20)

	// Non-empty list returns l.View()
	viewStr := m.renderListOrEmpty(l, 1, "No items")
	if !strings.Contains(viewStr, "item-1") {
		t.Errorf("expected item-1 in list view, got %q", viewStr)
	}

	// Empty list returns placeholder
	emptyView := m.renderListOrEmpty(l, 0, "No items found")
	if !strings.Contains(emptyView, "No items found") {
		t.Errorf("expected empty state text, got %q", emptyView)
	}
}

func TestModelViewScreenStates(t *testing.T) {
	o := &options{
		readOnly:  false,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)

	// Width/Height == 0 returns spinner loading
	m.width = 0
	m.height = 0
	viewStr := m.View()
	if !strings.Contains(viewStr, "Loading TUI...") {
		t.Errorf("expected Loading TUI... when width/height 0, got %q", viewStr)
	}

	m.width = 80
	m.height = 24

	// screenLoading without error
	m.screen = screenLoading
	viewStr = m.View()
	if !strings.Contains(viewStr, "Loading, please wait...") {
		t.Errorf("expected loading message, got %q", viewStr)
	}

	// screenLoading with error
	m.errorMsg = "Failed to load repo"
	viewStr = m.View()
	if !strings.Contains(viewStr, "Failed to load repo") {
		t.Errorf("expected errorMsg in loading screen, got %q", viewStr)
	}
	m.errorMsg = ""

	// Verifying mode
	m.verifying = true
	m.loadingMsg = "Verifying repository..."
	viewStr = m.View()
	if !strings.Contains(viewStr, "Verifying repository...") {
		t.Errorf("expected verifying message, got %q", viewStr)
	}
	m.verifying = false

	// Default unknown screen
	m.screen = screen(999)
	viewStr = m.View()
	if !strings.Contains(viewStr, "Unknown screen") {
		t.Errorf("expected 'Unknown screen' for unknown screen enum, got %q", viewStr)
	}
}
