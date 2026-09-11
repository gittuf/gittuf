// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

type screen int

const (
	screenLoading                    screen = iota // Loading screen shown on startup
	screenChoice                                   // Initial menu
	screenPolicy                                   // Menu for Policy operations
	screenPolicyRules                              // Rule management screen
	screenPolicyAddRule                            // Form: add a new policy rule
	screenPolicyEditRule                           // Form: edit selected rule (prefilled)
	screenPolicyPrincipals                         // Principals management screen
	screenPolicyPrincipalsForm                     // Form: Add/Edit principal or add key
	screenPolicyLifecycle                          // Menu for Policy lifecycle operations
	screenPolicyLifecycleForm                      // Form: policy lifecycle operation options
	screenTrust                                    // Menu for Trust operations
	screenTrustGlobalRules                         // Global rule management screen
	screenTrustAddGlobalRule                       // Form: add a new global rule
	screenTrustEditGlobalRule                      // Form: edit selected global rule
	screenTrustKeysThresholds                      // Manage root/policy keys and thresholds
	screenTrustKeyForm                             // Form: add/remove trust key
	screenTrustThresholdForm                       // Form: update trust threshold
	screenTrustHooks                               // Hook management screen
	screenTrustRemoveHookForm                      // Form: remove a trust hook
	screenTrustAddHookForm                         // Form: add a trust hook
	screenTrustUpdateHookForm                      // Form: update a trust hook
	screenTrustPropagation                         // Propagation directive management screen
	screenTrustAddPropagationForm                  // Form: add a propagation directive
	screenTrustUpdatePropagationForm               // Form: update a propagation directive
	screenTrustRemovePropagationForm               // Form: remove a propagation directive
	screenTrustGitHubApp                           // GitHub App management screens.
	screenTrustAddGitHubAppForm                    // Form: add a trusted GitHub App
	screenTrustGitHubAppActionForm                 // Form: manage GitHub App trust actions
	screenTrustLifecycle                           // Trust lifecycle operations.
	screenTrustRepoNetwork                         // Repo/network management screens.
	screenTrustRepoForm                            // Form: add or update trust repo settings
	screenTrustRepoLocationForm                    // Form: set the repository location
	screenVerify                                   // Menu for Verify operations
	screenVerifyRefForm                            // Form: verify reference
	screenVerifyMergeableForm                      // Form: verify mergeability
	screenHelp                                     // Generic help screen displaying keybindings.
)

type item struct {
	title, desc string
}

type errorDialog struct {
	title   string
	message string
}

// Note: virtual methods must be implemented for the item struct
// Title returns the title of the item.
func (i item) Title() string { return i.title }

// Description returns the description of the item.
func (i item) Description() string { return i.desc }

// FilterValue returns the value to filter on.
func (i item) FilterValue() string { return i.title }

type model struct {
	ctx                        context.Context
	screen                     screen
	spinner                    spinner.Model
	logViewport                viewport.Model
	homeScreen                 homeScreen
	helpScreen                 helpScreen
	policyScreen               policyScreen
	policyLifecycleScreen      policyLifecycleScreen
	trustScreen                trustScreen
	policyRulesScreen          policyRulesScreen
	trustGlobalRulesScreen     trustGlobalRulesScreen
	policyPrincipalsScreen     policyPrincipalsScreen
	policyPrincipalsFormScreen policyPrincipalsFormScreen
	trustKeysScreen            trustKeysThresholdsScreen
	trustLifecycleScreen       trustLifecycleScreen
	trustHookScreen            trustHookScreen
	trustPropagationScreen     trustPropagationScreen
	trustGitHubAppScreen       trustGitHubAppScreen
	trustRepoNetworkScreen     trustRepoNetworkScreen
	verifyScreen               verifyScreen
	verifyRefScreen            verifyRefScreen
	verifyMergeableScreen      verifyMergeableScreen
	cursorMode                 cursor.Mode
	repo                       *gittuf.Repository
	signer                     dsse.SignerVerifier
	policyName                 string
	options                    *options
	footer                     string
	errorMsg                   string
	errorDialog                *errorDialog
	readOnly                   bool
	width                      int
	height                     int
	showHelp                   bool
	signerError                string
	verifying                  bool
	loadingMsg                 string
	logsBuf                    *strings.Builder
	logCh                      chan string
	showDiffOverlay            bool
	diffViewport               viewport.Model
}

