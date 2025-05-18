// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gittuf/gittuf/experimental/gittuf"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/spf13/cobra"
)

const (
	colorRegularText = "#FFFFFF"
	colorFocus       = "#007AFF"
	colorBlur        = "#A0A0A0"
	colorFooter      = "#FF0000"
	colorSubtext     = "#555555"
)

var (
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorRegularText)).
			Padding(0, 2).
			MarginTop(1).
			Bold(true)

	itemStyle = lipgloss.NewStyle().
			PaddingLeft(4).
			Foreground(lipgloss.Color(colorRegularText))

	selectedItemStyle = lipgloss.NewStyle().
				PaddingLeft(4).
				Foreground(lipgloss.Color(colorRegularText)).
				Background(lipgloss.Color(colorFocus))

	focusedStyle = lipgloss.NewStyle().
			PaddingLeft(4)

	blurredStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorBlur))

	cursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorRegularText))
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
	screenViewRootMetadata
	screenViewTargetsMetadata
)

type rule struct {
	name    string
	pattern string
	key     string
}

type globalRule struct {
	ruleName     string
	ruleType     string
	rulePatterns []string
	threshold    int
}

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
}

// initialModel returns the initial model for the Terminal UI
func initialModel(o *options) model {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return model{}
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return model{}
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
	}

	// Set up the main list items
	mainItems := []list.Item{
		item{title: "Add Rule", desc: "Add a new policy rule"},
		item{title: "Remove Rule", desc: "Remove an existing policy rule"},
		item{title: "List Rules", desc: "View all current policy rules"},
		item{title: "Reorder Rules", desc: "Change the order of policy rules"},
		item{title: "List Global Rules", desc: "View repository-wide global rules"},
		item{title: "Add Global Rule", desc: "Add a new global rule"},
		item{title: "Update Global Rule", desc: "Modify an existing global rule"},
		item{title: "Remove Global Rule", desc: "Remove a global rule"},
		item{title: "View Root Metadata", desc: "View Root metadata information"},
		item{title: "View Targets Metadata", desc: "View Targets metadata information"},
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

	return m
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

// Init initializes the input field
func (m model) Init() tea.Cmd {
	return textinput.Blink
}

// Update updates the model based on the message received
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := lipgloss.NewStyle().Margin(1, 2).GetFrameSize()
		m.mainList.SetSize(msg.Width-h, msg.Height-v)
		m.ruleList.SetSize(msg.Width-h, msg.Height-v)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "left":
			if m.screen != screenMain {
				m.footer = "" // Clear footer on navigation
				m.screen = screenMain
				return m, nil
			}
		case "enter":
			switch m.screen {
			case screenMain:
				i, ok := m.mainList.SelectedItem().(item)
				if ok {
					switch i.title {
					case "Add Rule":
						m.screen = screenAddRule
						m.focusIndex = 0
						m.inputs[0].Focus()
					case "Remove Rule":
						m.screen = screenRemoveRule
						m.updateRuleList()
					case "List Rules":
						m.screen = screenListRules
					case "Reorder Rules":
						m.screen = screenReorderRules
						m.updateRuleList()
					case "List Global Rules":
						m.screen = screenListGlobalRules
						m.updateGlobalRuleList()

					case "Add Global Rule":
						m.screen = screenAddGlobalRule
						m.initGlobalInputs()

					case "Update Global Rule":
						m.screen = screenUpdateGlobalRule
						m.initGlobalInputs()

					case "Remove Global Rule":
						m.screen = screenRemoveGlobalRule
						m.updateGlobalRuleList()

					case "View Root Metadata":
						m.screen = screenViewRootMetadata

					case "View Targets Metadata":
						m.screen = screenViewTargetsMetadata
					}
				}

			case screenAddRule:
				if m.focusIndex == len(m.inputs)-1 {
					newRule := rule{
						name:    m.inputs[0].Value(),
						pattern: m.inputs[1].Value(),
						key:     m.inputs[2].Value(),
					}
					authorizedKeys := []string{m.inputs[2].Value()}
					err := repoAddRule(m.options, newRule, authorizedKeys)
					if err != nil {
						m.footer = fmt.Sprintf("Error adding rule: %v", err)
						return m, nil
					}
					m.rules = append(m.rules, newRule)
					m.updateRuleList()
					m.footer = "Rule added successfully!"
					m.screen = screenMain
				}
			case screenRemoveRule:
				if i, ok := m.ruleList.SelectedItem().(item); ok {
					err := repoRemoveRule(m.options, rule{name: i.title})
					if err != nil {
						m.footer = fmt.Sprintf("Error removing rule: %v", err)
						return m, nil
					}
					for idx, rule := range m.rules {
						if rule.name == i.title {
							m.rules = append(m.rules[:idx], m.rules[idx+1:]...)
							break
						}
					}
					m.updateRuleList()
					m.footer = "Rule removed successfully!"
					m.screen = screenMain
				}
			case screenAddGlobalRule:
				// parse comma-separated input into []string
				if m.focusIndex == len(m.inputs)-1 {
					raw := m.inputs[2].Value()
					parts := strings.Split(raw, ",")
					for i := range parts {
						parts[i] = strings.TrimSpace(parts[i])
					}
					// parse threshold only if that type
					thr := 0
					if m.inputs[1].Value() == tuf.GlobalRuleThresholdType {
						thr, _ = strconv.Atoi(m.inputs[3].Value())
					}
					newGR := globalRule{
						ruleName:     m.inputs[0].Value(),
						ruleType:     m.inputs[1].Value(),
						rulePatterns: parts,
						threshold:    thr,
					}
					if err := repoAddGlobalRule(m.options, newGR); err != nil {
						m.footer = fmt.Sprintf("Error: %v", err)
						return m, nil
					}
					m.globalRules = append(m.globalRules, newGR)
					m.updateGlobalRuleList()
					m.footer = "Global rule added!"
					m.screen = screenMain
				}
			case screenRemoveGlobalRule:
				if sel, ok := m.globalRuleList.SelectedItem().(item); ok {
					err := repoRemoveGlobalRule(m.options, globalRule{ruleName: sel.title})
					if err != nil {
						m.footer = fmt.Sprintf("Error removing global rule: %v", err)
						return m, nil
					}
					for idx, gr := range m.globalRules {
						if gr.ruleName == sel.title {
							m.globalRules = append(m.globalRules[:idx], m.globalRules[idx+1:]...)
							break
						}
					}
					m.updateGlobalRuleList()
					m.footer = "Global rule removed!"
					m.screen = screenMain
				}
			case screenUpdateGlobalRule:
				if m.focusIndex == len(m.inputs)-1 {
					// parse namespaces (split + TrimSpace)
					parts := strings.Split(m.inputs[2].Value(), ",")
					for i := range parts {
						parts[i] = strings.TrimSpace(parts[i])
					}
					// parse threshold if needed
					thr := 0
					if m.inputs[1].Value() == tuf.GlobalRuleThresholdType {
						thr, _ = strconv.Atoi(m.inputs[3].Value())
					}
					updated := globalRule{
						ruleName:     m.inputs[0].Value(),
						ruleType:     m.inputs[1].Value(),
						rulePatterns: parts,
						threshold:    thr,
					}
					if err := repoUpdateGlobalRule(m.options, updated); err != nil {
						m.footer = fmt.Sprintf("Error updating global rule: %v", err)
						return m, nil
					}
					for idx, gr := range m.globalRules {
						if gr.ruleName == updated.ruleName {
							m.globalRules[idx] = updated
							break
						}
					}
					m.updateGlobalRuleList()
					m.footer = "Global rule updated!"
					m.screen = screenMain
				}
			}
		case "u":
			if m.screen == screenReorderRules {
				if i := m.ruleList.Index(); i > 0 {
					m.rules[i], m.rules[i-1] = m.rules[i-1], m.rules[i]
					if err := repoReorderRules(m.options, m.rules); err != nil {
						m.footer = fmt.Sprintf("Error reordering rules: %v", err)
						return m, nil
					}
					m.updateRuleList()
					m.footer = "Rules reordered successfully!"
				}
			}
		case "d":
			if m.screen == screenReorderRules {
				if i := m.ruleList.Index(); i < len(m.rules)-1 {
					m.rules[i], m.rules[i+1] = m.rules[i+1], m.rules[i]
					if err := repoReorderRules(m.options, m.rules); err != nil {
						m.footer = fmt.Sprintf("Error reordering rules: %v", err)
						return m, nil
					}
					m.updateRuleList()
					m.footer = "Rules reordered successfully!"
				}
			}
		case "tab", "shift+tab", "up", "down":
			if m.screen == screenAddRule || m.screen == screenAddGlobalRule || m.screen == screenUpdateGlobalRule {
				s := msg.String()
				if s == "up" || s == "shift+tab" {
					if m.focusIndex > 0 {
						m.focusIndex--
						m.footer = ""
					} else {
						m.focusIndex = len(m.inputs) - 1
					}
				} else {
					if m.focusIndex < len(m.inputs)-1 {
						m.focusIndex++
					} else {
						m.focusIndex = 0
					}
				}

				for i := 0; i <= len(m.inputs)-1; i++ {
					if i == m.focusIndex {
						m.inputs[i].Focus()
						m.inputs[i].PromptStyle = focusedStyle
						m.inputs[i].TextStyle = focusedStyle
						continue
					}
					m.inputs[i].Blur()
					m.inputs[i].PromptStyle = blurredStyle
					m.inputs[i].TextStyle = blurredStyle
				}
				return m, nil
			}
		}
	}

	switch m.screen {
	case screenMain:
		m.mainList, cmd = m.mainList.Update(msg)
	case screenAddRule:
		m.inputs[m.focusIndex], cmd = m.inputs[m.focusIndex].Update(msg)
	case screenRemoveRule, screenReorderRules:
		m.ruleList, cmd = m.ruleList.Update(msg)
	case screenAddGlobalRule, screenUpdateGlobalRule:
		m.inputs[m.focusIndex], cmd = m.inputs[m.focusIndex].Update(msg)
	case screenListGlobalRules, screenRemoveGlobalRule:
		m.globalRuleList, cmd = m.globalRuleList.Update(msg)
	}

	return m, cmd
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

