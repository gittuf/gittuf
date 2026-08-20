// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
)

func TestHomeScreenMenuSelections(t *testing.T) {
	o := &options{
		readOnly:  false,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)
	s := &m.homeScreen
	m.screen = screenChoice

	// 1. Select Policy
	s.choiceList = list.New([]list.Item{item{title: "Policy"}}, list.NewDefaultDelegate(), 80, 20)
	updatedModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
	resModel := updatedModel.(model)
	if resModel.screen != screenPolicy {
		t.Errorf("expected screenPolicy, got %v", resModel.screen)
	}

	// 2. Select Trust
	s.choiceList = list.New([]list.Item{item{title: "Trust"}}, list.NewDefaultDelegate(), 80, 20)
	updatedModel, _ = s.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
	resModel = updatedModel.(model)
	if resModel.screen != screenTrust {
		t.Errorf("expected screenTrust, got %v", resModel.screen)
	}

	// 3. Select Verify
	s.choiceList = list.New([]list.Item{item{title: "Verify"}}, list.NewDefaultDelegate(), 80, 20)
	updatedModel, _ = s.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
	resModel = updatedModel.(model)
	if resModel.screen != screenVerify {
		t.Errorf("expected screenVerify, got %v", resModel.screen)
	}
}

func TestHomeScreenViewRendering(t *testing.T) {
	o := &options{
		readOnly:  false,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)
	s := &m.homeScreen

	viewStr := s.View(&m)
	if !strings.Contains(viewStr, "Home") {
		t.Errorf("expected view to contain 'Home', got %q", viewStr)
	}
}
