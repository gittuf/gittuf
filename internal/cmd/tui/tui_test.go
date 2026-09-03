// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	lipgloss.SetColorProfile(termenv.Ascii)
}

func TestTUI(t *testing.T) {
	tmpDir := t.TempDir()
	currentDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
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
		teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
			return strings.Contains(string(out), "Policy")
		}, teatest.WithCheckInterval(time.Millisecond*100), teatest.WithDuration(time.Second*15))

		// Select "Policy" (already selected by default, so just press enter)
		tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

		// Now we should be on the Policy Operations screen.
		teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
			return strings.Contains(string(out), "Home › Policy")
		}, teatest.WithCheckInterval(time.Millisecond*100), teatest.WithDuration(time.Second*15))

		// Select "View Rules" (already selected by default)
		tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

		// Now we should be on the Policy Rules screen.
		// We check for the "Policy Rules" title OR the screen-specific empty-state message.
		teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
			content := string(out)
			return strings.Contains(content, "Home › Policy › Rules") || strings.Contains(content, "No rules configured")
		}, teatest.WithCheckInterval(time.Millisecond*100), teatest.WithDuration(time.Second*15))

		// Send "q" to quit
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
		tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second*15))
	})

	t.Run("Trust UI Navigation", func(t *testing.T) {
		o := &options{
			readOnly:  true,
			targetRef: "policy",
		}

		m := initialModel(context.Background(), o)

		tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))

		teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
			return strings.Contains(string(out), "Policy")
		}, teatest.WithCheckInterval(time.Millisecond*100), teatest.WithDuration(time.Second*15))

		// select "Trust" from the main menu (click down arrow to move selection, then press enter to select)
		tm.Send(tea.KeyMsg{Type: tea.KeyDown})
		tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

		teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
			return strings.Contains(string(out), "Home › Trust")
		}, teatest.WithCheckInterval(time.Millisecond*100), teatest.WithDuration(time.Second*15))

		// select "View Global Rules" (already selected by default)
		tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

		// Now we should end up on the Trust Global Rules screen.
		// check for the screen title OR the screen-specific empty-state message.
		teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
			content := string(out)
			return strings.Contains(content, "Home › Trust › Global Rules") || strings.Contains(content, "No global rules configured")
		}, teatest.WithCheckInterval(time.Millisecond*100), teatest.WithDuration(time.Second*15))

		// Click "q" to quit
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
		tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second*15))
	})
	t.Run("Trust Menu Hides Write-Only Sections In Read-Only Mode", func(t *testing.T) {
		o := &options{readOnly: true, targetRef: "policy"}
		m := initialModel(context.Background(), o)
		tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))

		teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
			return strings.Contains(string(out), "Policy")
		}, teatest.WithCheckInterval(time.Millisecond*100), teatest.WithDuration(time.Second*15))

		tm.Send(tea.KeyMsg{Type: tea.KeyDown})
		tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

		teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
			content := string(out)
			return strings.Contains(content, "Home › Trust") &&
				strings.Contains(content, "View Global Rules") &&
				strings.Contains(content, "Hooks") &&
				strings.Contains(content, "Propagation") &&
				!strings.Contains(content, "Keys & Thresholds") &&
				!strings.Contains(content, "GitHub App") &&
				!strings.Contains(content, "Lifecycle") &&
				!strings.Contains(content, "Repo/Network")
		}, teatest.WithCheckInterval(time.Millisecond*100), teatest.WithDuration(time.Second*15))
	})

	t.Run("Trust Hooks Navigation", func(t *testing.T) {
		o := &options{readOnly: true, targetRef: "policy"}
		m := initialModel(context.Background(), o)
		tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))

		teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
			return strings.Contains(string(out), "Policy")
		}, teatest.WithCheckInterval(time.Millisecond*100), teatest.WithDuration(time.Second*15))

		tm.Send(tea.KeyMsg{Type: tea.KeyDown})
		tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
		tm.Send(tea.KeyMsg{Type: tea.KeyDown})
		tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

		teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
			content := string(out)
			return strings.Contains(content, "Home › Trust › Hooks") &&
				strings.Contains(content, "List Hooks") &&
				!strings.Contains(content, "Add Hook")
		}, teatest.WithCheckInterval(time.Millisecond*100), teatest.WithDuration(time.Second*15))

		tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

		teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
			content := string(out)
			return strings.Contains(content, "List Hooks Failed") &&
				strings.Contains(content, "Press Enter or Esc to close.")
		}, teatest.WithCheckInterval(time.Millisecond*100), teatest.WithDuration(time.Second*15))

		tm.Send(tea.KeyMsg{Type: tea.KeyEsc})

		teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
			content := string(out)
			return strings.Contains(content, "Trust Hooks") &&
				strings.Contains(content, "List Hooks")
		}, teatest.WithCheckInterval(time.Millisecond*100), teatest.WithDuration(time.Second*15))
	})

	t.Run("Trust Propagation Navigation", func(t *testing.T) {
		o := &options{readOnly: true, targetRef: "policy"}
		m := initialModel(context.Background(), o)
		tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))

		teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
			return strings.Contains(string(out), "Policy")
		}, teatest.WithCheckInterval(time.Millisecond*100), teatest.WithDuration(time.Second*15))

		tm.Send(tea.KeyMsg{Type: tea.KeyDown})
		tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
		tm.Send(tea.KeyMsg{Type: tea.KeyDown})
		tm.Send(tea.KeyMsg{Type: tea.KeyDown})
		tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

		teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
			content := string(out)
			return strings.Contains(content, "Home › Trust › Propagation") &&
				strings.Contains(content, "List Directives") &&
				!strings.Contains(content, "Add Directive")
		}, teatest.WithCheckInterval(time.Millisecond*100), teatest.WithDuration(time.Second*15))

		tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

		teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
			content := string(out)
			return strings.Contains(content, "List Directives Failed") &&
				strings.Contains(content, "Press Enter or Esc to close.")
		}, teatest.WithCheckInterval(time.Millisecond*100), teatest.WithDuration(time.Second*15))

		tm.Send(tea.KeyMsg{Type: tea.KeyEsc})

		teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
			content := string(out)
			return strings.Contains(content, "Trust Propagation") &&
				strings.Contains(content, "List Directives")
		}, teatest.WithCheckInterval(time.Millisecond*100), teatest.WithDuration(time.Second*15))
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

