// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gittuf/gittuf/experimental/gittuf"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
	"github.com/gittuf/gittuf/pkg/gitstore"
)

type wizardStep int

const (
	wizardStepOverview wizardStep = iota
	wizardStepKey
	wizardStepCustomKeyInput
	wizardStepPrincipal
	wizardStepBranch
	wizardStepApplying
	wizardStepComplete
	wizardStepError
)

type setupWizardModel struct {
	step          wizardStep
	gitConfigKey  string
	selectedKey   string
	keyCursor     int
	principalName string
	targetBranch  string
	personInput   textinput.Model
	branchInput   textinput.Model
	keyInput      textinput.Model
	spinner       spinner.Model
	applyingMsg   string
	err           error
}

// setupDoneMsg is sent when policy bootstrapping completes.
type setupDoneMsg struct {
	signer     dsse.SignerVerifier
	signingKey string
	err        error
}

var (
	setupModalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorSuccessMsg)).
			Padding(1, 2)

	setupTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorSuccessMsg)).
			Bold(true)

	setupDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorRegularText))

	setupMutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorSubtext))

	setupHighlightStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorFocus)).
				Bold(true)

	setupSelectedOptStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorFocus)).
				Bold(true)

	setupNormalOptStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorRegularText))

	setupFooterStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorSuccessMsg)).
				PaddingTop(1)
)

func newSetupWizard(repo *gittuf.Repository, detectedKey string) setupWizardModel {
	var defaultPersonID string
	if repo != nil {
		if detectedKey == "" {
			if key, err := gittuf.LoadPublicKeyFromGitConfig(repo); err == nil {
				detectedKey = key.ID()
			}
		}
		if name, ok, err := repo.GetStorer().LookupConfig(gitstore.ConfigUserName); err == nil && ok {
			defaultPersonID = strings.Join(strings.Fields(name), "-")
		}
	}

	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(colorSuccessMsg))

	pi := textinput.New()
	pi.Placeholder = "e.g. jane-doe"
	pi.SetValue(defaultPersonID)

	bi := textinput.New()
	bi.Placeholder = "e.g. main"
	bi.SetValue("main")

	ki := textinput.New()
	ki.Placeholder = "Path to SSH key, gpg:KEYID, or fulcio:IDENTITY::ISSUER"
	ki.SetValue(detectedKey)

	return setupWizardModel{
		step:          wizardStepOverview,
		gitConfigKey:  detectedKey,
		selectedKey:   detectedKey,
		keyCursor:     0,
		principalName: defaultPersonID,
		targetBranch:  "main",
		personInput:   pi,
		branchInput:   bi,
		keyInput:      ki,
		spinner:       s,
	}
}

// hasPolicyOrStaging returns true if the repository already has an applied
// gittuf policy or an unapplied policy in the policy-staging reference.
func hasPolicyOrStaging(repo *gittuf.Repository) bool {
	if repo == nil {
		return false
	}

	if hasPolicy, err := repo.HasPolicy(); err == nil && hasPolicy {
		return true
	}

	if _, err := repo.GetStorer().GetReference(policy.PolicyStagingRef); err == nil {
		return true
	}

	return false
}