// View renders the TUI
func (m model) View() string {
	switch m.screen {
	case screenMain:
		return lipgloss.NewStyle().Margin(1, 2).Render(
			m.mainList.View() + "\n" +
				lipgloss.NewStyle().Foreground(lipgloss.Color(colorFooter)).Render(m.footer) +
				"\nRun `gittuf policy apply` to apply staged changes to the selected policy file",
		)
	case screenAddRule:
		var b strings.Builder
		b.WriteString(titleStyle.Render("Add Rule") + "\n\n")
		for _, input := range m.inputs {
			b.WriteString(input.View() + "\n")
		}
		b.WriteString("\nPress Enter to add, Left Arrow to go back\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(colorFooter)).Render(m.footer))
		return lipgloss.NewStyle().Margin(1, 2).Render(b.String())
	case screenRemoveRule:
		return lipgloss.NewStyle().Margin(1, 2).Render(
			m.ruleList.View() + "\n\n" +
				lipgloss.NewStyle().Foreground(lipgloss.Color(colorFooter)).Render(m.footer) +
				"\nPress Enter to remove selected rule, Left Arrow to go back",
		)
	case screenListRules:
		var sb strings.Builder
		sb.WriteString(titleStyle.Render("Current Rules") + "\n\n")
		for _, rule := range m.rules {
			sb.WriteString(fmt.Sprintf("- %s\n  Pattern: %s\n  Key: %s\n\n",
				lipgloss.NewStyle().Foreground(lipgloss.Color(colorRegularText)).Bold(true).Render(rule.name),
				lipgloss.NewStyle().Foreground(lipgloss.Color(colorSubtext)).Render(rule.pattern),
				lipgloss.NewStyle().Foreground(lipgloss.Color(colorSubtext)).Render(rule.key)))
		}
		sb.WriteString("\nPress Left Arrow to go back")
		return lipgloss.NewStyle().Margin(1, 2).Render(sb.String())
	case screenReorderRules:
		return lipgloss.NewStyle().Margin(1, 2).Render(
			m.ruleList.View() + "\n\n" +
				lipgloss.NewStyle().Foreground(lipgloss.Color(colorFooter)).Render(m.footer) +
				"\nUse 'u' to move up, 'd' to move down, Left Arrow to go back",
		)
	case screenAddGlobalRule:
		var b strings.Builder
		b.WriteString(titleStyle.Render("Add Global Rule") + "\n\n")
		for _, input := range m.inputs {
			b.WriteString(input.View() + "\n")
		}
		b.WriteString("\nPress Enter to add, Left Arrow to go back\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(colorFooter)).Render(m.footer))
		return lipgloss.NewStyle().Margin(1, 2).Render(b.String())

	case screenListGlobalRules:
		return lipgloss.NewStyle().Margin(1, 2).Render(
			m.globalRuleList.View() + "\n\n" +
				lipgloss.NewStyle().Foreground(lipgloss.Color(colorFooter)).Render(m.footer) +
				"\nPress Left Arrow to go back",
		)

	case screenUpdateGlobalRule:
		var b strings.Builder
		b.WriteString(titleStyle.Render("Update Global Rule") + "\n\n")
		for _, input := range m.inputs {
			b.WriteString(input.View() + "\n")
		}
		b.WriteString("\nPress Enter to update, Left Arrow to go back\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(colorFooter)).Render(m.footer))
		return lipgloss.NewStyle().Margin(1, 2).Render(b.String())

	case screenRemoveGlobalRule:
		return lipgloss.NewStyle().Margin(1, 2).Render(
			m.globalRuleList.View() + "\n\n" +
				lipgloss.NewStyle().Foreground(lipgloss.Color(colorFooter)).Render(m.footer) +
				"\nPress Enter to remove selected global rule, Left Arrow to go back",
		)

	case screenViewRootMetadata:
		var sb strings.Builder
		sb.WriteString(titleStyle.Render("Root Metadata") + "\n\n")

		state, err := policy.LoadCurrentState(context.Background(), m.repo.GetGitRepository(), m.options.targetRef)
		if err != nil {
			m.footer = fmt.Sprintf("Error loading state: %v", err)
			sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(colorFooter)).Render(m.footer))
			return lipgloss.NewStyle().Margin(1, 2).Render(sb.String())
		}

		rootMetadata, err := state.GetRootMetadata(false)
		if err != nil {
			m.footer = fmt.Sprintf("Error loading root metadata: %v", err)
			sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(colorFooter)).Render(m.footer))
			return lipgloss.NewStyle().Margin(1, 2).Render(sb.String())
		}

		// Display schema version
		sb.WriteString(fmt.Sprintf("Schema Version: %s\n\n", rootMetadata.SchemaVersion()))

		// Display principals
		principals, err := rootMetadata.GetRootPrincipals()
		if err != nil {
			m.footer = fmt.Sprintf("Error loading principals: %v", err)
			sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(colorFooter)).Render(m.footer))
			return lipgloss.NewStyle().Margin(1, 2).Render(sb.String())
		}

		sb.WriteString("Root Principals:\n")
		for _, principal := range principals {
			sb.WriteString(fmt.Sprintf("\nPrincipal %s:\n", principal.ID()))
			if keys := principal.Keys(); len(keys) > 0 {
				sb.WriteString("    Keys:\n")
				for _, key := range keys {
					sb.WriteString(fmt.Sprintf("        %s (%s)\n", key.KeyID, key.KeyType))
				}
			}
			// Check if principal has custom metadata (richer object)
			if metadata := principal.CustomMetadata(); len(metadata) > 0 {
				sb.WriteString("    Custom Metadata:\n")
				for key, value := range metadata {
					sb.WriteString(fmt.Sprintf("        %s: %s\n", key, value))
				}
			}
		}

		sb.WriteString("\nPress Left Arrow to go back")
		return lipgloss.NewStyle().Margin(1, 2).Render(sb.String())

	case screenViewTargetsMetadata:
		var sb strings.Builder
		sb.WriteString(titleStyle.Render("Targets Metadata") + "\n\n")

		state, err := policy.LoadCurrentState(context.Background(), m.repo.GetGitRepository(), m.options.targetRef)
		if err != nil {
			m.footer = fmt.Sprintf("Error loading state: %v", err)
			sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(colorFooter)).Render(m.footer))
			return lipgloss.NewStyle().Margin(1, 2).Render(sb.String())
		}

		targetsMetadata, err := state.GetTargetsMetadata(m.policyName, false)
		if err != nil {
			m.footer = fmt.Sprintf("Error loading targets metadata: %v", err)
			sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(colorFooter)).Render(m.footer))
			return lipgloss.NewStyle().Margin(1, 2).Render(sb.String())
		}

		// Display schema version
		sb.WriteString(fmt.Sprintf("Schema Version: %s\n\n", targetsMetadata.SchemaVersion()))

		// Display policy principals
		principals := targetsMetadata.GetPrincipals()
		if len(principals) > 0 {
			sb.WriteString("Policy Principals:\n")
			for id, principal := range principals {
				sb.WriteString(fmt.Sprintf("\nPrincipal %s:\n", id))
				if metadata := principal.CustomMetadata(); len(metadata) > 0 {
					sb.WriteString("    Custom Metadata:\n")
					for key, value := range metadata {
						sb.WriteString(fmt.Sprintf("        %s: %s\n", key, value))
					}
				}
			}
		} else {
			sb.WriteString("No principals defined in the policy.\n")
		}

		sb.WriteString("\nPress Left Arrow to go back")
		return lipgloss.NewStyle().Margin(1, 2).Render(sb.String())

	default:
		return "Unknown screen"
	}
}

