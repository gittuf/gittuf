// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
)

const (
	colorRegularText = "#FFFFFF"
	colorFocus       = "#007AFF"
	colorBlur        = "#A0A0A0"
	colorFooter      = "#11ff00"
	colorSubtext     = "#555555"
	colorErrorMsg    = "#FF0000"
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

// renderWithMargin wraps content in the standard margin used by all screens.
func renderWithMargin(content string) string {
	return lipgloss.NewStyle().Margin(1, 2).Render(content)
}

// renderFooter returns the footer text styled in the footer color.
func renderFooter(text string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(colorFooter)).Render(text)
}

// renderErrorMsg returns error messages styled in the error color.
func renderErrorMsg(text string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(colorErrorMsg)).Render(text)
}

// renderFormScreen renders a form screen with a title, input fields, help text, and footer.
func (m model) renderFormScreen(title string) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(title) + "\n\n")
	for _, input := range m.inputs {
		b.WriteString(input.View() + "\n")
	}
	b.WriteString("\n" + "Press Tab to advance, Enter to advance/submit, and Esc to go back." + "\n")
	b.WriteString(renderFooter(m.footer))
	b.WriteString(renderErrorMsg(m.errorMsg))
	return renderWithMargin(b.String())
}

// renderListScreen renders a list with help text and footer.
func (m model) renderListScreen(l list.Model, helpText string, emptyMsg string, isEmpty bool) string {
	listView := l.View()
	if isEmpty {
		emptyMsgStyled := lipgloss.NewStyle().Foreground(lipgloss.Color(colorSubtext)).Render(emptyMsg)
		listView = l.Title + "\n\n" + emptyMsgStyled
	}

	return renderWithMargin(
		listView + "\n\n" +
			renderFooter(m.footer) +
			renderErrorMsg(m.errorMsg) +
			"\n" + helpText,
	)
}

// screenPolicyRulesHelp returns the help bar for the policy rules view screen.
func screenPolicyRulesHelp(readOnly bool) string {
	if readOnly {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(colorBlur)).Render(
			"esc: back  q: quit",
		)
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(colorBlur)).Render(
		"a: add  e: edit  d: delete  k: move-up  j: move-down  esc: back  q: quit",
	)
}

// screenTrustGlobalRulesHelp returns the help bar for the global rules view screen.
func screenTrustRootConfigHelp(readOnly bool) string {
	if readOnly {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(colorBlur)).Render(
			"p: principals  esc: back  q: quit",
		)
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(colorBlur)).Render(
		"p: principals  e: edit repo location  s: save changes  ↑/↓: change threshold  esc: back  q: quit",
	)
}

func screenTrustRootKeysHelp(readOnly bool) string {
	if readOnly {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(colorBlur)).Render(
			"esc: back  q: quit",
		)
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(colorBlur)).Render(
		"a: add  d: delete  esc: back  q: quit",
	)
}

func screenTrustGlobalRulesHelp(readOnly bool) string {
	if readOnly {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(colorBlur)).Render(
			"esc: back  q: quit",
		)
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(colorBlur)).Render(
		"a: add  e: edit  d: delete  esc: back  q: quit",
	)
}

// renderDeleteOverlay renders the delete confirmation prompt.
func renderDeleteOverlay(target string) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF0000")).
		Bold(true).
		Render(fmt.Sprintf("Delete rule %q? [y/n]", target))
}

// View renders the TUI.
func (m model) View() string {
	switch m.screen {
	case screenLoading:
		if m.errorMsg != "" {
			return renderWithMargin(
				titleStyle.Render("gittuf TUI") + "\n\n" +
					renderErrorMsg(m.errorMsg) + "\n\n" +
					lipgloss.NewStyle().Foreground(lipgloss.Color(colorBlur)).Render("Press Q or Ctrl+C to quit."),
			)
		}
		return renderWithMargin(
			titleStyle.Render("gittuf TUI") + "\n\n" +
				m.spinner.View() + " Loading, please wait...\n",
		)
	case screenChoice:
		return renderWithMargin(m.choiceList.View() + "\n" + renderFooter(m.footer) + renderErrorMsg(m.errorMsg))
	case screenPolicy:
		return renderWithMargin(m.policyScreenList.View() + "\n" + renderFooter(m.footer) + renderErrorMsg(m.errorMsg))
	case screenTrust:
		return renderWithMargin(m.trustScreenList.View() + "\n" + renderFooter(m.footer) + renderErrorMsg(m.errorMsg))
	case screenPolicyRules:
		overlay := ""
		if m.confirmDelete {
			overlay = "\n" + renderDeleteOverlay(m.deleteTarget) + "\n"
		}
		hint := ""
		if !m.readOnly {
			hint = "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color(colorSubtext)).Render(
				"Run `gittuf policy apply` to apply staged changes to the selected policy file.",
			)
		}

		emptyMsg := "No rules configured. Press 'A' to add one."
		return m.renderListScreen(m.ruleList, overlay+screenPolicyRulesHelp(m.readOnly)+hint, emptyMsg, len(m.rules) == 0)
	case screenTrustGlobalRules:
		overlay := ""
		if m.confirmDelete {
			overlay = "\n" + renderDeleteOverlay(m.deleteTarget) + "\n"
		}
		hint := ""
		if !m.readOnly {
			hint = "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color(colorSubtext)).Render(
				"Run `gittuf trust apply` to apply staged changes to the selected policy file.",
			)
		}

		emptyMsg := "No rules configured. Press 'A' to add one."
		return m.renderListScreen(m.globalRuleList, overlay+screenTrustGlobalRulesHelp(m.readOnly)+hint, emptyMsg, len(m.globalRules) == 0)
	case screenTrustRootConfig:
		hint := ""
		if !m.readOnly {
			hint = "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color(colorSubtext)).Render(
				"Run `gittuf trust apply` to apply staged changes to root metadata.",
			)
		}
		return renderWithMargin(
			m.trustConfig.View() + "\n" + screenTrustRootConfigHelp(m.readOnly) + hint + renderErrorMsg(m.errorMsg) + "\n" + renderFooter(m.footer),
		)
	case screenTrustRootKeys:
		overlay := ""
		if m.confirmDelete {
			overlay = "\n" + renderDeleteOverlay(m.deleteTarget) + "\n"
		}
		emptyMsg := "No root principals configured. Press 'A' to add one."
		return m.renderListScreen(m.rootKeyList, overlay+screenTrustRootKeysHelp(m.readOnly), emptyMsg, len(m.trustConfig.Principals) == 0)
	case screenTrustAddRootKey:
		return m.renderFormScreen("Add Root Key")
	case screenPolicyAddRule:
		return m.renderFormScreen("Add Rule")
	case screenPolicyEditRule:
		return m.renderFormScreen("Edit Rule")
	case screenTrustAddGlobalRule:
		return m.renderFormScreen("Add Global Rule")
	case screenTrustEditGlobalRule:
		return m.renderFormScreen("Edit Global Rule")
	default:
		return "Unknown screen"
	}
}
