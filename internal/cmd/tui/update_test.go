// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
)

func TestUpdateInitDoneMsg(t *testing.T) {
	o := &options{
		readOnly:  false,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)

	// 1. Success initDoneMsg
	msgSuccess := initDoneMsg{
		readOnly: false,
		footer:   "Ready",
		rules: []rule{
			{name: "rule-1"},
		},
		globalRules: []globalRule{
			{ruleName: "global-1"},
		},
	}

	updatedModel, _ := m.Update(msgSuccess)
	resModel := updatedModel.(model)
	if resModel.screen != screenChoice {
		t.Errorf("expected screenChoice after initDoneMsg, got %v", resModel.screen)
	}
	if resModel.footer != "Ready" {
		t.Errorf("expected footer 'Ready', got %q", resModel.footer)
	}

	// 2. Error initDoneMsg
	msgErr := initDoneMsg{
		err: fmt.Errorf("git repo error"),
	}
	updatedModel, _ = m.Update(msgErr)
	resModel = updatedModel.(model)
	if resModel.errorDialog == nil {
		t.Error("expected errorDialog on initDoneMsg error, got nil")
	} else {
		if resModel.errorDialog.title != "Initialization Failed" {
			t.Errorf("expected errorDialog title 'Initialization Failed', got %q", resModel.errorDialog.title)
		}
		if resModel.errorDialog.message != "git repo error" {
			t.Errorf("expected errorDialog message 'git repo error', got %q", resModel.errorDialog.message)
		}
	}
}

func TestUpdatePolicyLifecycleResultMsg(t *testing.T) {
	o := &options{
		readOnly:  false,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)

	// 1. Success message
	msgSuccess := policyLifecycleResultMsg{
		action: "Initialize Policy",
		msg:    "Successfully initialized policy",
		err:    nil,
	}
	updatedModel, _ := m.Update(msgSuccess)
	resModel := updatedModel.(model)
	if resModel.footer != "Successfully initialized policy" {
		t.Errorf("expected footer update on success, got %q", resModel.footer)
	}

	// 2. Error message
	msgError := policyLifecycleResultMsg{
		action: "Initialize Policy",
		err:    fmt.Errorf("already initialized"),
	}
	updatedModel, _ = m.Update(msgError)
	resModel = updatedModel.(model)
	if resModel.errorDialog == nil {
		t.Error("expected errorDialog on lifecycle error, got nil")
	}
}

func TestUpdateWindowSizeAndSpinner(t *testing.T) {
	o := &options{
		readOnly:  false,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)

	// WindowSizeMsg
	wsMsg := tea.WindowSizeMsg{Width: 100, Height: 40}
	updatedModel, _ := m.Update(wsMsg)
	resModel := updatedModel.(model)
	if resModel.width != 100 || resModel.height != 40 {
		t.Errorf("expected width 100 height 40, got %dx%d", resModel.width, resModel.height)
	}

	// Small WindowSizeMsg (testing <= 4 width and <= 6 height branches)
	wsSmallMsg := tea.WindowSizeMsg{Width: 2, Height: 4}
	updatedModel, _ = m.Update(wsSmallMsg)
	resModel = updatedModel.(model)
	if resModel.logViewport.Width != 2 || resModel.logViewport.Height != 4 {
		t.Errorf("expected viewport width 2 height 4, got %dx%d", resModel.logViewport.Width, resModel.logViewport.Height)
	}

	// Spinner TickMsg while loading
	m.screen = screenLoading
	spMsg := spinner.TickMsg{}
	m.Update(spMsg)
}