// getCurrRules returns the current rules from the policy file
func getCurrRules(o *options) []rule {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return nil
	}

	rules, err := repo.ListRules(context.Background(), o.targetRef)
	if err != nil {
		return nil
	}

	var currRules = make([]rule, len(rules))
	for i, r := range rules {
		currRules[i] = rule{
			name:    r.Delegation.ID(),
			pattern: strings.Join(r.Delegation.GetProtectedNamespaces(), ", "),
			key:     strings.Join(r.Delegation.GetPrincipalIDs().Contents(), ", "),
		}
	}
	return currRules
}

// repoAddRule adds a rule to the policy file
func repoAddRule(o *options, rule rule, keyPath []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	authorizedPrincipalIDs := []string{}
	for _, key := range keyPath {
		key, err := gittuf.LoadPublicKey(key)
		if err != nil {
			return err
		}

		authorizedPrincipalIDs = append(authorizedPrincipalIDs, key.ID())
	}
	res := repo.AddDelegation(context.Background(), signer, o.policyName, rule.name, authorizedPrincipalIDs, []string{rule.pattern}, 1, true)

	return res
}

// repoRemoveRule removes a rule from the policy file
func repoRemoveRule(o *options, rule rule) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}
	return repo.RemoveDelegation(context.Background(), signer, o.policyName, rule.name, true)
}