// initDoneMsg carries the result of the asynchronous TUI initialization.
type initDoneMsg struct {
	repo        *gittuf.Repository
	signer      dsse.SignerVerifier
	rules       []rule
	globalRules []globalRule
	readOnly    bool
	footer      string
	signerError string
	err         error
}

// verifyResultMsg carries the result of an async verification command.
type verifyResultMsg struct {
	err        error
	successMsg string
}

// logBatchMsg carries a batch of log lines for the verification viewport.
type logBatchMsg struct {
	lines []string
}

// inputField describes a single text input's placeholder and prompt label.
type inputField struct {
	placeholder string
	prompt      string
}

// newDelegate creates a styled list delegate for use in all list.Model instances.
func newDelegate(height int) list.DefaultDelegate {
	d := list.NewDefaultDelegate()
	d.SetHeight(height)
	d.Styles.SelectedTitle = selectedItemStyle
	d.Styles.SelectedDesc = selectedItemStyle
	d.Styles.NormalTitle = itemStyle
	d.Styles.NormalDesc = itemStyle
	return d
}

// newMenuList creates a configured list.Model with default settings.
func newMenuList(title string, items []list.Item, delegate list.DefaultDelegate) list.Model {
	l := list.New(items, delegate, 0, 0)
	l.Title = title
	l.Styles.Title = titleStyle
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	return l
}

func trustMenuItems(readOnly bool) []list.Item {
	items := []list.Item{
		item{title: "View Global Rules", desc: "View and manage global rules"},
		item{title: "Hooks", desc: "Manage trust hooks"},
		item{title: "Propagation", desc: "Manage propagation directives"},
	}
	if !readOnly {
		items = append(items,
			item{title: "Keys & Thresholds", desc: "Manage root keys, policy keys, and thresholds"},
			item{title: "GitHub App", desc: "Manage trusted GitHub App settings"},
			item{title: "Lifecycle", desc: "Stage, sign, and apply trust changes"},
			item{title: "Repo/Network", desc: "Manage controller, network, and repository settings"},
		)
	}
	return items
}

func trustKeysMenuItems(readOnly bool) []list.Item {
	if readOnly {
		return nil
	}
	return []list.Item{
		item{title: "Add Root Key", desc: "Add a trusted root key"},
		item{title: "Remove Root Key", desc: "Remove a trusted root key"},
		item{title: "Add Policy Key", desc: "Add a trusted policy key"},
		item{title: "Remove Policy Key", desc: "Remove a trusted policy key"},
		item{title: "Update Root Threshold", desc: "Set the root signature threshold"},
		item{title: "Update Policy Threshold", desc: "Set the policy signature threshold"},
	}
}

func trustLifecycleMenuItems(readOnly bool) []list.Item {
	if readOnly {
		return nil
	}
	return []list.Item{
		item{title: "Stage Trust Changes", desc: "Stage trust changes for signing"},
		item{title: "Sign Trust Changes", desc: "Sign staged trust changes"},
		item{title: "Apply Trust Changes", desc: "Apply signed trust changes to the policy file"},
	}
}

func trustHooksMenuItems(readOnly bool) []list.Item {
	items := []list.Item{
		item{title: "List Hooks", desc: "List all configured hooks"},
	}
	if !readOnly {
		items = append(items,
			item{title: "Add Hook", desc: "Add a new hook"},
			item{title: "Update Hook", desc: "Update an existing hook"},
			item{title: "Remove Hook", desc: "Remove an existing hook"},
		)
	}
	return items
}

func trustPropagationMenuItems(readOnly bool) []list.Item {
	items := []list.Item{
		item{title: "List Directives", desc: "List all configured propagation directives"},
	}
	if !readOnly {
		items = append(items,
			item{title: "Add Directive", desc: "Add a new propagation directive"},
			item{title: "Update Directive", desc: "Update an existing propagation directive"},
			item{title: "Remove Directive", desc: "Remove an existing propagation directive"},
		)
	}
	return items
}

