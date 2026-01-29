// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	colorRegularText = "#FFFFFF"
	colorFocus       = "#007AFF"
	colorBlur        = "#A0A0A0"
	colorFooter      = "#11ff00"
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

// View renders the TUI
func (m model) View() string {
	switch m.screen {
	case screenMain:
		// Show apply hint only when not in read-only mode.
		hint := ""
		if !m.readOnly {
			hint = "Run `gittuf policy apply` to apply staged changes to the selected policy file"
		}
		return lipgloss.NewStyle().Margin(1, 2).Render(
			m.mainList.View() + "\n" +
				lipgloss.NewStyle().Foreground(lipgloss.Color(colorFooter)).Render(m.footer) +
				"\n" + hint,
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
	default:
		return "Unknown screen"
	}
}
