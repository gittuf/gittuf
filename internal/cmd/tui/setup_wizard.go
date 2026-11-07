// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gittuf/gittuf/experimental/gittuf"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/gittuf/gittuf/pkg/gitstore"
	"golang.org/x/crypto/ssh"
)

type wizardStep int

const (
	wizardStepModeSelect wizardStep = iota
	wizardStepCustomKey
	wizardStepCustomKeyInput
	wizardStepCustomBranch
	wizardStepApplying
	wizardStepError
)

type setupWizardModel struct {
	step          wizardStep
	cursor        int
	defaultBranch string
	detectedKey   string
	keyCandidates []string
	keyCursor     int
	branchInput   textinput.Model
	keyInput      textinput.Model
	spinner       spinner.Model
	applyingMsg   string
	err           error
}

// setupDoneMsg is sent when policy bootstrapping completes.
type setupDoneMsg struct {
	err error
}

var (
	setupModalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#2ea043")).
			Padding(1, 2)

	setupTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3fb950")).
			Bold(true)

	setupDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#c9d1d9"))

	setupMutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8b949e"))

	setupHighlightStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#58a6ff")).
				Bold(true)

	setupSelectedOptStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#58a6ff")).
				Bold(true)

	setupNormalOptStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#c9d1d9"))

	setupFooterStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#3fb950")).
				PaddingTop(1)
)

func newSetupWizard(repo *gittuf.Repository, defaultBranch, detectedKey string) setupWizardModel {
	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#3fb950"))

	bi := textinput.New()
	bi.Placeholder = "e.g. main"
	bi.SetValue(defaultBranch)
	bi.Focus()

	ki := textinput.New()
	ki.Placeholder = "Path to SSH key or gpg:KEYID"
	ki.SetValue(detectedKey)

	candidates := scanLocalKeyCandidates(repo)

	return setupWizardModel{
		step:          wizardStepModeSelect,
		cursor:        0,
		defaultBranch: defaultBranch,
		detectedKey:   detectedKey,
		keyCandidates: candidates,
		branchInput:   bi,
		keyInput:      ki,
		spinner:       s,
	}
}

// resolveDefaultBranch identifies the default branch for the repository.
func resolveDefaultBranch(repo *gittuf.Repository) string {
	if repo != nil && repo.GetGitRepository() != nil {
		if ref, err := repo.GetGitRepository().GetSymbolicReferenceTarget("HEAD"); err == nil && ref != "" {
			return strings.TrimPrefix(ref, "refs/heads/")
		}
		if _, err := repo.GetGitRepository().GetReference("refs/heads/master"); err == nil {
			return "master"
		}
	}
	return "main"
}

// scanLocalKeyCandidates finds all available SSH and Git signing keys on the system.
func scanLocalKeyCandidates(repo *gittuf.Repository) []string {
	var candidates []string
	addCandidate := func(p string) {
		for _, c := range candidates {
			if c == p {
				return
			}
		}
		if _, err := os.Stat(p); err == nil {
			candidates = append(candidates, p)
		}
	}

	if repo != nil && repo.GetGitRepository() != nil {
		if key, _, err := repo.GetGitRepository().LookupConfig(gitstore.ConfigUserSigningKey); err == nil && key != "" {
			addCandidate(key)
		}
	}

	if home, err := os.UserHomeDir(); err == nil && home != "" {
		for _, base := range []string{"id_ed25519", "id_rsa", "id_ecdsa", "gittuf_id_ed25519"} {
			pub := filepath.Join(home, ".ssh", base+".pub")
			priv := filepath.Join(home, ".ssh", base)
			if _, err := os.Stat(pub); err == nil {
				addCandidate(pub)
			} else {
				addCandidate(priv)
			}
		}
	}

	return candidates
}

// discoverLocalKey determines the best default key to use for signing and trust.
func discoverLocalKey(repo *gittuf.Repository) string {
	candidates := scanLocalKeyCandidates(repo)
	if len(candidates) > 0 {
		return candidates[0]
	}

	return ""
}