func trustGitHubAppMenuItems(readOnly bool) []list.Item {
	if readOnly {
		return nil
	}
	return []list.Item{
		item{title: "Add GitHub App", desc: "Add a trusted GitHub App key"},
		item{title: "Remove GitHub App", desc: "Remove a trusted GitHub App"},
		item{title: "Enable App Approvals", desc: "Mark GitHub App approvals as trusted"},
		item{title: "Disable App Approvals", desc: "Mark GitHub App approvals as untrusted"},
	}
}

func trustRepoNetworkMenuItems(readOnly bool) []list.Item {
	if readOnly {
		return nil
	}
	return []list.Item{
		item{title: "Add Controller Repository", desc: "Add a controller repository"},
		item{title: "Add Network Repository", desc: "Add a network repository"},
		item{title: "Set Repository Location", desc: "Set this repository's canonical location"},
		item{title: "Make Controller", desc: "Mark this repository as a controller"},
	}
}

// initInputs creates a slice of text inputs from field definitions.
// The first field is focused; the rest are blurred.
func initInputs(fields []inputField) []textinput.Model {
	inputs := make([]textinput.Model, len(fields))
	for i, f := range fields {
		t := textinput.New()
		t.Cursor.Style = cursorStyle
		t.CharLimit = 0
		t.Placeholder = f.placeholder
		t.Prompt = f.prompt
		if i == 0 {
			t.Focus()
			t.PromptStyle = focusedStyle
			t.TextStyle = focusedStyle
		} else {
			t.Blur()
			t.PromptStyle = blurredStyle
			t.TextStyle = blurredStyle
		}
		inputs[i] = t
	}
	return inputs
}

func (m *model) openErrorDialog(title, message string) {
	m.footer = ""
	m.errorMsg = ""
	m.errorDialog = &errorDialog{
		title:   title,
		message: message,
	}
}

func (m *model) closeErrorDialog() {
	m.errorDialog = nil
}