// applySetupPolicyCmd coordinates initializing the root of trust, top-level
// targets policy, and branch protection rule.
func applySetupPolicyCmd(ctx context.Context, repo *gittuf.Repository, keyRef, principalName, branchName string) tea.Cmd {
	return func() tea.Msg {
		branchName = strings.TrimPrefix(strings.TrimSpace(branchName), "refs/heads/")
		if branchName == "" {
			branchName = "main"
		}
		principalName = strings.Join(strings.Fields(principalName), "-")
		if principalName == "" {
			return setupDoneMsg{err: errors.New("no principal name specified")}
		}

		var (
			signer       dsse.SignerVerifier
			keyPrincipal tuf.Principal
			signingKey   string
			err          error
		)

		if gitKey, gitErr := gittuf.LoadPublicKeyFromGitConfig(repo); gitErr == nil && (keyRef == "" || keyRef == gitKey.ID()) {
			keyPrincipal = gitKey
			signer, err = gittuf.LoadSignerFromGitConfig(repo)
			if err != nil {
				return setupDoneMsg{err: fmt.Errorf("failed to load signer from Git config: %w", err)}
			}
		} else {
			if keyRef == "" {
				return setupDoneMsg{err: errors.New("no signing key specified")}
			}

			signer, err = gittuf.LoadSigner(repo, keyRef)
			if err != nil {
				return setupDoneMsg{err: fmt.Errorf("failed to load signer for key '%s': %w", keyRef, err)}
			}

			keyPrincipal, err = gittuf.LoadPublicKey(keyRef)
			if err != nil {
				return setupDoneMsg{err: fmt.Errorf("failed to load public key for '%s': %w", keyRef, err)}
			}
			signingKey = keyRef
		}

		key, ok := keyPrincipal.(*tufv02.Key)
		if !ok {
			return setupDoneMsg{err: errors.New("unexpected public key principal type")}
		}

		person := &tufv02.Person{
			PersonID:   principalName,
			PublicKeys: map[string]*tufv02.Key{key.ID(): key},
		}

		// 1. Initialize Root of Trust
		if err := repo.InitializeRoot(ctx, signer, false); err != nil {
			return setupDoneMsg{err: fmt.Errorf("failed to initialize root of trust: %w", err)}
		}

		// 2. Add Top-Level Targets Key
		if err := repo.AddTopLevelTargetsKey(ctx, signer, keyPrincipal, false, trustpolicyopts.WithRSLEntry()); err != nil {
			return setupDoneMsg{err: fmt.Errorf("failed to add top-level targets key: %w", err)}
		}

		// 3. Initialize Targets Role
		if err := repo.InitializeTargets(ctx, signer, policy.TargetsRoleName, false, trustpolicyopts.WithRSLEntry()); err != nil {
			return setupDoneMsg{err: fmt.Errorf("failed to initialize targets: %w", err)}
		}

		// 4. Authorize Person Principal in Targets
		if err := repo.AddPrincipalToTargets(ctx, signer, policy.TargetsRoleName, []tuf.Principal{person}, false, trustpolicyopts.WithRSLEntry()); err != nil {
			return setupDoneMsg{err: fmt.Errorf("failed to authorize principal in targets: %w", err)}
		}

		// 5. Add Branch Protection Delegation
		ruleName := "protect-" + branchName
		rulePattern := "git:refs/heads/" + branchName
		if err := repo.AddDelegation(ctx, signer, policy.TargetsRoleName, ruleName, []string{person.ID()}, []string{rulePattern}, 1, false, trustpolicyopts.WithRSLEntry()); err != nil {
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
		if _, refErr := repo.GetStorer().GetReference("refs/heads/" + branchName); refErr == nil {
			_ = repo.RecordRSLEntryForReference(ctx, "refs/heads/"+branchName, false)
		}

		return setupDoneMsg{
			signer:     signer,
			signingKey: signingKey,
			err:        nil,
		}
	}
}

// updateSetupWizard handles keyboard interactions when the setup modal is active.
func (m model) updateSetupWizard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.setupWizard.step {
	case wizardStepOverview:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "esc":
			m.showSetupWizard = false
			m.readOnly = true
			m.footer = "Setup skipped. Exploring in read-only mode."
			return m, nil
		case "enter":
			m.setupWizard.step = wizardStepKey
			return m, nil
		}

	case wizardStepKey:
		hasGitKey := m.setupWizard.gitConfigKey != ""
		maxCursor := 0
		if hasGitKey {
			maxCursor = 1
		}

		switch msg.String() {
		case "esc":
			m.setupWizard.step = wizardStepOverview
			return m, nil
		case "up", "k":
			if m.setupWizard.keyCursor > 0 {
				m.setupWizard.keyCursor--
			}
			return m, nil
		case "down", "j":
			if m.setupWizard.keyCursor < maxCursor {
				m.setupWizard.keyCursor++
			}
			return m, nil
		case "enter":
			if hasGitKey && m.setupWizard.keyCursor == 0 {
				m.setupWizard.selectedKey = m.setupWizard.gitConfigKey
				m.setupWizard.step = wizardStepPrincipal
				m.setupWizard.personInput.Focus()
				return m, textinput.Blink
			}
			m.setupWizard.step = wizardStepCustomKeyInput
			m.setupWizard.keyInput.Focus()
			return m, textinput.Blink
		}

	case wizardStepCustomKeyInput:
		switch msg.String() {
		case "esc":
			m.setupWizard.step = wizardStepKey
			return m, nil
		case "enter":
			val := strings.TrimSpace(m.setupWizard.keyInput.Value())
			if val != "" {
				m.setupWizard.selectedKey = val
				m.setupWizard.step = wizardStepPrincipal
				m.setupWizard.personInput.Focus()
				return m, textinput.Blink
			}
			return m, nil
		default:
			var cmd tea.Cmd
			m.setupWizard.keyInput, cmd = m.setupWizard.keyInput.Update(msg)
			return m, cmd
		}

	case wizardStepPrincipal:
		switch msg.String() {
		case "esc":
			m.setupWizard.step = wizardStepKey
			return m, nil
		case "enter":
			val := strings.Join(strings.Fields(m.setupWizard.personInput.Value()), "-")
			if val != "" {
				m.setupWizard.principalName = val
				m.setupWizard.step = wizardStepBranch
				m.setupWizard.branchInput.Focus()
				return m, textinput.Blink
			}
			return m, nil
		default:
			var cmd tea.Cmd
			m.setupWizard.personInput, cmd = m.setupWizard.personInput.Update(msg)
			return m, cmd
		}

	case wizardStepBranch:
		switch msg.String() {
		case "esc":
			m.setupWizard.step = wizardStepPrincipal
			m.setupWizard.personInput.Focus()
			return m, textinput.Blink
		case "enter":
			val := strings.TrimPrefix(strings.TrimSpace(m.setupWizard.branchInput.Value()), "refs/heads/")
			if val == "" {
				val = "main"
			}
			m.setupWizard.targetBranch = val
			m.setupWizard.step = wizardStepApplying
			m.setupWizard.applyingMsg = fmt.Sprintf("Initializing gittuf policy & protecting '%s'...", m.setupWizard.targetBranch)
			return m, tea.Batch(
				m.setupWizard.spinner.Tick,
				applySetupPolicyCmd(m.ctx, m.repo, m.setupWizard.selectedKey, m.setupWizard.principalName, m.setupWizard.targetBranch),
			)
		default:
			var cmd tea.Cmd
			m.setupWizard.branchInput, cmd = m.setupWizard.branchInput.Update(msg)
			return m, cmd
		}

	case wizardStepComplete:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "enter", "esc":
			m.showSetupWizard = false
			m.footer = "Repository policy initialized! Protected branch: " + m.setupWizard.targetBranch
			return m, nil
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
			m.setupWizard.step = wizardStepOverview
			m.setupWizard.err = nil
			return m, nil
		}
	}

	return m, nil
}

