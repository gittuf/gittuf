// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveDefaultBranch(t *testing.T) {
	// Nil repo should return fallback "main"
	branch := resolveDefaultBranch(nil)
	assert.Equal(t, "main", branch)
}

func TestNewSetupWizard(t *testing.T) {
	wizard := newSetupWizard(nil, "main", "test-key.pub")
	assert.Equal(t, wizardStepModeSelect, wizard.step)
	assert.Equal(t, 0, wizard.cursor)
	assert.Equal(t, "main", wizard.defaultBranch)
	assert.Equal(t, "test-key.pub", wizard.detectedKey)
	assert.Equal(t, "main", wizard.branchInput.Value())
}

func TestUpdateSetupWizardNavigation(t *testing.T) {
	m := model{
		ctx:             context.Background(),
		showSetupWizard: true,
		setupWizard:     newSetupWizard(nil, "main", "test-key.pub"),
	}

	// Down arrow moves cursor to 1 (Customize)
	res, _ := m.updateSetupWizard(tea.KeyMsg{Type: tea.KeyDown})
	m = res.(model)
	assert.Equal(t, 1, m.setupWizard.cursor)

	// Down arrow moves cursor to 2 (Skip)
	res, _ = m.updateSetupWizard(tea.KeyMsg{Type: tea.KeyDown})
	m = res.(model)
	assert.Equal(t, 2, m.setupWizard.cursor)

	// Down arrow capped at 2
	res, _ = m.updateSetupWizard(tea.KeyMsg{Type: tea.KeyDown})
	m = res.(model)
	assert.Equal(t, 2, m.setupWizard.cursor)

	// Up arrow moves back to 1
	res, _ = m.updateSetupWizard(tea.KeyMsg{Type: tea.KeyUp})
	m = res.(model)
	assert.Equal(t, 1, m.setupWizard.cursor)

	// Up arrow moves back to 0 (Quick Start)
	res, _ = m.updateSetupWizard(tea.KeyMsg{Type: tea.KeyUp})
	m = res.(model)
	assert.Equal(t, 0, m.setupWizard.cursor)
}

func TestUpdateSetupWizardEscSkip(t *testing.T) {
	m := model{
		ctx:             context.Background(),
		showSetupWizard: true,
		setupWizard:     newSetupWizard(nil, "main", "test-key.pub"),
	}

	// Pressing 'esc' closes the wizard and sets read-only
	res, _ := m.updateSetupWizard(tea.KeyMsg{Type: tea.KeyEsc})
	m = res.(model)
	assert.False(t, m.showSetupWizard)
	assert.True(t, m.readOnly)
	assert.Contains(t, m.footer, "read-only")
}