// initialModel returns a lightweight loading model for the Terminal UI.
// All heavy work (repo I/O, signing key, rules) is deferred to loadRepoCmd.
func initialModel(ctx context.Context, o *options) model {
	s := spinner.New()
	s.Spinner = spinner.Dot

	delegate := newDelegate(2)
	delegateMultiline := newDelegate(4)

	m := model{
		ctx:         ctx,
		screen:      screenLoading,
		spinner:     s,
		cursorMode:  cursor.CursorBlink,
		policyName:  o.policyName,
		options:     o,
		logViewport: viewport.New(80, 20),
		logsBuf:     &strings.Builder{},

		homeScreen: homeScreen{
			choiceList: newMenuList("gittuf TUI", []list.Item{
				item{title: "Policy", desc: "View and manage gittuf Policy"},
				item{title: "Trust", desc: "View and manage gittuf Root of Trust"},
				item{title: "Verify", desc: "Verify references and mergeability"},
			}, delegate),
		},
		policyScreen: policyScreen{
			policyScreenList: newMenuList("gittuf Policy Operations", []list.Item{
				item{title: "View Rules", desc: "View and manage policy rules"},
				item{title: "Manage Principals", desc: "View and manage policy principals and keys"},
				item{title: "Manage Lifecycle", desc: "Initialize, sign, stage, apply, discard, pull or push policy changes"},
			}, delegate),
		},
		policyLifecycleScreen: policyLifecycleScreen{
			list: newMenuList("Policy Lifecycle", []list.Item{
				item{title: "Initialize Policy", desc: "Initialize a new gittuf policy file"},
				item{title: "Increment Version", desc: "Increment the version of the specified rule file metadata"},
				item{title: "Sign Policy", desc: "Sign the specified policy file"},
				item{title: "Stage Changes", desc: "Stage local policy changes"},
				item{title: "Apply Changes", desc: "Apply staged policy changes"},
				item{title: "Discard Changes", desc: "Discard staged policy changes"},
				item{title: "Pull Policy", desc: "Pull policy from a remote repository"},
				item{title: "Push Policy", desc: "Push policy to a remote repository"},
			}, delegate),
		},
		trustScreen: trustScreen{
			trustScreenList: newMenuList("gittuf Trust Operations", trustMenuItems(o.readOnly), delegate),
		},
		policyRulesScreen: policyRulesScreen{
			ruleList: newMenuList("Policy Rules", []list.Item{}, delegate),
		},
		trustGlobalRulesScreen: trustGlobalRulesScreen{
			globalRuleList: newMenuList("Global Rules", []list.Item{}, delegate),
		},
		trustKeysScreen: trustKeysThresholdsScreen{
			operationList: newMenuList("Keys & Thresholds", trustKeysMenuItems(o.readOnly), delegate),
		},
		trustLifecycleScreen: trustLifecycleScreen{
			operationList: newMenuList("Trust Lifecycle", trustLifecycleMenuItems(o.readOnly), delegate),
		},
		trustHookScreen: trustHookScreen{
			operationList: newMenuList("Trust Hooks", trustHooksMenuItems(o.readOnly), delegate),
		},
		trustPropagationScreen: trustPropagationScreen{
			operationList: newMenuList("Trust Propagation", trustPropagationMenuItems(o.readOnly), delegate),
		},
		trustGitHubAppScreen: trustGitHubAppScreen{
			operationList: newMenuList("Trust GitHub App", trustGitHubAppMenuItems(o.readOnly), delegate),
		},
		trustRepoNetworkScreen: trustRepoNetworkScreen{
			operationList: newMenuList("Trust Repo/Network", trustRepoNetworkMenuItems(o.readOnly), delegate),
		},
		policyPrincipalsScreen: policyPrincipalsScreen{
			list: newMenuList("Policy Principals", []list.Item{}, delegateMultiline),
		},
		verifyScreen: verifyScreen{
			choiceList: newMenuList("gittuf Verify Operations", []list.Item{
				item{title: "Verify Reference", desc: "Verify a specific reference"},
				item{title: "Verify Mergeability", desc: "Check if a feature branch can be merged"},
				item{title: "Verify Network", desc: "Verify state of network repositories"},
			}, delegate),
		},
	}

	return m
}

func (m *model) applyReadOnlyMenus() {
	m.trustScreen.trustScreenList.SetItems(trustMenuItems(m.readOnly))
	m.trustKeysScreen.operationList.SetItems(trustKeysMenuItems(m.readOnly))
	m.trustLifecycleScreen.operationList.SetItems(trustLifecycleMenuItems(m.readOnly))
	m.trustHookScreen.operationList.SetItems(trustHooksMenuItems(m.readOnly))
	m.trustPropagationScreen.operationList.SetItems(trustPropagationMenuItems(m.readOnly))
	m.trustGitHubAppScreen.operationList.SetItems(trustGitHubAppMenuItems(m.readOnly))
	m.trustRepoNetworkScreen.operationList.SetItems(trustRepoNetworkMenuItems(m.readOnly))
}

