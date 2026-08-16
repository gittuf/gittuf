// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
)

func TestPolicyLifecycleScreenInitialization(t *testing.T) {
	o := &options{
		readOnly:   false,
		targetRef:  "policy",
		policyName: "targets",
		p:          &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)
	s := &m.policyLifecycleScreen

	// Test initInputs for Initialize Policy
	s.initInputs("Initialize Policy", &m)
	if s.inputs[0].Value() != "targets" {
		t.Errorf("expected default policyName 'targets', got %q", s.inputs[0].Value())
	}

	// Test initInputs for Pull Policy (default: origin)
	s.initInputs("Pull Policy", &m)
	if s.inputs[0].Value() != "origin" {
		t.Errorf("expected default remote 'origin', got %q", s.inputs[0].Value())
	}

	// Test initInputs for Stage Changes (default: empty for local)
	s.initInputs("Stage Changes", &m)
	if len(s.inputs) != 1 {
		t.Fatalf("expected 1 input for Stage Changes, got %d", len(s.inputs))
	}
}

func TestPolicyLifecycleScreenFocusCycle(t *testing.T) {
	o := &options{
		readOnly:  false,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)
	s := &m.policyLifecycleScreen
	s.initInputs("Initialize Policy", &m)

	if s.focusIndex != 0 {
		t.Errorf("expected focus 0, got %d", s.focusIndex)
	}

	// Cycle tab
	s.cycleFocus("tab")
	if s.focusIndex != 0 { // single field form stays at 0
		t.Errorf("expected focus 0 for single field, got %d", s.focusIndex)
	}
}

func TestPolicyLifecycleScreenReadOnlyAction(t *testing.T) {
	o := &options{
		readOnly:  true,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)
	m.readOnly = true
	s := &m.policyLifecycleScreen
	m.screen = screenPolicyLifecycle

	s.list = list.New([]list.Item{item{title: "Initialize Policy"}}, list.NewDefaultDelegate(), 80, 20)

	// Pressing enter in read-only mode should trigger error dialog
	s.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
	if m.errorDialog == nil {
		t.Error("expected errorDialog in read-only mode, got nil")
	}
	if !strings.Contains(m.errorDialog.message, "cannot perform action in read-only mode") {
		t.Errorf("expected read-only error message, got %q", m.errorDialog.message)
	}
}

func TestPolicyLifecycleScreenFormSubmissions(t *testing.T) {
	actions := []string{
		"Initialize Policy",
		"Increment Version",
		"Sign Policy",
		"Stage Changes",
		"Apply Changes",
		"Pull Policy",
		"Push Policy",
	}

	for _, act := range actions {
		t.Run(act, func(t *testing.T) {
			o := &options{
				readOnly:   false,
				targetRef:  "policy",
				policyName: "targets",
				p:          &persistent.Options{SigningKey: "dummy-key"},
			}

			m := initialModel(context.Background(), o)
			s := &m.policyLifecycleScreen
			m.screen = screenPolicyLifecycleForm

			s.initInputs(act, &m)
			_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
			if m.screen != screenPolicyLifecycle {
				t.Errorf("expected screen to return to screenPolicyLifecycle, got %v", m.screen)
			}
			if cmd == nil {
				t.Error("expected non-nil cmd returned for lifecycle command submission")
			}
		})
	}
}

func TestPolicyLifecycleScreenNavigationKeys(t *testing.T) {
	o := &options{
		readOnly:  false,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)
	s := &m.policyLifecycleScreen

	// Esc from screenPolicyLifecycle returns to screenPolicy
	m.screen = screenPolicyLifecycle
	s.Update(tea.KeyMsg{Type: tea.KeyEsc}, &m)
	if m.screen != screenPolicy {
		t.Errorf("expected screenPolicy on Esc, got %v", m.screen)
	}

	// Esc from screenPolicyLifecycleForm returns to screenPolicyLifecycle
	m.screen = screenPolicyLifecycleForm
	s.initInputs("Initialize Policy", &m)
	s.Update(tea.KeyMsg{Type: tea.KeyEsc}, &m)
	if m.screen != screenPolicyLifecycle {
		t.Errorf("expected screenPolicyLifecycle on form Esc, got %v", m.screen)
	}
}

func TestPolicyLifecycleScreenViewRendering(t *testing.T) {
	o := &options{
		readOnly:  false,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)
	s := &m.policyLifecycleScreen

	// Main list view
	m.screen = screenPolicyLifecycle
	viewStr := s.View(&m)
	if !strings.Contains(viewStr, "Home › Policy › Lifecycle") {
		t.Errorf("expected header in view, got %q", viewStr)
	}

	// Form view for Increment Version (includes warning banner)
	m.screen = screenPolicyLifecycleForm
	s.initInputs("Increment Version", &m)
	viewStr = s.View(&m)
	if !strings.Contains(viewStr, "Warning: This is an advanced operation") {
		t.Errorf("expected warning banner in Increment Version view, got %q", viewStr)
	}
}

func TestBackendExecInterface(t *testing.T) {
	be := backendExec{run: func() error { return nil }}
	if err := be.Run(); err != nil {
		t.Errorf("expected nil error from backendExec.Run(), got %v", err)
	}

	beErr := backendExec{run: func() error { return errors.New("exec error") }}
	if err := beErr.Run(); err == nil || err.Error() != "exec error" {
		t.Errorf("expected 'exec error' from backendExec.Run(), got %v", err)
	}

	// Cover setters
	be.SetStdin(nil)
	be.SetStdout(nil)
	be.SetStderr(nil)
}

func TestHandlePolicyLifecycleCommandReadOnly(t *testing.T) {
	o := &options{
		readOnly:  true,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)
	m.readOnly = true

	cmd := handlePolicyLifecycleCommand(&m, "Initialize Policy", "targets", "", false)
	msg := cmd()
	res, ok := msg.(policyLifecycleResultMsg)
	if !ok {
		t.Fatalf("expected policyLifecycleResultMsg, got %T", msg)
	}
	if res.err == nil || !strings.Contains(res.err.Error(), "read-only mode") {
		t.Errorf("expected read-only mode error, got %v", res.err)
	}
}