// renderSetupWizardDialog builds the centered setup modal view.
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
	case wizardStepOverview:
		lines := []string{
			setupTitleStyle.Render("Welcome to gittuf!"),
			setupMutedStyle.Render("First-Run Repository Setup"),
			"",
			setupDescStyle.Render("No gittuf policy was detected. This guided setup will apply:"),
			fmt.Sprintf("  %s %s", setupHighlightStyle.Render("- Root of Trust:"), setupDescStyle.Render("root metadata with your key (threshold: 1)")),
			fmt.Sprintf("  %s %s", setupHighlightStyle.Render("- Top-Level Policy:"), setupDescStyle.Render("'targets' role with your key (threshold: 1)")),
			fmt.Sprintf("  %s %s", setupHighlightStyle.Render("- Branch Protection:"), setupDescStyle.Render("protect-<branch> rule (threshold: 1)")),
			"",
			setupMutedStyle.Render("Note: More advanced policies require the gittuf CLI for now."),
			"",
			setupFooterStyle.Render("Press <enter> to begin setup   |   <esc> to skip   |   <q> to quit"),
		}
		content = lipgloss.JoinVertical(lipgloss.Left, lines...)

	case wizardStepKey:
		var items []string
		items = append(items, setupTitleStyle.Render("Step 1 of 3: Select Signing Key"), "")

		var labels []string
		if w.gitConfigKey != "" {
			items = append(items,
				setupDescStyle.Render("Detected signing key from Git configuration:"),
				fmt.Sprintf("  %s", setupHighlightStyle.Render(w.gitConfigKey)),
				"",
				setupDescStyle.Render("Use this key for Root of Trust and Top-Level Policy, or override it:"),
				"",
			)
			labels = []string{
				fmt.Sprintf("Use key from Git config (%s)", w.gitConfigKey),
				"Override signing key",
			}
		} else {
			items = append(items,
				setupDescStyle.Render("No signing key was found in your Git configuration."),
				setupDescStyle.Render("Select below to specify a signing key for Root of Trust and Policy:"),
				"",
			)
			labels = []string{
				"Specify signing key",
			}
		}

		for i, label := range labels {
			itemStr := fmt.Sprintf("(%d) %s", i+1, label)
			if i == w.keyCursor {
				items = append(items, setupSelectedOptStyle.Render("> "+itemStr))
			} else {
				items = append(items, setupNormalOptStyle.Render("  "+itemStr))
			}
		}

		items = append(items, "", setupFooterStyle.Render("Press <enter> to select   |   <esc> back"))
		content = lipgloss.JoinVertical(lipgloss.Left, items...)

	case wizardStepCustomKeyInput:
		content = lipgloss.JoinVertical(lipgloss.Left,
			setupTitleStyle.Render("Step 1 of 3: Override Signing Key"),
			setupDescStyle.Render("Enter path to SSH key, GPG key (gpg:KEYID), or Sigstore (fulcio:IDENTITY::ISSUER):"),
			"",
			w.keyInput.View(),
			"",
			setupFooterStyle.Render("Press <enter> to confirm   |   <esc> back"),
		)

	case wizardStepPrincipal:
		content = lipgloss.JoinVertical(lipgloss.Left,
			setupTitleStyle.Render("Step 2 of 3: Enter Principal Name"),
			setupDescStyle.Render("Enter a name (Person ID) for the principal to be added to the policy:"),
			"",
			w.personInput.View(),
			"",
			setupFooterStyle.Render("Press <enter> to confirm   |   <esc> back"),
		)

	case wizardStepBranch:
		branchPreview := strings.TrimPrefix(strings.TrimSpace(w.branchInput.Value()), "refs/heads/")
		if branchPreview == "" {
			branchPreview = "main"
		}
		keyLabel := w.selectedKey
		personLabel := w.principalName

		content = lipgloss.JoinVertical(lipgloss.Left,
			setupTitleStyle.Render("Step 3 of 3: Confirm Protected Branch & Policy"),
			setupDescStyle.Render("Enter the branch name to protect and review the policy to be applied:"),
			"",
			w.branchInput.View(),
			"",
			setupMutedStyle.Render("Policy summary to be applied:"),
			fmt.Sprintf("  %s %s", setupMutedStyle.Render("Root of Trust:   "), setupHighlightStyle.Render(keyLabel+" (threshold: 1)")),
			fmt.Sprintf("  %s %s", setupMutedStyle.Render("Top-Level Policy:"), setupHighlightStyle.Render(keyLabel+" (threshold: 1)")),
			fmt.Sprintf("  %s %s", setupMutedStyle.Render("Policy Principal:"), setupHighlightStyle.Render(fmt.Sprintf("%s (%s)", personLabel, keyLabel))),
			fmt.Sprintf("  %s %s", setupMutedStyle.Render("Branch Rule:     "), setupHighlightStyle.Render(fmt.Sprintf("protect-%s -> git:refs/heads/%s", branchPreview, branchPreview))),
			"",
			setupFooterStyle.Render("Press <enter> to apply policy   |   <esc> back"),
		)

	case wizardStepApplying:
		content = lipgloss.JoinVertical(lipgloss.Center,
			"",
			fmt.Sprintf("%s %s", w.spinner.View(), setupHighlightStyle.Render(w.applyingMsg)),
			"",
			setupMutedStyle.Render("Creating root of trust, top-level targets policy, and initial RSL entry..."),
			"",
		)

	case wizardStepComplete:
		keyLabel := w.selectedKey
		content = lipgloss.JoinVertical(lipgloss.Left,
			setupTitleStyle.Render("Policy Applied Successfully"),
			"",
			setupDescStyle.Render("The gittuf security policy has been initialized and applied:"),
			"",
			fmt.Sprintf("  %s %s", setupMutedStyle.Render("Root of Trust:   "), setupHighlightStyle.Render("Initialized with "+keyLabel+" (threshold: 1)")),
			fmt.Sprintf("  %s %s", setupMutedStyle.Render("Top-Level Policy:"), setupHighlightStyle.Render("Initialized 'targets' role with "+keyLabel+" (threshold: 1)")),
			fmt.Sprintf("  %s %s", setupMutedStyle.Render("Policy Principal:"), setupHighlightStyle.Render(fmt.Sprintf("%s (%s)", w.principalName, keyLabel))),
			fmt.Sprintf("  %s %s", setupMutedStyle.Render("Protected Branch:"), setupHighlightStyle.Render(fmt.Sprintf("git:refs/heads/%s (rule: protect-%s)", w.targetBranch, w.targetBranch))),
			"",
			setupFooterStyle.Render("Press <enter> to open TUI   |   <q> to quit"),
		)

	case wizardStepError:
		content = lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Foreground(lipgloss.Color(colorErrorMsg)).Bold(true).Render("Setup Failed"),
			"",
			setupDescStyle.Render(fmt.Sprintf("Error: %v", w.err)),
			"",
			setupFooterStyle.Render("Press <enter> to retry   |   <esc> to skip   |   <q> to quit"),
		)
	}

	return setupModalStyle.Width(width).Render(content)
}