func TestUpdateGlobalKeybindings(t *testing.T) {
	o := &options{
		readOnly:  false,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)

	// 1. Ctrl+C quits
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("expected quit cmd on ctrl+c, got nil")
	}

	// 2. 'h' toggles help screen
	m.screen = screenChoice
	updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	resModel := updatedModel.(model)
	if resModel.screen != screenHelp {
		t.Errorf("expected screenHelp after pressing 'h', got %v", resModel.screen)
	}

	// Pressing 'h' again toggles back
	updatedModel, _ = resModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	resModel = updatedModel.(model)
	if resModel.screen != screenChoice {
		t.Errorf("expected screenChoice after pressing 'h' again, got %v", resModel.screen)
	}

	// 3. 'q' quits from non-form screens
	m.screen = screenPolicy
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Error("expected quit cmd on 'q' from non-form screen, got nil")
	}

	// 4. 'q' ignored as quit command in form screens (types 'q' into text input instead)
	m.screen = screenPolicyAddRule
	m.policyRulesScreen.initRuleInputs()
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	resModel = updatedModel.(model)
	if resModel.screen != screenPolicyAddRule {
		t.Errorf("expected screen to stay screenPolicyAddRule when typing 'q', got %v", resModel.screen)
	}
}

func TestUpdateEscNavigationAcrossScreens(t *testing.T) {
	o := &options{
		readOnly:  false,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)

	escScreens := []struct {
		name     string
		start    screen
		expected screen
	}{
		{"screenPolicy to screenChoice", screenPolicy, screenChoice},
		{"screenTrust to screenChoice", screenTrust, screenChoice},
		{"screenVerify to screenChoice", screenVerify, screenChoice},
		{"screenPolicyLifecycle to screenPolicy", screenPolicyLifecycle, screenPolicy},
		{"screenPolicyLifecycleForm to screenPolicyLifecycle", screenPolicyLifecycleForm, screenPolicyLifecycle},
		{"screenPolicyAddRule to screenPolicyRules", screenPolicyAddRule, screenPolicyRules},
		{"screenPolicyEditRule to screenPolicyRules", screenPolicyEditRule, screenPolicyRules},
		{"screenVerifyRefForm to screenVerify", screenVerifyRefForm, screenVerify},
		{"screenVerifyMergeableForm to screenVerify", screenVerifyMergeableForm, screenVerify},
	}

	for _, tc := range escScreens {
		t.Run(tc.name, func(t *testing.T) {
			m.screen = tc.start
			updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
			resModel := updatedModel.(model)
			if resModel.screen != tc.expected {
				t.Errorf("expected %v after Esc, got %v", tc.expected, resModel.screen)
			}
		})
	}
}

func TestUpdateErrorDialogKeyHandling(t *testing.T) {
	o := &options{
		readOnly:  false,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)
	m.openErrorDialog("Test Title", "Test Message")

	// 1. 'q' in error dialog quits
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Error("expected quit cmd when pressing 'q' in error dialog, got nil")
	}

	// 2. 'enter' in error dialog dismisses dialog
	updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	resModel := updatedModel.(model)
	if resModel.errorDialog != nil {
		t.Error("expected errorDialog to be dismissed after Enter, got non-nil")
	}
}

func TestLogStreamingHelpers(t *testing.T) {
	// Test splitAndTrim
	parts := splitAndTrim("  a , b,c  ")
	if len(parts) != 3 || parts[0] != "a" || parts[1] != "b" || parts[2] != "c" {
		t.Errorf("unexpected splitAndTrim result: %v", parts)
	}

	// Test collectLogsCmd and drainLogChannel
	scanner := bufio.NewScanner(strings.NewReader("line1\nline2\n"))
	ch := make(chan string, 10)
	cmd := collectLogsCmd(scanner, ch)
	cmd() // execute goroutine logic synchronously

	lines := drainLogChannel(ch)
	if len(lines) != 2 || lines[0] != "line1" || lines[1] != "line2" {
		t.Errorf("unexpected drained lines: %v", lines)
	}

	// Test logFlushTick
	tickCmd := logFlushTick(ch)
	if tickCmd == nil {
		t.Error("expected non-nil tea.Cmd from logFlushTick")
	}
}