// resizeLists updates all list sizes to match the available content area, accounting for
// the status bar, renderWithMargin margins (v=2), borders, footer, and readOnly/signerError state.
// This must be called both on WindowSizeMsg and after initDoneMsg updates readOnly/signerError.
func (m *model) resizeLists() {
	// Width: subtract horizontal margin frame (h=4) + box padding+border (2+2=4) = 8
	innerWidth := m.width - 8
	if innerWidth < 0 {
		innerWidth = 0
	}

	// Height offsets must match view.go renderScreen's boxHeight formula:
	// boxHeight = m.height - v(2) - heightOffset_view
	// so innerHeight = m.height - (v + heightOffset_view) = m.height - heightOffset_here
	//
	// Height offsets must match view.go renderScreen's boxHeight formula:
	// boxHeight = m.height - v(2) - heightOffset_view
	// so innerHeight = m.height - (v + heightOffset_view) = m.height - heightOffset_here
	//
	// Normal:   heightOffset_view=7 → innerHeight = m.height - 9
	// readOnly: heightOffset_view=9 → innerHeight = m.height - 11
	// readOnly+signerError: heightOffset_view = 7 + signerNoticeLines (dynamic)
	//   → innerHeight = m.height - (2 + 7 + noticeLines) = m.height - 9 - noticeLines
	bottomHeight := 1
	footerBox := renderFooterBox(*m)
	if footerBox != "" {
		bottomHeight += strings.Count(footerBox, "\n") + 1
	}
	errorMsg := renderErrorMsg(m.errorMsg)
	if errorMsg != "" {
		bottomHeight += strings.Count(errorMsg, "\n") + 1
	}

	innerHeight := m.height - 6 - bottomHeight
	if innerHeight < 0 {
		innerHeight = 0
	}

	m.homeScreen.choiceList.SetSize(innerWidth, innerHeight)
	m.policyScreen.policyScreenList.SetSize(innerWidth, innerHeight)
	m.policyLifecycleScreen.list.SetSize(innerWidth, innerHeight)
	m.trustScreen.trustScreenList.SetSize(innerWidth, innerHeight)
	m.policyRulesScreen.ruleList.SetSize(innerWidth, innerHeight)
	m.trustGlobalRulesScreen.globalRuleList.SetSize(innerWidth, innerHeight)
	m.policyPrincipalsScreen.list.SetSize(innerWidth, innerHeight)
	m.trustKeysScreen.operationList.SetSize(innerWidth, innerHeight)
	m.trustHookScreen.operationList.SetSize(innerWidth, innerHeight)
	m.trustLifecycleScreen.operationList.SetSize(innerWidth, innerHeight)
	m.trustHookScreen.operationList.SetSize(innerWidth, innerHeight)
	m.trustPropagationScreen.operationList.SetSize(innerWidth, innerHeight)
	m.trustGitHubAppScreen.operationList.SetSize(innerWidth, innerHeight)
	m.trustRepoNetworkScreen.operationList.SetSize(innerWidth, innerHeight)
	m.verifyScreen.choiceList.SetSize(innerWidth, innerHeight)
}

// loadRepoCmd performs all heavy TUI initialization asynchronously and sends
// an initDoneMsg back to the program when complete.
func loadRepoCmd(ctx context.Context, o *options) tea.Cmd {
	return func() tea.Msg {
		repo, err := gittuf.LoadRepository(".")
		if err != nil {
			return initDoneMsg{err: err}
		}

		readOnly := o.readOnly
		var signer dsse.SignerVerifier
		var footer string
		var signerError string

		if !readOnly {
			signer, err = gittuf.LoadSigner(repo, o.p.SigningKey)
			if err != nil {
				readOnly = true
				if errors.Is(err, gittuf.ErrSigningKeyNotSpecified) {
					footer = "Read-only mode. Press 'h' to view help."
				} else {
					mErr := strings.TrimPrefix(err.Error(), "failed to load signing key from Git config: ")
					signerError = fmt.Sprintf("Signing key issue: %s", mErr)
					footer = "Read-only mode. Press 'h' to view help."
				}
			}
		}

		return initDoneMsg{
			repo:        repo,
			signer:      signer,
			rules:       getCurrRules(ctx, o),
			globalRules: getGlobalRules(ctx, o),
			readOnly:    readOnly,
			footer:      footer,
			signerError: signerError,
		}
	}
}

// Init starts the spinner tick and kicks off async repo loading.
func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick, loadRepoCmd(m.ctx, m.options))
}