// repoReorderRules reorders the rules in the policy file
func repoReorderRules(o *options, rules []rule) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	ruleNames := make([]string, len(rules))
	for i, r := range rules {
		ruleNames[i] = r.
			name
	}

	return repo.ReorderDelegations(context.Background(), signer, o.policyName, ruleNames, true)
}

// startTUI starts the TUI
func startTUI(o *options) error {
	p := tea.NewProgram(initialModel(o), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

type options struct {
	p          *persistent.Options
	policyName string
	targetRef  string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.targetRef,
		"target-ref",
		"policy",
		"specify which policy ref should be inspected",
	)

	cmd.Flags().StringVar(
		&o.policyName,
		"policy-name",
		policy.TargetsRoleName,
		"name of policy file to make changes to",
	)
}

func (o *options) Run(_ *cobra.Command, _ []string) error {
	return startTUI(o)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "tui",
		Short:             "Start the TUI for managing policies",
		Long:              "This command allows users to start a terminal-based interface to manage policies. The signing key specified will be used to sign all operations while in the TUI. Changes to the policy files in the TUI are staged immediately without further confirmation and users are required to run `gittuf policy apply` to commit the changes",
		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}

// getGlobalRules returns a slice of globalRule for the TUI
func getGlobalRules(o *options) []globalRule {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return nil
	}

	rules, err := repo.ListGlobalRules(context.Background(), o.targetRef)
	if err != nil {
		return nil
	}

	var currRules = make([]globalRule, len(rules))
	for i, r := range rules {
		switch gRule := r.(type) {
		case tuf.GlobalRuleThreshold:
			currRules[i] = globalRule{
				ruleName:     gRule.GetName(),
				ruleType:     tuf.GlobalRuleThresholdType,
				rulePatterns: gRule.GetProtectedNamespaces(),
				threshold:    gRule.GetThreshold(),
			}
		case tuf.GlobalRuleBlockForcePushes:
			currRules[i] = globalRule{
				ruleName:     gRule.GetName(),
				ruleType:     tuf.GlobalRuleBlockForcePushesType,
				rulePatterns: gRule.GetProtectedNamespaces(),
			}
		}
	}
	return currRules
}

