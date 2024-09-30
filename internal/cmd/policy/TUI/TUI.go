package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/gittuf/gittuf/internal/repository"
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
	cursorMode textinput.CursorMode
	repo       *repository.Repository
	signer     dsse.SignerVerifier
	policyName string
	options    *options
}

func initialModel(o *options) model {
	repo, err := repository.LoadRepository()
	if err != nil {
		return model{}
	}

	signer, err := common.LoadSigner(o.p.SigningKey)
	if err != nil {
		return model{}
	}
	m := model{
		screen:     screenMain,
		cursorMode: textinput.CursorBlink,
		repo:       repo,
		signer:     signer,
		policyName: o.policyName,
		rules:      getCurrRules(o),
		options:    o,
	}

	mainItems := []list.Item{
		item{title: "Add Rule", desc: "Add a new policy rule"},
		item{title: "Remove Rule", desc: "Remove an existing policy rule"},
		item{title: "List Rules", desc: "View all current policy rules"},
		item{title: "Reorder Rules", desc: "Change the order of policy rules"},
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = selectedItemStyle
	delegate.Styles.SelectedDesc = selectedItemStyle
	delegate.Styles.NormalTitle = itemStyle
	delegate.Styles.NormalDesc = itemStyle

	m.mainList = list.New(mainItems, delegate, 0, 0)
	m.mainList.Title = "gittuf Policy Operations"
	m.mainList.SetShowStatusBar(false)
	m.mainList.SetFilteringEnabled(false)
	m.mainList.Styles.Title = titleStyle
	m.mainList.SetShowHelp(false)

	m.ruleList = list.New([]list.Item{}, delegate, 0, 0)
	m.ruleList.Title = "Current Rules"
	m.ruleList.SetShowStatusBar(false)
	m.ruleList.SetFilteringEnabled(false)
	m.ruleList.Styles.Title = titleStyle
	m.ruleList.SetShowHelp(false)

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

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

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
				m.screen = screenMain
				return m, nil
			}
		case "enter":
			if m.screen == screenMain {
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
			} else if m.screen == screenAddRule {
				if m.focusIndex == len(m.inputs)-1 {
					m.rules = append(m.rules, rule{
						name:    m.inputs[0].Value(),
						pattern: m.inputs[1].Value(),
						key:     m.inputs[2].Value(),
					})
					authorizedKeys := []string{m.inputs[2].Value()}
					repoAddRule(m.options, m.rules[len(m.rules)-1], authorizedKeys)
					m.updateRuleList()
					m.screen = screenMain
				}
			} else if m.screen == screenRemoveRule {
				if i, ok := m.ruleList.SelectedItem().(item); ok {
					for idx, rule := range m.rules {
						if rule.name == i.title {
							m.rules = append(m.rules[:idx], m.rules[idx+1:]...)
							break
						}
					}
					repoRemoveRule(m.options, rule{name: i.title})
					m.updateRuleList()
					m.screen = screenMain
				}
			}
		case "u":
			if m.screen == screenReorderRules {
				if i := m.ruleList.Index(); i > 0 {
					m.rules[i], m.rules[i-1] = m.rules[i-1], m.rules[i]
					repoReorderRules(m.options, m.rules)
					m.updateRuleList()
					m.ruleList.Select(i - 1)
				}
			}
		case "d":
			if m.screen == screenReorderRules {
				if i := m.ruleList.Index(); i < len(m.rules)-1 {
					m.rules[i], m.rules[i+1] = m.rules[i+1], m.rules[i]
					repoReorderRules(m.options, m.rules)
					m.updateRuleList()
					m.ruleList.Select(i + 1)
				}
			}
		case "tab", "shift+tab", "up", "down":
			if m.screen == screenAddRule {
				s := msg.String()
				if s == "up" || s == "shift+tab" {
					if m.focusIndex > 0 {
						m.focusIndex--
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

func (m *model) updateRuleList() {
	items := make([]list.Item, len(m.rules))
	for i, rule := range m.rules {
		items[i] = item{title: rule.name, desc: fmt.Sprintf("Pattern: %s, Key: %s", rule.pattern, rule.key)}
	}
	m.ruleList.SetItems(items)
}

func (m model) View() string {
	switch m.screen {
	case screenMain:
		return lipgloss.NewStyle().Margin(1, 2).Render(m.mainList.View())
	case screenAddRule:
		var b strings.Builder
		b.WriteString(titleStyle.Render("Add Rule") + "\n\n")
		for _, input := range m.inputs {
			b.WriteString(input.View() + "\n")
		}
		b.WriteString("\nPress Enter to add, Left Arrow to go back")
		return lipgloss.NewStyle().Margin(1, 2).Render(b.String())
	case screenRemoveRule:
		return lipgloss.NewStyle().Margin(1, 2).Render(
			m.ruleList.View() + "\n\nPress Enter to remove selected rule, Left Arrow to go back",
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
			m.ruleList.View() + "\n\nUse 'u' to move up, 'd' to move down, Left Arrow to go back",
		)
	default:
		return "Unknown screen"
	}
}

func getCurrRules(o *options) []rule {
	repo, err := repository.LoadRepository()
	if err != nil {
		return nil
	}

	rules, err := repo.ListRules(context.Background(), o.policyName)
	if err != nil {
		return nil
	}

	var currRules []rule
	for _, r := range rules {
		currRules = append(currRules, rule{
			name:    r.Delegation.Name,
			pattern: strings.Join(r.Delegation.Paths, ", "),
			key:     strings.Join(r.Delegation.Role.KeyIDs, ", "),
		})
	}
	return currRules
}

func repoAddRule(o *options, rule rule, keyPath []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	signer, err := common.LoadSigner(o.p.SigningKey)
	if err != nil {
		return err
	}

	authorizedKeys := []*tuf.Key{}
	for _, key := range keyPath {
		key, err := common.LoadPublicKey(key)
		if err != nil {
			return err
		}

		authorizedKeys = append(authorizedKeys, key)
	}

	return repo.AddDelegation(context.Background(), signer, o.policyName, rule.name, authorizedKeys, []string{rule.pattern}, 1, true)
}

func repoRemoveRule(o *options, rule rule) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	signer, err := common.LoadSigner(o.p.SigningKey)
	if err != nil {
		return err
	}
	return repo.RemoveDelegation(context.Background(), signer, o.policyName, rule.name, true)
}

func repoReorderRules(o *options, rules []rule) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	signer, err := common.LoadSigner(o.p.SigningKey)
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

func startTUI(o *options) error {
	p := tea.NewProgram(initialModel(o), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

type options struct {
	p          *persistent.Options
	policyName string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.policyName,
		"target-ref",
		"policy",
		"specify which policy ref should be inspected",
	)
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
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
