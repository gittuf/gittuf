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
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

type screen int

const (
	screenLoading              screen = iota // Loading screen shown on startup
	screenChoice                             // Initial menu
	screenPolicy                             // Menu for Policy operations
	screenPolicyRules                        // Rule management screen
	screenPolicyAddRule                      // Form: add a new policy rule
	screenPolicyEditRule                     // Form: edit selected rule (prefilled)
	screenPolicyPrincipals                   // Principals management screen
	screenPolicyPrincipalsForm               // Form: Add/Edit principal or add key
	screenPolicyLifecycle                    // Menu for Policy lifecycle operations
	screenPolicyLifecycleForm                // Form: policy lifecycle operation options
	screenTrust                              // Menu for Trust operations
	screenTrustGlobalRules                   // Global rule management screen
	screenTrustAddGlobalRule                 // Form: add a new global rule
	screenTrustEditGlobalRule                // Form: edit selected global rule (prefilled)
	screenHelp                               // Generic Help screen displaying keybindings
)

type item struct {
	title, desc string
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
	homeScreen                 homeScreen
	helpScreen                 helpScreen
	policyScreen               policyScreen
	policyLifecycleScreen      policyLifecycleScreen
	trustScreen                trustScreen
	policyRulesScreen          policyRulesScreen
	trustGlobalRulesScreen     trustGlobalRulesScreen
	policyPrincipalsScreen     policyPrincipalsScreen
	policyPrincipalsFormScreen policyPrincipalsFormScreen
	cursorMode                 cursor.Mode
	repo                       *gittuf.Repository
	signer                     dsse.SignerVerifier
	policyName                 string
	options                    *options
	footer                     string
	errorMsg                   string
	readOnly                   bool
	width                      int
	height                     int
	showHelp                   bool
	signerError                string
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

// initialModel returns a lightweight loading model for the Terminal UI.
// All heavy work (repo I/O, signing key, rules) is deferred to loadRepoCmd.
func initialModel(ctx context.Context, o *options) model {
	s := spinner.New()
	s.Spinner = spinner.Dot

	delegate := newDelegate(2)
	delegateMultiline := newDelegate(4)

	m := model{
		ctx:        ctx,
		screen:     screenLoading,
		spinner:    s,
		cursorMode: cursor.CursorBlink,
		policyName: o.policyName,
		options:    o,

		homeScreen: homeScreen{
			choiceList: newMenuList("gittuf TUI", []list.Item{
				item{title: "Policy", desc: "View and manage gittuf Policy"},
				item{title: "Trust", desc: "View and manage gittuf Root of Trust"},
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
			trustScreenList: newMenuList("gittuf Trust Operations", []list.Item{
				item{title: "View Global Rules", desc: "View and manage global rules"},
			}, delegate),
		},
		policyRulesScreen: policyRulesScreen{
			ruleList: newMenuList("Policy Rules", []list.Item{}, delegate),
		},
		trustGlobalRulesScreen: trustGlobalRulesScreen{
			globalRuleList: newMenuList("Global Rules", []list.Item{}, delegate),
		},
		policyPrincipalsScreen: policyPrincipalsScreen{
			list: newMenuList("Policy Principals", []list.Item{}, delegateMultiline),
		},
	}

	return m
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