// repoAddGlobalRule adds a global rule
func repoAddGlobalRule(o *options, gr globalRule) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}
	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}
	var opts []trustpolicyopts.Option
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}
	switch gr.ruleType {
	case tuf.GlobalRuleThresholdType:
		if len(gr.rulePatterns) == 0 {
			return fmt.Errorf("required flag --rule-pattern not set for global rule type '%s'", tuf.GlobalRuleThresholdType)
		}
		return repo.AddGlobalRuleThreshold(
			context.Background(), signer,
			gr.ruleName, gr.rulePatterns,
			gr.threshold, true, opts...,
		)
	case tuf.GlobalRuleBlockForcePushesType:
		if len(gr.rulePatterns) == 0 {
			return fmt.Errorf("required flag --rule-pattern not set for global rule type '%s'", tuf.GlobalRuleBlockForcePushesType)
		}
		return repo.AddGlobalRuleBlockForcePushes(
			context.Background(), signer,
			gr.ruleName, gr.rulePatterns,
			true, opts...,
		)
	default:
		return fmt.Errorf("unknown global-rule type %q", gr.ruleType)
	}
}

// repoRemoveGlobalRule removes a global rule
func repoRemoveGlobalRule(o *options, gr globalRule) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}
	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}
	var opts []trustpolicyopts.Option
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}
	return repo.RemoveGlobalRule(
		context.Background(), signer, gr.ruleName, true, opts...,
	)
}

// repoUpdateGlobalRule updates a global rule
func repoUpdateGlobalRule(o *options, gr globalRule) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}
	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}
	var opts []trustpolicyopts.Option
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}
	switch gr.ruleType {
	case tuf.GlobalRuleThresholdType:
		if len(gr.rulePatterns) == 0 {
			return fmt.Errorf("required flag --rule-pattern not set for global rule type '%s'", tuf.GlobalRuleThresholdType)
		}

		return repo.UpdateGlobalRuleThreshold(context.Background(), signer, gr.ruleName, gr.rulePatterns, gr.threshold, true, opts...)

	case tuf.GlobalRuleBlockForcePushesType:
		if len(gr.rulePatterns) == 0 {
			return fmt.Errorf("required flag --rule-pattern not set for global rule type '%s'", tuf.GlobalRuleBlockForcePushesType)
		}

		return repo.UpdateGlobalRuleBlockForcePushes(context.Background(), signer, gr.ruleName, gr.rulePatterns, true, opts...)

	default:
		return tuf.ErrUnknownGlobalRuleType
	}
}