// currentScreenTitle returns the status bar title for the active screen.
func (m *model) currentScreenTitle() string {
	switch m.screen {
	case screenChoice:
		return "Home"
	case screenPolicy:
		return "Home › Policy"
	case screenPolicyRules:
		return "Home › Policy › Rules"
	case screenPolicyAddRule:
		return "Home › Policy › Rules › Add"
	case screenPolicyEditRule:
		return "Home › Policy › Rules › Edit"
	case screenPolicyPrincipals:
		return "Home › Policy › Principals"
	case screenPolicyPrincipalsForm:
		return "Home › Policy › Principals › Edit"
	case screenPolicyLifecycle, screenPolicyLifecycleForm:
		return "Home › Policy › Lifecycle"
	case screenTrust:
		return "Home › Trust"
	case screenTrustGlobalRules, screenTrustAddGlobalRule, screenTrustEditGlobalRule:
		return "Home › Trust › Global Rules"
	case screenTrustKeysThresholds, screenTrustKeyForm, screenTrustThresholdForm:
		return "Home › Trust › Keys & Thresholds"
	case screenTrustHooks, screenTrustAddHookForm, screenTrustUpdateHookForm, screenTrustRemoveHookForm:
		return "Home › Trust › Hooks"
	case screenTrustLifecycle:
		return "Home › Trust › Lifecycle"
	case screenTrustPropagation, screenTrustAddPropagationForm, screenTrustUpdatePropagationForm, screenTrustRemovePropagationForm:
		return "Home › Trust › Propagation"
	case screenTrustGitHubApp, screenTrustAddGitHubAppForm, screenTrustGitHubAppActionForm:
		return "Home › Trust › GitHub App"
	case screenTrustRepoNetwork, screenTrustRepoForm, screenTrustRepoLocationForm:
		return "Home › Trust › Repo/Network"
	case screenVerify:
		return "Home › Verify"
	case screenVerifyRefForm:
		return "Home › Verify › Ref"
	case screenVerifyMergeableForm:
		return "Home › Verify › Mergeable"
	case screenHelp:
		return "Help"
	default:
		return "Home"
	}
}

func (m *model) toggleDiffOverlay() {
	if m.showDiffOverlay {
		m.showDiffOverlay = false
		return
	}
	m.showDiffOverlay = true
	w := m.width - 10
	h := m.height - 8
	if w < 30 {
		w = 30
	}
	if h < 6 {
		h = 6
	}
	m.diffViewport = viewport.New(w, h)
	m.diffViewport.SetContent(m.generateStagedDiff())
}