// generateLocalSSHKey generates a new ed25519 SSH key dedicated to gittuf in ~/.ssh/gittuf_id_ed25519.
func generateLocalSSHKey() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return "", err
	}

	privPath := filepath.Join(sshDir, "gittuf_id_ed25519")
	pubPath := privPath + ".pub"

	if _, err := os.Stat(privPath); err == nil {
		return privPath, nil
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", fmt.Errorf("failed to generate ed25519 key: %w", err)
	}

	privBlock, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		return "", fmt.Errorf("failed to marshal private key: %w", err)
	}
	if err := os.WriteFile(privPath, pem.EncodeToMemory(privBlock), 0o600); err != nil {
		return "", fmt.Errorf("failed to write private key: %w", err)
	}

	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return "", fmt.Errorf("failed to create ssh public key: %w", err)
	}
	if err := os.WriteFile(pubPath, ssh.MarshalAuthorizedKey(sshPub), 0o600); err != nil {
		return "", fmt.Errorf("failed to write public key: %w", err)
	}

	return privPath, nil
}

// formatKeyLabel converts a raw key path into a safe, clean, human-readable display string.
func formatKeyLabel(keyPath string) string {
	if keyPath == "" {
		return ""
	}

	// Always prefer public key extension for display if available
	displayPath := keyPath
	if !strings.HasSuffix(displayPath, ".pub") && !strings.HasPrefix(displayPath, "gpg:") && !strings.HasPrefix(displayPath, "fulcio:") {
		if _, err := os.Stat(displayPath + ".pub"); err == nil {
			displayPath += ".pub"
		}
	}

	home, err := os.UserHomeDir()
	if err == nil && home != "" && strings.HasPrefix(displayPath, home) {
		displayPath = "~" + strings.TrimPrefix(displayPath, home)
	}
	displayPath = filepath.ToSlash(displayPath)

	keyType := "SSH Key"
	if strings.HasPrefix(displayPath, "gpg:") {
		keyType = "GPG Key"
	} else if strings.HasPrefix(displayPath, "fulcio:") {
		keyType = "Sigstore OIDC"
	}

	return fmt.Sprintf("%s (%s)", displayPath, keyType)
}