func TestTrustGlobalRulesScreenHandleEsc(t *testing.T) {
	t.Run("clears delete confirm before leaving list", func(t *testing.T) {
		m := model{screen: screenTrustGlobalRules}
		screen := trustGlobalRulesScreen{
			confirmDelete: true,
			deleteTarget:  "test-rule",
		}

		handled := screen.handleEsc(&m)

		assert.True(t, handled)
		assert.Equal(t, screenTrustGlobalRules, m.screen)
		assert.False(t, screen.confirmDelete)
		assert.Empty(t, screen.deleteTarget)
	})

	t.Run("returns to trust menu from list", func(t *testing.T) {
		m := model{screen: screenTrustGlobalRules}
		screen := trustGlobalRulesScreen{}

		handled := screen.handleEsc(&m)

		assert.True(t, handled)
		assert.Equal(t, screenTrust, m.screen)
	})

	t.Run("returns to trust list from form", func(t *testing.T) {
		m := model{screen: screenTrustAddGlobalRule}
		screen := trustGlobalRulesScreen{}

		handled := screen.handleEsc(&m)

		assert.True(t, handled)
		assert.Equal(t, screenTrustGlobalRules, m.screen)
	})
}
func TestPolicyLifecycleResultShowsErrorDialog(t *testing.T) {
	m := initialModel(context.Background(), &options{readOnly: false, targetRef: "policy"})

	updated, _ := m.Update(policyLifecycleResultMsg{
		action: "Apply Changes",
		err:    errors.New("boom"),
	})
	typed := updated.(model)

	require.NotNil(t, typed.errorDialog)
	assert.Equal(t, "Apply Changes Failed", typed.errorDialog.title)
	assert.Equal(t, "boom", typed.errorDialog.message)
	assert.Empty(t, typed.footer)
	assert.Empty(t, typed.errorMsg)
}

func TestPolicyLifecycleResultSuccessKeepsFooter(t *testing.T) {
	m := initialModel(context.Background(), &options{readOnly: false, targetRef: "policy"})
	m.errorDialog = &errorDialog{title: "old", message: "old"}

	updated, _ := m.Update(policyLifecycleResultMsg{
		action: "Apply Changes",
		msg:    "ok",
	})
	typed := updated.(model)

	assert.Nil(t, typed.errorDialog)
	assert.Equal(t, "ok", typed.footer)
}

func TestLifecycleReadOnlyUsesErrorDialog(t *testing.T) {
	m := initialModel(context.Background(), &options{readOnly: true, targetRef: "policy"})
	m.screen = screenPolicyLifecycle
	m.readOnly = true

	updated, _ := m.policyLifecycleScreen.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
	typed := updated.(model)

	require.NotNil(t, typed.errorDialog)
	assert.Equal(t, "Initialize Policy Failed", typed.errorDialog.title)
	assert.Equal(t, "cannot perform action in read-only mode", typed.errorDialog.message)
	assert.Empty(t, typed.errorMsg)
}

func TestErrorDialogBlocksInputAndDismisses(t *testing.T) {
	m := initialModel(context.Background(), &options{readOnly: false, targetRef: "policy"})
	m.screen = screenPolicyLifecycle
	m.width = 80
	m.height = 24
	m.openErrorDialog("Stage Changes Failed", "boom")

	startIndex := m.policyLifecycleScreen.list.Index()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	typed := updated.(model)
	require.NotNil(t, typed.errorDialog)
	assert.Equal(t, startIndex, typed.policyLifecycleScreen.list.Index())

	updated, _ = typed.Update(tea.KeyMsg{Type: tea.KeyEnter})
	typed = updated.(model)
	assert.Nil(t, typed.errorDialog)
	assert.Equal(t, startIndex, typed.policyLifecycleScreen.list.Index())
}

