// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/gittuf/gittuf/pkg/gitstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSetupWizard(t *testing.T) {
	wizard := newSetupWizard(nil, "test-key.pub")
	assert.Equal(t, wizardStepOverview, wizard.step)
	assert.Equal(t, "main", wizard.targetBranch)
	assert.Equal(t, "test-key.pub", wizard.gitConfigKey)
	assert.Equal(t, "test-key.pub", wizard.selectedKey)
	assert.Equal(t, "main", wizard.branchInput.Value())
	assert.Contains(t, wizard.keyInput.Placeholder, "fulcio:")

	t.Run("detects key and user.name from Git config", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo := gitinterface.CreateTestGitRepository(t, tmpDir, false)
		require.NoError(t, gitRepo.SetGitConfig(string(gitstore.ConfigGPGFormat), "x509"))
		require.NoError(t, gitRepo.SetGitConfig(string(gitstore.ConfigGPGX509Program), "gitsign"))
		require.NoError(t, gitRepo.SetGitConfig(string(gitstore.ConfigUserSigningKey), "dev@example.com::https://accounts.google.com"))
		require.NoError(t, gitRepo.SetGitConfig(string(gitstore.ConfigUserName), "Jane Doe"))

		t.Chdir(tmpDir)
		repo, err := gittuf.LoadRepository(".")
		require.NoError(t, err)

		w := newSetupWizard(repo, "")
		assert.Equal(t, "dev@example.com::https://accounts.google.com", w.gitConfigKey)
		assert.Equal(t, "dev@example.com::https://accounts.google.com", w.selectedKey)
		assert.Equal(t, "Jane-Doe", w.principalName)
		assert.Equal(t, "Jane-Doe", w.personInput.Value())
	})
}

func TestHasPolicyOrStaging(t *testing.T) {
	assert.False(t, hasPolicyOrStaging(nil))

	tmpDir := t.TempDir()
	gitinterface.CreateTestGitRepository(t, tmpDir, false)

	t.Chdir(tmpDir)

	repo, err := gittuf.LoadRepository(".")
	require.NoError(t, err)

	// Uninitialized repository has neither policy nor policy-staging
	assert.False(t, hasPolicyOrStaging(repo))

	// Write to refs/gittuf/policy-staging without applying to refs/gittuf/policy
	storer := repo.GetStorer()
	emptyTreeID, err := storer.EmptyTree()
	require.NoError(t, err)
	_, err = storer.Commit(emptyTreeID, policy.PolicyStagingRef, "Stage initial policy\n", false)
	require.NoError(t, err)

	hasApplied, err := repo.HasPolicy()
	require.NoError(t, err)
	assert.False(t, hasApplied)

	// hasPolicyOrStaging must return true when policy-staging exists
	assert.True(t, hasPolicyOrStaging(repo))
}