// applySetupPolicyCmd coordinates the one-click bootstrap of root, targets, and branch protection.
func applySetupPolicyCmd(ctx context.Context, repo *gittuf.Repository, keyPath, branchName string) tea.Cmd {
	return func() tea.Msg {
		branchName = strings.TrimPrefix(strings.TrimSpace(branchName), "refs/heads/")
		if branchName == "" {
			branchName = "main"
		}
		if keyPath == "" {
			return setupDoneMsg{err: errors.New("no signing key specified")}
		}

		// Resolve signing key (private key) vs principal key (public key)
		signingKeyPath, pubKeyPath := keyPath, keyPath
		if !strings.Contains(keyPath, ":") {
			priv := strings.TrimSuffix(keyPath, ".pub")
			if _, err := os.Stat(priv); err == nil {
				signingKeyPath = priv
			}
			if _, err := os.Stat(priv + ".pub"); err == nil {
				pubKeyPath = priv + ".pub"
			}
		}

		signer, err := gittuf.LoadSigner(repo, signingKeyPath)
		if err != nil {
			return setupDoneMsg{err: fmt.Errorf("failed to load signer for key '%s': %w", signingKeyPath, err)}
		}

		principal, err := gittuf.LoadPublicKey(pubKeyPath)
		if err != nil {
			return setupDoneMsg{err: fmt.Errorf("failed to load public key for '%s': %w", pubKeyPath, err)}
		}

		// 1. Initialize Root of Trust
		if err := repo.InitializeRoot(ctx, signer, false); err != nil {
			return setupDoneMsg{err: fmt.Errorf("failed to initialize root of trust: %w", err)}
		}

		// 2. Add Top-Level Targets Key
		if err := repo.AddTopLevelTargetsKey(ctx, signer, principal, false, trustpolicyopts.WithRSLEntry()); err != nil {
			return setupDoneMsg{err: fmt.Errorf("failed to add top-level targets key: %w", err)}
		}

		// 3. Initialize Targets Role
		if err := repo.InitializeTargets(ctx, signer, policy.TargetsRoleName, false, trustpolicyopts.WithRSLEntry()); err != nil {
			return setupDoneMsg{err: fmt.Errorf("failed to initialize targets: %w", err)}
		}

		// 4. Authorize Principal in Targets
		if err := repo.AddPrincipalToTargets(ctx, signer, policy.TargetsRoleName, []tuf.Principal{principal}, false, trustpolicyopts.WithRSLEntry()); err != nil {
			return setupDoneMsg{err: fmt.Errorf("failed to authorize principal in targets: %w", err)}
		}

		// 5. Add Branch Protection Delegation
		ruleName := "protect-" + branchName
		rulePattern := "git:refs/heads/" + branchName
		if err := repo.AddDelegation(ctx, signer, policy.TargetsRoleName, ruleName, []string{principal.ID()}, []string{rulePattern}, 1, false, trustpolicyopts.WithRSLEntry()); err != nil {
			return setupDoneMsg{err: fmt.Errorf("failed to create branch protection rule: %w", err)}
		}

		// 6. Stage & Apply Policy
		if err := repo.StagePolicy(ctx, "", true, false); err != nil {
			return setupDoneMsg{err: fmt.Errorf("failed to stage policy: %w", err)}
		}

		if err := repo.ApplyPolicy(ctx, "", true, false); err != nil {
			return setupDoneMsg{err: fmt.Errorf("failed to apply policy: %w", err)}
		}

		// 7. Record initial RSL entry for target branch if reference exists
		if repo.GetGitRepository() != nil {
			if _, refErr := repo.GetGitRepository().GetReference("refs/heads/" + branchName); refErr == nil {
				_ = repo.RecordRSLEntryForReference(ctx, "refs/heads/"+branchName, false)
			}
		}

		return setupDoneMsg{err: nil}
	}
}

