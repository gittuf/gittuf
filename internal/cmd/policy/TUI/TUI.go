// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/spf13/cobra"
)

var (
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 2).
			MarginTop(1).
			Bold(true)

	itemStyle = lipgloss.NewStyle().
			PaddingLeft(4).
			Foreground(lipgloss.Color("#FFFFFF"))

	selectedItemStyle = lipgloss.NewStyle().
				PaddingLeft(4).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#007AFF"))

	focusedStyle = lipgloss.NewStyle().
			PaddingLeft(4)

	blurredStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A0A0A0"))

	cursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))
)

type screen int

const (
	screenMain screen = iota
	screenAddRule
	screenRemoveRule
	screenListRules
	screenReorderRules
)

type rule struct {
	name    string
	pattern string
	key     string
}

type model struct {
	screen     screen
	mainList   list.Model
	rules      []rule
	ruleList   list.Model
	inputs     []textinput.Model
	focusIndex int
	cursorMode cursor.Mode
	repo       *gittuf.Repository
	signer     dsse.SignerVerifier
	policyName string
	options    *options
	footer     string
}

// initialModel returns the initial model for the Terminal UI
func initialModel(o *options) model {
	repo, err := gittuf.LoadRepository()
	if err != nil {
		return model{}
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return model{}
	}

	// Initialize the model
	m := model{
		screen:     screenMain,
		cursorMode: cursor.CursorBlink,
		repo:       repo,
		signer:     signer,
		policyName: o.policyName,
		rules:      getCurrRules(o),
		options:    o,
	}

	// Set up the main list items
	mainItems := []list.Item{
		item{title: "Add Rule", desc: "Add a new policy rule"},
		item{title: "Remove Rule", desc: "Remove an existing policy rule"},
		item{title: "List Rules", desc: "View all current policy rules"},
		item{title: "Reorder Rules", desc: "Change the order of policy rules"},
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
			t.Placeholder = "Enter Key Here"
			t.Prompt = "Authorize Key:"
			t.PromptStyle = blurredStyle
			t.TextStyle = blurredStyle
		}

		m.inputs[i] = t
	}

	return m
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

// `Update updates the model based on the message received
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
			if m.screen == screenAddRule {
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
	}

	return m, cmd
}

// `updateRuleList` updates the rule list within TUI
func (m *model) updateRuleList() {
	items := make([]list.Item, len(m.rules))
	for i, rule := range m.rules {
		items[i] = item{title: rule.name, desc: fmt.Sprintf("Pattern: %s, Key: %s", rule.pattern, rule.key)}
	}
	m.ruleList.SetItems(items)
}

// `View` renders the TUI
func (m model) View() string {
	switch m.screen {
	case screenMain:
		return lipgloss.NewStyle().Margin(1, 2).Render(
			m.mainList.View() + "\n" +
				lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).Render(m.footer) +
				"\nRun `gittuf policy apply` to apply staged changes to the selected policy file",
		)
	case screenAddRule:
		var b strings.Builder
		b.WriteString(titleStyle.Render("Add Rule") + "\n\n")
		for _, input := range m.inputs {
			b.WriteString(input.View() + "\n")
		}
		b.WriteString("\nPress Enter to add, Left Arrow to go back\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Render(m.footer))
		return lipgloss.NewStyle().Margin(1, 2).Render(b.String())
	case screenRemoveRule:
		return lipgloss.NewStyle().Margin(1, 2).Render(
			m.ruleList.View() + "\n\n" +
				lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Render(m.footer) +
				"\nPress Enter to remove selected rule, Left Arrow to go back",
		)
	case screenListRules:
		var sb strings.Builder
		sb.WriteString(titleStyle.Render("Current Rules") + "\n\n")
		for _, rule := range m.rules {
			sb.WriteString(fmt.Sprintf("- %s\n  Pattern: %s\n  Key: %s\n\n",
				lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true).Render(rule.name),
				lipgloss.NewStyle().Foreground(lipgloss.Color("#555555")).Render(rule.pattern),
				lipgloss.NewStyle().Foreground(lipgloss.Color("#555555")).Render(rule.key)))
		}
		sb.WriteString("\nPress Left Arrow to go back")
		return lipgloss.NewStyle().Margin(1, 2).Render(sb.String())
	case screenReorderRules:
		return lipgloss.NewStyle().Margin(1, 2).Render(
			m.ruleList.View() + "\n\n" +
				lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Render(m.footer) +
				"\nUse 'u' to move up, 'd' to move down, Left Arrow to go back",
		)
	default:
		return "Unknown screen"
	}
}

// getCurrRules returns the current rules from the policy file
func getCurrRules(o *options) []rule {
	repo, err := gittuf.LoadRepository()
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
	repo, err := gittuf.LoadRepository()
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	authorizedKeys := []tuf.Principal{}
	for _, key := range keyPath {
		key, err := gittuf.LoadPublicKey(key)
		if err != nil {
			return err
		}

		authorizedKeys = append(authorizedKeys, key)
	}

	res := repo.AddDelegation(context.Background(), signer, o.policyName, rule.name, authorizedKeys, []string{rule.pattern}, 1, true)

	return res
}

// repoRemoveRule removes a rule from the policy file
func repoRemoveRule(o *options, rule rule) error {
	repo, err := gittuf.LoadRepository()
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
	repo, err := gittuf.LoadRepository()
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
		"name of policy file to add rule to",
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
		PreRunE:           common.CheckIfSigningViableWithFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
