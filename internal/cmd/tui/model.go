// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
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
	screenMain screen = iota
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

// virtual methods must be implemented for the item struct

// Title returns the title of the item
func (i item) Title() string { return i.title }

// Description returns the description of the item
func (i item) Description() string { return i.desc }

// FilterValue returns the value to filter on
func (i item) FilterValue() string { return i.title }

type model struct {
	screen         screen
	mainList       list.Model
	rules          []rule
	ruleList       list.Model
	globalRules    []globalRule
	globalRuleList list.Model
	inputs         []textinput.Model
	focusIndex     int
	cursorMode     cursor.Mode
	repo           *gittuf.Repository
	signer         dsse.SignerVerifier
	policyName     string
	options        *options
	footer         string
	// readOnly indicates whether the TUI is running without a signing key
	// and should therefore hide mutating operations.
	readOnly bool
}

// initialModel returns the initial model for the Terminal UI
func initialModel(o *options) (model, error) {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return model{}, err
	}

	// Determine if we are in read-only mode. (no signing key provided, or read-only mode specified)
	var footer string
	if !o.readOnly && o.p.SigningKey == "" {
		footer = "No signing key provided, running in read-only mode."
	}
	readOnly := o.p.SigningKey == "" || o.readOnly

	var signer dsse.SignerVerifier
	if !readOnly {
		// Load signer only if a signing key was provided.
		signer, err = gittuf.LoadSigner(repo, o.p.SigningKey)
		if err != nil {
			// If a signing key was specified but cannot be loaded, return an empty model
			// to preserve existing error behavior.
			return model{}, err
		}
	}

	// Initialize the model
	m := model{
		screen:      screenMain,
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

	// Set up the main list items
	// In read-only mode, only non-mutating operations are available.
	var mainItems []list.Item
	if m.readOnly {
		mainItems = []list.Item{
			item{title: "List Rules", desc: "View all current policy rules"},
			item{title: "List Global Rules", desc: "View repository-wide global rules"},
		}
	} else {
		mainItems = []list.Item{
			item{title: "Add Rule", desc: "Add a new policy rule"},
			item{title: "Remove Rule", desc: "Remove an existing policy rule"},
			item{title: "List Rules", desc: "View all current policy rules"},
			item{title: "Reorder Rules", desc: "Change the order of policy rules"},
			item{title: "List Global Rules", desc: "View repository-wide global rules"},
			item{title: "Add Global Rule", desc: "Add a new global rule"},
			item{title: "Update Global Rule", desc: "Modify an existing global rule"},
			item{title: "Remove Global Rule", desc: "Remove a global rule"},
		}
	}

	// Set up the list delegate
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = selectedItemStyle
	delegate.Styles.SelectedDesc = selectedItemStyle
	delegate.Styles.NormalTitle = itemStyle
	delegate.Styles.NormalDesc = itemStyle

	// Set up the main list
	m.mainList = list.New(mainItems, delegate, 0, 0)
	m.mainList.Title = "gittuf Policy Operations"
	m.mainList.SetShowStatusBar(false)
	m.mainList.SetFilteringEnabled(false)
	m.mainList.Styles.Title = titleStyle
	m.mainList.SetShowHelp(false)

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