// updateSetupWizard handles keyboard interactions when the setup modal is active.
func (m model) updateSetupWizard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.setupWizard.step {
	case wizardStepModeSelect:
		switch msg.String() {
		case "esc", "q":
			m.showSetupWizard = false
			m.readOnly = true
			m.footer = "Setup skipped. Exploring in read-only mode."
			return m, nil

		case "up", "k":
			if m.setupWizard.cursor > 0 {
				m.setupWizard.cursor--
			}
			return m, nil

		case "down", "j":
			if m.setupWizard.cursor < 2 {
				m.setupWizard.cursor++
			}
			return m, nil

		case "enter":
			switch m.setupWizard.cursor {
			case 0: // Quick Start (1-Click)
				if m.setupWizard.detectedKey == "" {
					// No key detected; navigate to key selection rather than silently generating
					m.setupWizard.step = wizardStepCustomKey
					return m, nil
				}
				m.setupWizard.step = wizardStepApplying
				m.setupWizard.applyingMsg = fmt.Sprintf("Initializing gittuf policy & protecting '%s'...", m.setupWizard.defaultBranch)
				return m, tea.Batch(
					m.setupWizard.spinner.Tick,
					applySetupPolicyCmd(m.ctx, m.repo, m.setupWizard.detectedKey, m.setupWizard.defaultBranch),
				)

			case 1: // Customize
				m.setupWizard.step = wizardStepCustomKey
				return m, nil

			case 2: // Skip / Read-Only
				m.showSetupWizard = false
				m.readOnly = true
				m.footer = "Setup skipped. Exploring in read-only mode."
				return m, nil
			}
		}

	case wizardStepCustomKey:
		totalOpts := len(m.setupWizard.keyCandidates) + 2
		switch msg.String() {
		case "esc":
			m.setupWizard.step = wizardStepModeSelect
			return m, nil
		case "up", "k":
			if m.setupWizard.keyCursor > 0 {
				m.setupWizard.keyCursor--
			}
			return m, nil
		case "down", "j":
			if m.setupWizard.keyCursor < totalOpts-1 {
				m.setupWizard.keyCursor++
			}
			return m, nil
		case "enter":
			if m.setupWizard.keyCursor < len(m.setupWizard.keyCandidates) {
				m.setupWizard.detectedKey = m.setupWizard.keyCandidates[m.setupWizard.keyCursor]
				m.setupWizard.step = wizardStepCustomBranch
				return m, nil
			} else if m.setupWizard.keyCursor == len(m.setupWizard.keyCandidates) {
				// Enter custom key path
				m.setupWizard.step = wizardStepCustomKeyInput
				m.setupWizard.keyInput.Focus()
				return m, textinput.Blink
			} else {
				// User explicitly chose to generate a dedicated gittuf key
				privPath, err := generateLocalSSHKey()
				if err != nil {
					m.setupWizard.step = wizardStepError
					m.setupWizard.err = fmt.Errorf("failed to generate dedicated gittuf SSH key: %w", err)
					return m, nil
				}
				m.setupWizard.detectedKey = privPath
				m.setupWizard.step = wizardStepCustomBranch
				return m, nil
			}
		}

	case wizardStepCustomKeyInput:
		switch msg.String() {
		case "esc":
			m.setupWizard.step = wizardStepCustomKey
			return m, nil
		case "enter":
			val := strings.TrimSpace(m.setupWizard.keyInput.Value())
			if val != "" {
				m.setupWizard.detectedKey = val
				m.setupWizard.step = wizardStepCustomBranch
				return m, nil
			}
			return m, nil
		default:
			var cmd tea.Cmd
			m.setupWizard.keyInput, cmd = m.setupWizard.keyInput.Update(msg)
			return m, cmd
		}

	case wizardStepCustomBranch:
		switch msg.String() {
		case "esc":
			m.setupWizard.step = wizardStepModeSelect
			return m, nil
		case "enter":
			val := strings.TrimSpace(m.setupWizard.branchInput.Value())
			if val != "" {
				m.setupWizard.defaultBranch = val
			}
			m.setupWizard.step = wizardStepApplying
			m.setupWizard.applyingMsg = fmt.Sprintf("Initializing gittuf policy & protecting '%s'...", m.setupWizard.defaultBranch)
			return m, tea.Batch(
				m.setupWizard.spinner.Tick,
				applySetupPolicyCmd(m.ctx, m.repo, m.setupWizard.detectedKey, m.setupWizard.defaultBranch),
			)
		default:
			var cmd tea.Cmd
			m.setupWizard.branchInput, cmd = m.setupWizard.branchInput.Update(msg)
			return m, cmd
		}

	case wizardStepError:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "esc":
			m.showSetupWizard = false
			m.readOnly = true
			m.footer = "Setup cancelled due to error. Exploring in read-only mode."
			return m, nil
		case "enter":
			m.setupWizard.step = wizardStepModeSelect
			m.setupWizard.err = nil
			return m, nil
		}
	}

	return m, nil
}