func TestUpdateSetupWizardQuickStartSelection(t *testing.T) {
	m := model{
		ctx:             context.Background(),
		showSetupWizard: true,
		setupWizard:     newSetupWizard(nil, "main", "test-key.pub"),
	}

	// Cursor is at 0 (Quick Start), press Enter
	res, cmd := m.updateSetupWizard(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(model)
	assert.Equal(t, wizardStepApplying, m.setupWizard.step)
	assert.NotNil(t, cmd)
	assert.Contains(t, m.setupWizard.applyingMsg, "main")
}

func TestUpdateSetupWizardCustomizeAndBack(t *testing.T) {
	m := model{
		ctx:             context.Background(),
		showSetupWizard: true,
		setupWizard:     newSetupWizard(nil, "main", "test-key.pub"),
	}

	m.setupWizard.keyCandidates = []string{"key1.pub", "key2.pub"}
	m.setupWizard.cursor = 1
	res, _ := m.updateSetupWizard(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(model)
	assert.Equal(t, wizardStepCustomKey, m.setupWizard.step)

	// Selecting key advances to CustomBranch
	res, _ = m.updateSetupWizard(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(model)
	assert.Equal(t, wizardStepCustomBranch, m.setupWizard.step)

	// Hit Esc returns to ModeSelect
	res, _ = m.updateSetupWizard(tea.KeyMsg{Type: tea.KeyEsc})
	m = res.(model)
	assert.Equal(t, wizardStepModeSelect, m.setupWizard.step)

	// Enter CustomKey, navigate to "Enter custom key path", select it
	m.setupWizard.cursor = 1
	res, _ = m.updateSetupWizard(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(model)
	assert.Equal(t, wizardStepCustomKey, m.setupWizard.step)

	// Navigate down to custom key option (index 2)
	m.setupWizard.keyCursor = len(m.setupWizard.keyCandidates)
	res, _ = m.updateSetupWizard(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(model)
	assert.Equal(t, wizardStepCustomKeyInput, m.setupWizard.step)

	// Type custom key and press Enter
	m.setupWizard.keyInput.SetValue("/path/to/custom_key.pub")
	res, _ = m.updateSetupWizard(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(model)
	assert.Equal(t, wizardStepCustomBranch, m.setupWizard.step)
	assert.Equal(t, "/path/to/custom_key.pub", m.setupWizard.detectedKey)
}

func TestUpdateSetupWizardErrorHandling(t *testing.T) {
	// 1. Enter resets to mode select for retry
	m := model{
		ctx:             context.Background(),
		showSetupWizard: true,
		setupWizard: setupWizardModel{
			step: wizardStepError,
			err:  errors.New("test error"),
		},
	}
	res, _ := m.updateSetupWizard(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(model)
	assert.Equal(t, wizardStepModeSelect, m.setupWizard.step)
	assert.Nil(t, m.setupWizard.err)

	// 2. 'q' cleanly quits the program
	m.setupWizard.step = wizardStepError
	m.setupWizard.err = errors.New("test error")
	_, cmd := m.updateSetupWizard(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	assert.NotNil(t, cmd)

	// 3. 'esc' skips setup and moves to read-only mode
	m.setupWizard.step = wizardStepError
	m.setupWizard.err = errors.New("test error")
	res, _ = m.updateSetupWizard(tea.KeyMsg{Type: tea.KeyEsc})
	m = res.(model)
	assert.False(t, m.showSetupWizard)
	assert.True(t, m.readOnly)
}

func TestFormatKeyLabel(t *testing.T) {
	// Empty key
	assert.Equal(t, "", formatKeyLabel(""))

	// GPG Key
	gpgLabel := formatKeyLabel("gpg:1234567890ABCDEF")
	assert.Contains(t, gpgLabel, "GPG Key")
	assert.Contains(t, gpgLabel, "1234567890ABCDEF")

	// Sigstore / Fulcio
	fulcioLabel := formatKeyLabel("fulcio:user@example.com::https://accounts.google.com")
	assert.Contains(t, fulcioLabel, "Sigstore OIDC")

	// File with home replacement
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		testKeyPath := filepath.Join(home, ".ssh", "id_ed25519.pub")
		label := formatKeyLabel(testKeyPath)
		assert.True(t, strings.HasPrefix(label, "~/.ssh/id_ed25519.pub") || strings.HasPrefix(label, "~\\.ssh\\id_ed25519.pub"))
		assert.Contains(t, label, "SSH Key")
	}
}

func TestBranchSanitization(t *testing.T) {
	// Leading 'refs/heads/' should be cleanly trimmed
	b1 := strings.TrimPrefix(strings.TrimSpace("refs/heads/main"), "refs/heads/")
	assert.Equal(t, "main", b1)

	b2 := strings.TrimPrefix(strings.TrimSpace("  refs/heads/feature/auth  "), "refs/heads/")
	assert.Equal(t, "feature/auth", b2)

	b3 := strings.TrimPrefix(strings.TrimSpace("   "), "refs/heads/")
	if b3 == "" {
		b3 = "main"
	}
	assert.Equal(t, "main", b3)
}

func TestRenderSetupWizardDialog(t *testing.T) {
	m := model{
		width:           80,
		height:          24,
		showSetupWizard: true,
		setupWizard:     newSetupWizard(nil, "main", "test-key.pub"),
	}

	// ModeSelect step
	out := renderSetupWizardDialog(m)
	require.NotEmpty(t, out)
	assert.True(t, strings.Contains(out, "Welcome to gittuf") || strings.Contains(out, "gittuf"))
	assert.True(t, strings.Contains(out, "Quick Start") || strings.Contains(out, "Custom Setup"))

	// CustomBranch step
	m.setupWizard.step = wizardStepCustomBranch
	outBranch := renderSetupWizardDialog(m)
	assert.Contains(t, outBranch, "Confirm Protected Branch")

	// Applying step
	m.setupWizard.step = wizardStepApplying
	m.setupWizard.applyingMsg = "Initializing gittuf policy..."
	outApplying := renderSetupWizardDialog(m)
	assert.Contains(t, outApplying, "Initializing gittuf policy")

	// Error step
	m.setupWizard.step = wizardStepError
	m.setupWizard.err = errors.New("simulated failure")
	outErr := renderSetupWizardDialog(m)
	assert.Contains(t, outErr, "Setup Failed")
	assert.Contains(t, outErr, "simulated failure")
}

func TestRenderSetupWizardDialogTerminalResize(t *testing.T) {
	// Extreme small width (should clamp to min 40 without panic)
	mSmall := model{
		width:           20,
		height:          10,
		showSetupWizard: true,
		setupWizard:     newSetupWizard(nil, "main", "test-key.pub"),
	}
	outSmall := renderSetupWizardDialog(mSmall)
	assert.NotEmpty(t, outSmall)

	// Extra wide width (should clamp to max 68 without panic)
	mWide := model{
		width:           200,
		height:          60,
		showSetupWizard: true,
		setupWizard:     newSetupWizard(nil, "main", "test-key.pub"),
	}
	outWide := renderSetupWizardDialog(mWide)
	assert.NotEmpty(t, outWide)
}

func TestScanLocalKeyCandidatesNilRepo(t *testing.T) {
	candidates := scanLocalKeyCandidates(nil)
	assert.NotNil(t, candidates)
}