func (m *model) generateStagedDiff() string {
	var b strings.Builder

	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Bold(true)
	addStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#28A745"))
	delStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5252"))
	modStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E0AF68"))
	sectionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorFocus)).Bold(true)
	subtextStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorSubtext))

	b.WriteString(headerStyle.Render("--- Active Policy (refs/gittuf/policy)\n+++ Staged Policy Changes (refs/gittuf/policy-staging)") + "\n\n")

	repo := m.repo
	if repo == nil {
		var err error
		repo, err = gittuf.LoadRepository(".")
		if err != nil {
			b.WriteString("No staged policy changes detected.\n\n")
			b.WriteString(subtextStyle.Render("Tip: Add, edit, or remove policy rules & principals, then press 'v' to review unapplied staged changes."))
			return b.String()
		}
	}

	gitRepo := repo.GetGitRepository()
	policyTip, errPolicy := gitRepo.GetReference(policy.PolicyRef)
	stagingTip, errStaging := gitRepo.GetReference(policy.PolicyStagingRef)

	// If staging ref does not exist, nothing is staged.
	if errStaging != nil {
		b.WriteString("No staged policy changes detected.\n\n")
		b.WriteString(subtextStyle.Render("Policy staging reference (refs/gittuf/policy-staging) does not exist."))
		return b.String()
	}

	// If policy ref exists and tips are identical, staging is fully in sync with policy.
	if errPolicy == nil && policyTip.Equal(stagingTip) {
		b.WriteString("No staged policy changes detected.\n\n")
		b.WriteString(subtextStyle.Render("Policy staging is up to date with active policy (tips are identical)."))
		return b.String()
	}

	var activeRules []rule
	if errPolicy == nil {
		activeRules = getRulesForRef(m.ctx, repo, policy.PolicyRef)
	}
	stagedRules := getRulesForRef(m.ctx, repo, policy.PolicyStagingRef)

	var activeGlobalRules []globalRule
	if errPolicy == nil {
		activeGlobalRules = getGlobalRulesForRef(m.ctx, repo, policy.PolicyRef)
	}
	stagedGlobalRules := getGlobalRulesForRef(m.ctx, repo, policy.PolicyStagingRef)

	policyName := policy.TargetsRoleName
	if m.options != nil && m.options.policyName != "" {
		policyName = m.options.policyName
	}
	var activePrincipals []tuf.Principal
	if errPolicy == nil {
		activePrincipals = getPrincipalsForRef(m.ctx, repo, policy.PolicyRef, policyName)
	}
	stagedPrincipals := getPrincipalsForRef(m.ctx, repo, policy.PolicyStagingRef, policyName)

	activeRuleMap := make(map[string]rule, len(activeRules))
	for _, r := range activeRules {
		activeRuleMap[r.name] = r
	}
	stagedRuleMap := make(map[string]rule, len(stagedRules))
	for _, r := range stagedRules {
		stagedRuleMap[r.name] = r
	}

	var addedRules []rule
	var removedRules []rule
	type modifiedRule struct {
		oldRule rule
		newRule rule
	}
	var modifiedRules []modifiedRule

	for _, sr := range stagedRules {
		if ar, exists := activeRuleMap[sr.name]; !exists {
			addedRules = append(addedRules, sr)
		} else if ar.pattern != sr.pattern || ar.threshold != sr.threshold || ar.key != sr.key {
			modifiedRules = append(modifiedRules, modifiedRule{oldRule: ar, newRule: sr})
		}
	}
	for _, ar := range activeRules {
		if _, exists := stagedRuleMap[ar.name]; !exists {
			removedRules = append(removedRules, ar)
		}
	}

	activeGRMap := make(map[string]globalRule, len(activeGlobalRules))
	for _, gr := range activeGlobalRules {
		activeGRMap[gr.ruleName] = gr
	}
	stagedGRMap := make(map[string]globalRule, len(stagedGlobalRules))
	for _, gr := range stagedGlobalRules {
		stagedGRMap[gr.ruleName] = gr
	}

	var addedGRs []globalRule
	var removedGRs []globalRule
	type modifiedGR struct {
		oldGR globalRule
		newGR globalRule
	}
	var modifiedGRs []modifiedGR

	for _, sgr := range stagedGlobalRules {
		if agr, exists := activeGRMap[sgr.ruleName]; !exists {
			addedGRs = append(addedGRs, sgr)
		} else if agr.ruleType != sgr.ruleType || agr.threshold != sgr.threshold || strings.Join(agr.rulePatterns, ",") != strings.Join(sgr.rulePatterns, ",") {
			modifiedGRs = append(modifiedGRs, modifiedGR{oldGR: agr, newGR: sgr})
		}
	}
	for _, agr := range activeGlobalRules {
		if _, exists := stagedGRMap[agr.ruleName]; !exists {
			removedGRs = append(removedGRs, agr)
		}
	}

	activePMap := make(map[string]tuf.Principal, len(activePrincipals))
	for _, p := range activePrincipals {
		activePMap[p.ID()] = p
	}
	stagedPMap := make(map[string]tuf.Principal, len(stagedPrincipals))
	for _, p := range stagedPrincipals {
		stagedPMap[p.ID()] = p
	}

	var addedPrincipals []tuf.Principal
	var removedPrincipals []tuf.Principal

	for _, sp := range stagedPrincipals {
		if _, exists := activePMap[sp.ID()]; !exists {
			addedPrincipals = append(addedPrincipals, sp)
		}
	}
	for _, ap := range activePrincipals {
		if _, exists := stagedPMap[ap.ID()]; !exists {
			removedPrincipals = append(removedPrincipals, ap)
		}
	}

	hasChanges := false

	// Policy Rules
	if len(addedRules) > 0 || len(removedRules) > 0 || len(modifiedRules) > 0 {
		hasChanges = true
		b.WriteString(sectionStyle.Render("Staged Rules:") + "\n")
		for _, r := range addedRules {
			b.WriteString(addStyle.Render(fmt.Sprintf("+ Rule: %s (pattern: %s, threshold: %d)", r.name, r.pattern, r.threshold)) + "\n")
			if r.key != "" {
				b.WriteString(subtextStyle.Render(fmt.Sprintf("  Authorized Principals: %s", r.key)) + "\n")
			}
		}
		for _, r := range removedRules {
			b.WriteString(delStyle.Render(fmt.Sprintf("- Rule: %s (pattern: %s, threshold: %d)", r.name, r.pattern, r.threshold)) + "\n")
			if r.key != "" {
				b.WriteString(subtextStyle.Render(fmt.Sprintf("  Authorized Principals: %s", r.key)) + "\n")
			}
		}
		for _, mr := range modifiedRules {
			b.WriteString(modStyle.Render(fmt.Sprintf("~ Rule: %s", mr.newRule.name)) + "\n")
			b.WriteString(delStyle.Render(fmt.Sprintf("  - pattern: %s, threshold: %d", mr.oldRule.pattern, mr.oldRule.threshold)) + "\n")
			b.WriteString(addStyle.Render(fmt.Sprintf("  + pattern: %s, threshold: %d", mr.newRule.pattern, mr.newRule.threshold)) + "\n")
			if mr.oldRule.key != mr.newRule.key {
				b.WriteString(delStyle.Render(fmt.Sprintf("  - Authorized Principals: %s", mr.oldRule.key)) + "\n")
				b.WriteString(addStyle.Render(fmt.Sprintf("  + Authorized Principals: %s", mr.newRule.key)) + "\n")
			}
		}
	}

	// Global Rules
	if len(addedGRs) > 0 || len(removedGRs) > 0 || len(modifiedGRs) > 0 {
		if hasChanges {
			b.WriteString("\n")
		}
		hasChanges = true
		b.WriteString(sectionStyle.Render("Staged Global Rules:") + "\n")
		for _, gr := range addedGRs {
			b.WriteString(addStyle.Render(fmt.Sprintf("+ Global Rule: %s (type: %s, patterns: %v)", gr.ruleName, gr.ruleType, gr.rulePatterns)) + "\n")
		}
		for _, gr := range removedGRs {
			b.WriteString(delStyle.Render(fmt.Sprintf("- Global Rule: %s (type: %s, patterns: %v)", gr.ruleName, gr.ruleType, gr.rulePatterns)) + "\n")
		}
		for _, mgr := range modifiedGRs {
			b.WriteString(modStyle.Render(fmt.Sprintf("~ Global Rule: %s", mgr.newGR.ruleName)) + "\n")
			b.WriteString(delStyle.Render(fmt.Sprintf("  - type: %s, patterns: %v", mgr.oldGR.ruleType, mgr.oldGR.rulePatterns)) + "\n")
			b.WriteString(addStyle.Render(fmt.Sprintf("  + type: %s, patterns: %v", mgr.newGR.ruleType, mgr.newGR.rulePatterns)) + "\n")
		}
	}

	// Principals
	if len(addedPrincipals) > 0 || len(removedPrincipals) > 0 {
		if hasChanges {
			b.WriteString("\n")
		}
		hasChanges = true
		b.WriteString(sectionStyle.Render("Staged Principals:") + "\n")
		for _, p := range addedPrincipals {
			b.WriteString(addStyle.Render(fmt.Sprintf("+ Principal: %s", p.ID())) + "\n")
		}
		for _, p := range removedPrincipals {
			b.WriteString(delStyle.Render(fmt.Sprintf("- Principal: %s", p.ID())) + "\n")
		}
	}

	if !hasChanges {
		filesChanged, _ := gitRepo.GetFilePathsChangedByCommit(stagingTip)
		if len(filesChanged) > 0 {
			b.WriteString(sectionStyle.Render("Staged Trust Metadata Changes:") + "\n")
			for _, f := range filesChanged {
				b.WriteString(addStyle.Render(fmt.Sprintf("+ Modified %s", f)) + "\n")
			}
		} else {
			b.WriteString("No staged policy changes detected.\n\n")
			b.WriteString(subtextStyle.Render("Policy staging has no rule, principal, or metadata differences."))
		}
	}

	return b.String()
}