// renderSetupWizardDialog builds the centered Lazygit-style modal view.
func renderSetupWizardDialog(m model) string {
	w := m.setupWizard
	width := m.width - 10
	if width > 74 {
		width = 74
	}
	if width < 44 {
		width = 44
	}

	var content string

	switch w.step {
	case wizardStepModeSelect:
		keyDisplay := formatKeyLabel(w.detectedKey)
		if keyDisplay == "" {
			keyDisplay = "(None detected — select Custom Setup)"
		}

		branchDisplay := w.defaultBranch
		if branchDisplay == "" {
			branchDisplay = "main"
		}

		menuItems := []struct{ title, desc string }{
			{"[1] Quick Start (Recommended)", "Initializes root, targets, and protects branch in one step."},
			{"[2] Custom Setup", "Select a different signing key or customize branch name."},
			{"[3] Skip to Read-Only Mode", "Browse repository state in read-only mode without initializing."},
		}
		var menuLines []string
		for i, item := range menuItems {
			prefix := "  "
			style := setupNormalOptStyle
			if i == w.cursor {
				prefix = "▶ "
				style = setupSelectedOptStyle
			}
			menuLines = append(menuLines, style.Render(prefix+item.title), setupMutedStyle.Render("      "+item.desc), "")
		}

		headerLines := []string{
			setupTitleStyle.Render("Welcome to gittuf! 🛡️"),
			setupMutedStyle.Render("First-Run Repository Setup"),
			"",
			setupDescStyle.Render("No security policy was detected in this repository."),
			setupDescStyle.Render("Protect your repository in 1 click:"),
			"",
			fmt.Sprintf("  %s %s", setupMutedStyle.Render("Signing Key:   "), setupHighlightStyle.Render(keyDisplay)),
			fmt.Sprintf("  %s %s", setupMutedStyle.Render("Target Branch: "), setupHighlightStyle.Render(branchDisplay)),
			"",
		}
		footerLine := setupFooterStyle.Render("Press <enter> to proceed   │   <esc> to skip")
		content = lipgloss.JoinVertical(lipgloss.Left, append(append(headerLines, menuLines...), footerLine)...)

	case wizardStepCustomKey:
		var items []string
		items = append(items, setupTitleStyle.Render("Select Signing Key"))
		if len(w.keyCandidates) > 0 {
			items = append(items, setupDescStyle.Render("Choose from discovered local keys or specify custom:"), "")
		} else {
			items = append(items, setupDescStyle.Render("No signing keys detected automatically:"), "")
		}

		labels := make([]string, 0, len(w.keyCandidates)+2)
		for _, k := range w.keyCandidates {
			labels = append(labels, formatKeyLabel(k))
		}
		labels = append(labels, "✎  Enter custom key path or gpg:KEYID...", "⚡ Generate dedicated gittuf key (~/.ssh/gittuf_id_ed25519)")

		for i, label := range labels {
			itemStr := fmt.Sprintf("(%d) %s", i+1, label)
			if i == w.keyCursor {
				items = append(items, setupSelectedOptStyle.Render("▶ "+itemStr))
			} else {
				items = append(items, setupNormalOptStyle.Render("  "+itemStr))
			}
		}

		items = append(items, "", setupFooterStyle.Render("Press <enter> to select   │   <esc> back"))
		content = lipgloss.JoinVertical(lipgloss.Left, items...)

	case wizardStepCustomKeyInput:
		content = lipgloss.JoinVertical(lipgloss.Left,
			setupTitleStyle.Render("Custom Signing Key"),
			setupDescStyle.Render("Enter path to SSH key, GPG key (gpg:KEYID), or Sigstore ID:"),
			"",
			w.keyInput.View(),
			"",
			setupFooterStyle.Render("Press <enter> to confirm   │   <esc> back"),
		)

	case wizardStepCustomBranch:
		content = lipgloss.JoinVertical(lipgloss.Left,
			setupTitleStyle.Render("Confirm Protected Branch"),
			setupDescStyle.Render("Enter the primary branch name to protect with policy:"),
			"",
			w.branchInput.View(),
			"",
			setupFooterStyle.Render("Press <enter> to apply   │   <esc> back"),
		)

	case wizardStepApplying:
		content = lipgloss.JoinVertical(lipgloss.Center,
			"",
			fmt.Sprintf("%s %s", w.spinner.View(), setupHighlightStyle.Render(w.applyingMsg)),
			"",
			setupMutedStyle.Render("Creating root of trust, targets metadata, and initial RSL entry..."),
			"",
		)

	case wizardStepError:
		content = lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Foreground(lipgloss.Color("#f85149")).Bold(true).Render("Setup Failed"),
			"",
			setupDescStyle.Render(fmt.Sprintf("Error: %v", w.err)),
			"",
			setupFooterStyle.Render("Press <enter> to retry   │   <esc> to skip   │   <q> to quit"),
		)
	}

	return setupModalStyle.Width(width).Render(content)
}