func TestErrorDialogDismissesOnEsc(t *testing.T) {
	m := initialModel(context.Background(), &options{readOnly: false, targetRef: "policy"})
	m.screen = screenPolicyLifecycle
	m.openErrorDialog("Apply Changes Failed", "boom")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	typed := updated.(model)

	assert.Nil(t, typed.errorDialog)
}

func TestPrincipalValidationStaysInline(t *testing.T) {
	m := initialModel(context.Background(), &options{readOnly: false, targetRef: "policy"})
	m.screen = screenPolicyPrincipalsForm
	m.policyPrincipalsFormScreen = policyPrincipalsFormScreen{
		action: "Add Person",
		inputs: initInputs([]inputField{
			{placeholder: "Person ID", prompt: "Person ID: "},
			{placeholder: "Keys", prompt: "Public Keys: "},
			{placeholder: "Identities", prompt: "Associated Identities: "},
			{placeholder: "Custom", prompt: "Custom: "},
		}),
		focusIndex: 3,
	}

	updated, _ := m.policyPrincipalsFormScreen.handleFormSubmit(&m)
	typed := updated.(model)

	assert.Nil(t, typed.errorDialog)
	assert.Equal(t, "Error: Principal ID is required", typed.errorMsg)
}

func TestErrorDialogAllowsQuit(t *testing.T) {
	m := initialModel(context.Background(), &options{readOnly: false, targetRef: "policy"})
	m.screen = screenPolicyLifecycle
	m.openErrorDialog("Apply Changes Failed", "boom")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	typed := updated.(model)

	require.NotNil(t, cmd)
	assert.NotNil(t, typed.errorDialog)
}

func TestViewRendersErrorDialog(t *testing.T) {
	m := initialModel(context.Background(), &options{readOnly: false, targetRef: "policy"})
	m.screen = screenPolicyLifecycle
	m.width = 80
	m.height = 24
	m.openErrorDialog("Apply Changes Failed", "remote rejected update")

	view := m.View()

	assert.Contains(t, view, "Apply Changes Failed")
	assert.Contains(t, view, "remote rejected update")
	assert.Contains(t, view, "Press Enter or Esc to close.")
}

func TestTrustMenuRoutesToWriteScreens(t *testing.T) {
	m := initialModel(context.Background(), &options{readOnly: false, targetRef: "policy"})
	m.screen = screenTrust

	tests := []struct {
		name   string
		index  int
		screen screen
	}{
		{name: "keys and thresholds", index: 3, screen: screenTrustKeysThresholds},
		{name: "github app", index: 4, screen: screenTrustGitHubApp},
		{name: "lifecycle", index: 5, screen: screenTrustLifecycle},
		{name: "repo network", index: 6, screen: screenTrustRepoNetwork},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			local := m
			local.screen = screenTrust
			local.trustScreen.trustScreenList.Select(tt.index)

			updated, _ := local.trustScreen.Update(tea.KeyMsg{Type: tea.KeyEnter}, &local)
			typed := updated.(model)

			assert.Equal(t, tt.screen, typed.screen)
		})
	}
}

func TestTrustPropagationBackendErrorUsesErrorDialog(t *testing.T) {
	m := initialModel(context.Background(), &options{readOnly: false, targetRef: "policy"})
	m.screen = screenTrustUpdatePropagationForm
	m.trustPropagationScreen.selectedAction = trustUpdateDirectiveAction
	m.trustPropagationScreen.inputs = initInputs([]inputField{
		{placeholder: "Directive name", prompt: "Name: "},
		{placeholder: "Upstream repository", prompt: "From Repository: "},
		{placeholder: "Upstream reference", prompt: "From Reference: "},
		{placeholder: "Upstream path", prompt: "From Path: "},
		{placeholder: "Downstream reference", prompt: "Into Reference: "},
		{placeholder: "Downstream path", prompt: "Into Path: "},
	})
	m.trustPropagationScreen.inputs[0].SetValue("demo")
	m.trustPropagationScreen.inputs[1].SetValue("repo")
	m.trustPropagationScreen.inputs[2].SetValue("main")
	m.trustPropagationScreen.inputs[4].SetValue("refs/heads/main")
	m.trustPropagationScreen.inputs[5].SetValue("policy")

	updated, _ := m.trustPropagationScreen.handlePropagationFormSubmit(&m)
	typed := updated.(model)

	require.NotNil(t, typed.errorDialog)
	assert.Equal(t, "Update Directive Failed", typed.errorDialog.title)
	assert.NotEmpty(t, typed.errorDialog.message)
	assert.Empty(t, typed.errorMsg)
}

func TestTrustKeyValidationStaysInline(t *testing.T) {
	m := initialModel(context.Background(), &options{readOnly: false, targetRef: "policy"})
	m.screen = screenTrustKeyForm
	m.trustKeysScreen.selectedAction = trustKeysActionAddRootKey
	m.trustKeysScreen.inputs = initInputs([]inputField{
		{placeholder: "Path to public key or key ref", prompt: "Key: "},
	})

	updated, _ := m.trustKeysScreen.handleFormSubmit(&m)
	typed := updated.(model)

	assert.Nil(t, typed.errorDialog)
	assert.Equal(t, "Key input is required", typed.errorMsg)
}
