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
	screenChoice screen = iota // initial screen
	screenPolicy
	screenTrust
	screenAddRule
	screenRemoveRule
	screenListRules
	screenReorderRules
	screenListGlobalRules
	screenAddGlobalRule
	screenUpdateGlobalRule
	screenRemoveGlobalRule
)

type item struct {
	title, desc string
}

// Note: virtual methods must be implemented for the item struct
// Title returns the title of the item
func (i item) Title() string { return i.title }

// Description returns the description of the item
func (i item) Description() string { return i.desc }

// FilterValue returns the value to filter on
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
}

// initialModel returns the initial model for the Terminal UI
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
		// Try to load signer. Uses Git config if signing key not explicitly provided
		signer, err = gittuf.LoadSigner(repo, o.p.SigningKey)
		if err != nil {
			if !errors.Is(err, gittuf.ErrSigningKeyNotSpecified) {
				// If a signing key was found but cannot be loaded, return error
				return model{}, fmt.Errorf("failed to load signing key from Git config: %w", err)
			}
			readOnly = true
			footer = "No signing key found in Git config, running in read-only mode."
		}
	}

	// Initialize the model
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
	}

	// Set up choice screen list items
	choiceItems := []list.Item{
		item{title: "Policy", desc: "Manage gittuf Policy"},
		item{title: "Trust", desc: "Manage gittuf Root of Trust"},
	}

	// Set up the policy screen list items
	// In read-only mode, only non-mutating operations are available.
	var policyItems []list.Item
	if m.readOnly {
		policyItems = []list.Item{
			item{title: "List Rules", desc: "View all current policy rules"},
		}
	} else {
		policyItems = []list.Item{
			item{title: "Add Rule", desc: "Add a new policy rule"},
			item{title: "Remove Rule", desc: "Remove an existing policy rule"},
			item{title: "List Rules", desc: "View all current policy rules"},
			item{title: "Reorder Rules", desc: "Change the order of policy rules"},
		}
	}

	// Set up trust screen list items
	var trustItems []list.Item
	if m.readOnly {
		trustItems = []list.Item{
			item{title: "List Global Rules", desc: "View repository-wide global rules"},
		}
	} else {
		trustItems = []list.Item{
			item{title: "Add Global Rule", desc: "Add a new global rule"},
			item{title: "Remove Global Rule", desc: "Remove a global rule"},
			item{title: "Update Global Rule", desc: "Modify an existing global rule"},
			item{title: "List Global Rules", desc: "View repository-wide global rules"},
		}
	}

	// Set up the list delegate
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = selectedItemStyle
	delegate.Styles.SelectedDesc = selectedItemStyle
	delegate.Styles.NormalTitle = itemStyle
	delegate.Styles.NormalDesc = itemStyle

	// Set up choice screen list
	m.choiceList = list.New(choiceItems, delegate, 0, 0)
	m.choiceList.Title = "gittuf TUI"
	m.choiceList.SetShowStatusBar(false)
	m.choiceList.SetFilteringEnabled(false)
	m.choiceList.Styles.Title = titleStyle
	m.choiceList.SetShowHelp(false)

	// Set up the policy screen list
	m.policyScreenList = list.New(policyItems, delegate, 0, 0)
	m.policyScreenList.Title = "gittuf Policy Operations"
	m.policyScreenList.SetShowStatusBar(false)
	m.policyScreenList.SetFilteringEnabled(false)
	m.policyScreenList.Styles.Title = titleStyle
	m.policyScreenList.SetShowHelp(false)

	// Set up the trust screen list
	m.trustScreenList = list.New(trustItems, delegate, 0, 0)
	m.trustScreenList.Title = "gittuf Trust Operations"
	m.trustScreenList.SetShowStatusBar(false)
	m.trustScreenList.SetFilteringEnabled(false)
	m.trustScreenList.Styles.Title = titleStyle
	m.trustScreenList.SetShowHelp(false)

	// Set up the rule list
	m.ruleList = list.New([]list.Item{}, delegate, 0, 0)
	m.ruleList.Title = "Current Rules"
	m.ruleList.SetShowStatusBar(false)
	m.ruleList.SetFilteringEnabled(false)
	m.ruleList.Styles.Title = titleStyle
	m.ruleList.SetShowHelp(false)

	// set up global rule list
	m.globalRuleList = list.New([]list.Item{}, delegate, 0, 0)
	m.globalRuleList.Title = "Global Rules"
	m.globalRuleList.SetShowStatusBar(false)
	m.globalRuleList.SetFilteringEnabled(false)
	m.globalRuleList.Styles.Title = titleStyle
	m.globalRuleList.SetShowHelp(false)

	// Set up the input fields
	m.inputs = make([]textinput.Model, 3)
	for i := range m.inputs {
		t := textinput.New()
		t.Cursor.Style = cursorStyle
		t.CharLimit = 64

		switch i {
		case 0:
			t.Placeholder = "Enter Rule Name Here"
			t.Focus()
			t.PromptStyle = focusedStyle
			t.Prompt = "Rule Name:"
			t.TextStyle = focusedStyle
		case 1:
			t.Placeholder = "Enter Pattern Here"
			t.Prompt = "Pattern:"
			t.PromptStyle = blurredStyle
			t.TextStyle = blurredStyle
		case 2:
			t.Placeholder = "Enter Path to Key Here"
			t.Prompt = "Authorize Key:"
			t.PromptStyle = blurredStyle
			t.TextStyle = blurredStyle
		}

		m.inputs[i] = t
	}

	return m, nil
}

// Init initializes the input field
func (m model) Init() tea.Cmd {
	return textinput.Blink
}

// reinitialize inputs for global rules
func (m *model) initGlobalInputs() {
	prompts := []struct{ placeholder, prompt string }{
		{"Enter Global Rule Name Here", "Rule Name:"},
		{"Enter Rule Type (threshold|block-force-pushes)", "Type:"},
		{"Enter Namespaces (comma-separated)", "Namespaces:"},
		{"Enter Threshold (if threshold type)", "Threshold:"},
	}
	m.inputs = make([]textinput.Model, len(prompts))
	for i, p := range prompts {
		t := textinput.New()
		t.Cursor.Style = cursorStyle
		t.CharLimit = 64
		t.Placeholder = p.placeholder
		t.Prompt = p.prompt
		if i == 0 {
			t.Focus()
			t.PromptStyle = focusedStyle
			t.TextStyle = focusedStyle
		} else {
			t.Blur()
			t.PromptStyle = blurredStyle
			t.TextStyle = blurredStyle
		}
		m.inputs[i] = t
	}
}

// updateRuleList updates the rule list within TUI
func (m *model) updateRuleList() {
	items := make([]list.Item, len(m.rules))
	for i, rule := range m.rules {
		items[i] = item{title: rule.name, desc: fmt.Sprintf("Pattern: %s, Key: %s", rule.pattern, rule.key)}
	}
	m.ruleList.SetItems(items)
}

// updateGlobalRuleList updates the global rule list within TUI
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
