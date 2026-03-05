// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

type screen int

const (
	screenChoice              screen = iota // Choose Policy or Trust
	screenPolicy                            // Intermediate menu for Policy operations
	screenPolicyRules                       // View all policy rules - primary interface
	screenPolicyAddRule                     // Form: add a new policy rule
	screenPolicyEditRule                    // Form: edit selected rule (prefilled)
	screenTrust                             // Intermediate menu for Trust operations
	screenTrustGlobalRules                  // View all global rules - primary interface
	screenTrustAddGlobalRule                // Form: add a new global rule
	screenTrustEditGlobalRule               // Form: edit selected global rule (prefilled)
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
	screen           screen
	choiceList       list.Model
	policyScreenList list.Model
	trustScreenList  list.Model
	rules            []rule
	ruleList         list.Model
	globalRules      []globalRule
	globalRuleList   list.Model
	inputs           []textinput.Model
	focusIndex       int
	cursorMode       cursor.Mode
	repo             *gittuf.Repository
	signer           dsse.SignerVerifier
	policyName       string
	options          *options
	footer           string
	readOnly         bool
	confirmDelete    bool
	deleteTarget     string
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

// initialModel returns the initial model for the Terminal UI.
func initialModel(o *options) (model, error) {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return model{}, err
	}

	// Determine if we are in read-only mode. (read-only mode specified, or no signing key found)
	readOnly := o.readOnly
	var signer dsse.SignerVerifier
	var footer string

	if !readOnly {
		signer, err = gittuf.LoadSigner(repo, o.p.SigningKey)
		if err != nil {
			if !errors.Is(err, gittuf.ErrSigningKeyNotSpecified) {
				return model{}, fmt.Errorf("failed to load signing key from Git config: %w", err)
			}
			readOnly = true
			footer = "No signing key found in Git config, running in read-only mode."
		}
	}

	delegate := newDelegate()

	m := model{
		screen:      screenChoice,
		cursorMode:  cursor.CursorBlink,
		repo:        repo,
		signer:      signer,
		policyName:  o.policyName,
		rules:       getCurrRules(o),
		globalRules: getGlobalRules(o),
		options:     o,
		readOnly:    readOnly,
		footer:      footer,

		choiceList: newMenuList("gittuf TUI", []list.Item{
			item{title: "Policy", desc: "View and manage gittuf Policy"},
			item{title: "Trust", desc: "View and manage gittuf Root of Trust"},
		}, delegate),
		policyScreenList: newMenuList("gittuf Policy Operations", []list.Item{
			item{title: "View Rules", desc: "View and manage policy rules"},
		}, delegate),
		trustScreenList: newMenuList("gittuf Trust Operations", []list.Item{
			item{title: "View Global Rules", desc: "View and manage global rules"},
		}, delegate),
		ruleList:       newMenuList("Policy Rules", []list.Item{}, delegate),
		globalRuleList: newMenuList("Global Rules", []list.Item{}, delegate),

		inputs: initInputs([]inputField{
			{"Enter Rule Name Here", "Rule Name:"},
			{"Enter Pattern Here", "Pattern:"},
			{"Enter Principal IDs Here (comma-separated)", "Authorize Principal:"},
		}),
	}

	return m, nil
}

// Init initializes the input field.
func (m model) Init() tea.Cmd {
	return textinput.Blink
}

// initRuleInputs reinitializes the input fields for (policy) rule forms.
func (m *model) initRuleInputs() {
	m.inputs = initInputs([]inputField{
		{"Enter Rule Name Here", "Rule Name:"},
		{"Enter Pattern Here", "Pattern:"},
		{"Enter Principal IDs Here (comma-separated)", "Authorize Principal:"},
	})
	m.focusIndex = 0
}

// initRuleInputsPrefilled initializes rule inputs prefilled with an existing rule's values.
func (m *model) initRuleInputsPrefilled(r rule) {
	m.initRuleInputs()
	m.inputs[0].SetValue(r.name)
	m.inputs[1].SetValue(r.pattern)
	m.inputs[2].SetValue(r.key)
}

// initGlobalRuleInputs reinitializes the input fields for global rule forms.
func (m *model) initGlobalRuleInputs() {
	m.inputs = initInputs([]inputField{
		{"Enter Global Rule Name Here", "Rule Name:"},
		{"Enter Rule Type (threshold|block-force-pushes)", "Type:"},
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

// refreshRules re-fetches rules from the repo and rebuilds the list.
func (m *model) refreshRules() {
	m.rules = getCurrRules(m.options)
	m.updateRuleList()
}

// refreshGlobalRules re-fetches global rules from the repo and rebuilds the list.
func (m *model) refreshGlobalRules() {
	m.globalRules = getGlobalRules(m.options)
	m.updateGlobalRuleList()
}

// updateRuleList updates the rule list within TUI.
func (m *model) updateRuleList() {
	items := make([]list.Item, len(m.rules))
	for i, rule := range m.rules {
		items[i] = item{title: rule.name, desc: fmt.Sprintf("Pattern: %s, Key: %s", rule.pattern, rule.key)}
	}
	m.ruleList.SetItems(items)
}

// updateGlobalRuleList updates the global rule list within TUI.
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
