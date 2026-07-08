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
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

type screen int

const (
	screenLoading             screen = iota // Loading screen shown on startup
	screenChoice                            // Initial menu
	screenPolicy                            // Menu for Policy operations
	screenPolicyRules                       // Rule management screen
	screenPolicyAddRule                     // Form: add a new policy rule
	screenPolicyEditRule                    // Form: edit selected rule (prefilled)
	screenTrust                             // Menu for Trust operations
	screenTrustGlobalRules                  // Global rule management screen
	screenTrustAddGlobalRule                // Form: add a new global rule
	screenTrustEditGlobalRule               // Form: edit selected global rule (prefilled)
	screenHelp                              // Generic Help screen displaying keybindings
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
	ctx               context.Context
	screen            screen
	spinner           spinner.Model
	homeScreen        homeScreen
	policyScreen      policyScreen
	trustScreenList   list.Model
	policyRulesScreen policyRulesScreen
	globalRules       []globalRule
	globalRuleList    list.Model
	inputs            []textinput.Model
	focusIndex        int
	cursorMode        cursor.Mode
	repo              *gittuf.Repository
	signer            dsse.SignerVerifier
	policyName        string
	options           *options
	footer            string
	errorMsg          string
	readOnly          bool
	width             int
	height            int
	confirmDelete     bool
	deleteTarget      string
	showHelp          bool
	signerError       string
	previousScreen    screen
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
func newDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()
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
		t.CharLimit = 64
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

	delegate := newDelegate()

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
			}, delegate),
		},
		trustScreenList: newMenuList("gittuf Trust Operations", []list.Item{
			item{title: "View Global Rules", desc: "View and manage global rules"},
		}, delegate),
		policyRulesScreen: policyRulesScreen{
			ruleList: newMenuList("Policy Rules", []list.Item{}, delegate),
		},
		globalRuleList: newMenuList("Global Rules", []list.Item{}, delegate),
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
	heightOffset := 9
	if m.readOnly {
		heightOffset = 11
		if m.signerError != "" {
			// Same formula as view.go: v(2) + fixed(7) + dynamic notice lines
			heightOffset = 9 + signerNoticeLines(m.signerError, m.width)
		}
	}
	innerHeight := m.height - heightOffset
	if innerHeight < 0 {
		innerHeight = 0
	}

	m.homeScreen.choiceList.SetSize(innerWidth, innerHeight)
	m.policyScreen.policyScreenList.SetSize(innerWidth, innerHeight)
	m.trustScreenList.SetSize(innerWidth, innerHeight)
	m.policyRulesScreen.ruleList.SetSize(innerWidth, innerHeight)
	m.globalRuleList.SetSize(innerWidth, innerHeight)
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

// initGlobalRuleInputs initializes the input fields for global rule forms.
func (m *model) initGlobalRuleInputs() {
	m.inputs = initInputs([]inputField{
		{"Enter Global Rule Name Here", "Rule Name:"},
		{"Enter Global Rule Type (threshold|block-force-pushes)", "Type:"},
		{"Enter Namespaces (comma-separated)", "Namespaces:"},
		{"Enter Threshold (if threshold type)", "Threshold:"},
	})
	m.focusIndex = 0
}

// initGlobalRuleInputsPrefilled initializes global rule inputs prefilled with an existing global rule's values.
func (m *model) initGlobalRuleInputsPrefilled(gr globalRule) {
	m.initGlobalRuleInputs()
	m.inputs[0].SetValue(gr.ruleName)
	m.inputs[1].SetValue(gr.ruleType)
	m.inputs[2].SetValue(strings.Join(gr.rulePatterns, ", "))
	if gr.ruleType == tuf.GlobalRuleThresholdType {
		m.inputs[3].SetValue(fmt.Sprintf("%d", gr.threshold))
	}
}

// refreshGlobalRules re-fetches global rules from the repo and rebuilds the list.
func (m *model) refreshGlobalRules() {
	m.globalRules = getGlobalRules(m.ctx, m.options)
	m.updateGlobalRuleList()
}

// updateGlobalRuleList updates the global rule list within the TUI.
func (m *model) updateGlobalRuleList() {
	items := make([]list.Item, len(m.globalRules))
	for i, gr := range m.globalRules {
		desc := fmt.Sprintf(
			"Type: %s\nNamespaces: %s",
			gr.ruleType,
			strings.Join(gr.rulePatterns, ", "),
		)
		if gr.ruleType == tuf.GlobalRuleThresholdType {
			desc += fmt.Sprintf("\nThreshold: %d", gr.threshold)
		}
		items[i] = item{title: gr.ruleName, desc: desc}
	}
	m.globalRuleList.SetItems(items)
}