func TestUpdateSetupWizardGuidedFlow(t *testing.T) {
	m := model{
		ctx:             context.Background(),
		showSetupWizard: true,
		setupWizard:     newSetupWizard(nil, "test-key.pub"),
	}

	// Step 0: Overview -> Enter advances to Key step
	assert.Equal(t, wizardStepOverview, m.setupWizard.step)
	res, _ := m.updateSetupWizard(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(model)
	assert.Equal(t, wizardStepKey, m.setupWizard.step)

	// Step 1: Key step -> Down moves cursor to Override (1), Up moves back to Use Git config key (0)
	res, _ = m.updateSetupWizard(tea.KeyMsg{Type: tea.KeyDown})
	m = res.(model)
	assert.Equal(t, 1, m.setupWizard.keyCursor)

	res, _ = m.updateSetupWizard(tea.KeyMsg{Type: tea.KeyUp})
	m = res.(model)
	assert.Equal(t, 0, m.setupWizard.keyCursor)

	// Selecting option 0 (Use key from Git config) advances to Principal step
	res, _ = m.updateSetupWizard(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(model)
	assert.Equal(t, wizardStepPrincipal, m.setupWizard.step)
	assert.Equal(t, "test-key.pub", m.setupWizard.selectedKey)

	// Esc from Principal returns to Key step
	res, _ = m.updateSetupWizard(tea.KeyMsg{Type: tea.KeyEsc})
	m = res.(model)
	assert.Equal(t, wizardStepKey, m.setupWizard.step)

	// Move down to Override (1) and press Enter -> CustomKeyInput step
	res, _ = m.updateSetupWizard(tea.KeyMsg{Type: tea.KeyDown})
	m = res.(model)
	res, _ = m.updateSetupWizard(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(model)
	assert.Equal(t, wizardStepCustomKeyInput, m.setupWizard.step)

	// Enter custom key path and confirm -> Principal step
	m.setupWizard.keyInput.SetValue("/custom/path/key.pub")
	res, _ = m.updateSetupWizard(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(model)
	assert.Equal(t, wizardStepPrincipal, m.setupWizard.step)
	assert.Equal(t, "/custom/path/key.pub", m.setupWizard.selectedKey)

	// Enter principal name and confirm -> Branch step
	m.setupWizard.personInput.SetValue("jane-doe")
	res, _ = m.updateSetupWizard(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(model)
	assert.Equal(t, wizardStepBranch, m.setupWizard.step)
	assert.Equal(t, "jane-doe", m.setupWizard.principalName)

	// Confirm branch -> Applying step
	m.setupWizard.branchInput.SetValue("refs/heads/develop")
	res, cmd := m.updateSetupWizard(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(model)
	assert.Equal(t, wizardStepApplying, m.setupWizard.step)
	assert.Equal(t, "develop", m.setupWizard.targetBranch)
	assert.NotNil(t, cmd)
	assert.Contains(t, m.setupWizard.applyingMsg, "develop")
}

func TestUpdateSetupWizardCompleteAndQuit(t *testing.T) {
	o := &options{
		readOnly:  true,
		targetRef: "policy",
		p:         &persistent.Options{},
	}
	m := initialModel(context.Background(), o)
	m.showSetupWizard = true
	m.setupWizard = newSetupWizard(nil, "test-key.pub")
	m.setupWizard.step = wizardStepApplying
	m.setupWizard.targetBranch = "main"

	// Receiving setupDoneMsg transitions to wizardStepComplete while keeping modal open
	res, _ := m.updateInternal(setupDoneMsg{signingKey: "test-key", err: nil})
	m = res.(model)
	assert.True(t, m.showSetupWizard)
	assert.Equal(t, wizardStepComplete, m.setupWizard.step)
	assert.False(t, m.readOnly)
	assert.Equal(t, "test-key", m.options.p.SigningKey)

	// Pressing Enter on wizardStepComplete drops into the regular TUI
	res, _ = m.updateSetupWizard(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(model)
	assert.False(t, m.showSetupWizard)
	assert.Contains(t, m.footer, "main")

	// Pressing 'q' on wizardStepComplete quits the TUI
	m.showSetupWizard = true
	m.setupWizard.step = wizardStepComplete
	_, quitCmd := m.updateSetupWizard(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	assert.NotNil(t, quitCmd)
}

func TestUpdateSetupWizardEscSkip(t *testing.T) {
	m := model{
		ctx:             context.Background(),
		showSetupWizard: true,
		setupWizard:     newSetupWizard(nil, "test-key.pub"),
	}

	// Pressing 'esc' on Overview closes the wizard and sets read-only
	res, _ := m.updateSetupWizard(tea.KeyMsg{Type: tea.KeyEsc})
	m = res.(model)
	assert.False(t, m.showSetupWizard)
	assert.True(t, m.readOnly)
	assert.Contains(t, m.footer, "read-only")
}

func TestUpdateSetupWizardErrorHandling(t *testing.T) {
	// 1. Enter resets to overview for retry
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
	assert.Equal(t, wizardStepOverview, m.setupWizard.step)
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

func TestRenderSetupWizardDialog(t *testing.T) {
	m := model{
		width:           80,
		height:          24,
		showSetupWizard: true,
		setupWizard:     newSetupWizard(nil, "test-key.pub"),
	}

	// Overview step communicates Root of Trust, Top-Level Policy, and Branch Protection
	outOverview := renderSetupWizardDialog(m)
	require.NotEmpty(t, outOverview)
	assert.Contains(t, outOverview, "Welcome to gittuf!")
	assert.Contains(t, outOverview, "Root of Trust")
	assert.Contains(t, outOverview, "Top-Level Policy")
	assert.Contains(t, outOverview, "Branch Protection")
	assert.Contains(t, outOverview, "CLI")

	// Key step
	m.setupWizard.step = wizardStepKey
	outKey := renderSetupWizardDialog(m)
	assert.Contains(t, outKey, "Select Signing Key")
	assert.Contains(t, outKey, "Override signing key")

	// Principal step
	m.setupWizard.step = wizardStepPrincipal
	outPrincipal := renderSetupWizardDialog(m)
	assert.Contains(t, outPrincipal, "Enter Principal Name")

	// Branch step shows policy summary
	m.setupWizard.step = wizardStepBranch
	m.setupWizard.principalName = "jane-doe"
	outBranch := renderSetupWizardDialog(m)
	assert.Contains(t, outBranch, "Confirm Protected Branch & Policy")
	assert.Contains(t, outBranch, "Root of Trust")
	assert.Contains(t, outBranch, "Top-Level Policy")
	assert.Contains(t, outBranch, "jane-doe")

	// Applying step
	m.setupWizard.step = wizardStepApplying
	m.setupWizard.applyingMsg = "Initializing gittuf policy..."
	outApplying := renderSetupWizardDialog(m)
	assert.Contains(t, outApplying, "Initializing gittuf policy")

	// Complete step informs user and offers TUI or quit
	m.setupWizard.step = wizardStepComplete
	outComplete := renderSetupWizardDialog(m)
	assert.Contains(t, outComplete, "Policy Applied Successfully")
	assert.Contains(t, outComplete, "open TUI")
	assert.Contains(t, outComplete, "quit")

	// Error step
	m.setupWizard.step = wizardStepError
	m.setupWizard.err = errors.New("simulated failure")
	outErr := renderSetupWizardDialog(m)
	assert.Contains(t, outErr, "Setup Failed")
	assert.Contains(t, outErr, "simulated failure")
}

func TestRenderSetupWizardDialogTerminalResize(t *testing.T) {
	mSmall := model{
		width:           20,
		height:          10,
		showSetupWizard: true,
		setupWizard:     newSetupWizard(nil, "test-key.pub"),
	}
	assert.NotEmpty(t, renderSetupWizardDialog(mSmall))

	mWide := model{
		width:           200,
		height:          60,
		showSetupWizard: true,
		setupWizard:     newSetupWizard(nil, "test-key.pub"),
	}
	assert.NotEmpty(t, renderSetupWizardDialog(mWide))
}
