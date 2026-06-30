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
	colorAccent      = "#007AFF"
	colorBorder      = "#1E3A5F"
	colorBlur        = "#A0A0A0"
	colorFooter      = "#007AFF"
	colorSubtext     = "#A0A0A0"
	colorErrorMsg    = "#FF5252"
	colorStatusBg    = "#1A1A2E"
	colorEditMode    = "#007AFF"
	colorReadOnly    = "#FF6B6B"
	colorHelpKeyBg   = "#2A2A3E"
)

var (
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorAccent)).
			Padding(0, 2).
			MarginTop(1).
			Bold(true)

	itemStyle = lipgloss.NewStyle().
			PaddingLeft(4).
			Foreground(lipgloss.Color(colorRegularText))

	selectedItemStyle = lipgloss.NewStyle().
				PaddingLeft(4).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color(colorFocus)).
				Bold(true)

	focusedStyle = lipgloss.NewStyle().
			PaddingLeft(4)

	blurredStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorBlur))

	cursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorAccent))

	statusTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorAccent)).
				Background(lipgloss.Color(colorStatusBg)).
				Bold(true).
				Padding(0, 1)

	statusScreenStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorBlur)).
				Background(lipgloss.Color(colorStatusBg)).
				Padding(0, 1)

	statusEditModeStyle = lipgloss.NewStyle().
				Background(lipgloss.Color(colorEditMode)).
				Foreground(lipgloss.Color("#FFFFFF")).
				Padding(0, 1).
				Bold(true)

	statusReadOnlyStyle = lipgloss.NewStyle().
				Background(lipgloss.Color(colorReadOnly)).
				Foreground(lipgloss.Color("#FFFFFF")).
				Padding(0, 1).
				Bold(true)

	// Screen box — border around the content
	screenBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorBorder)).
			Padding(0, 1)

	// Help bar key and description styles
	helpKeyStyle = lipgloss.NewStyle().
			Background(lipgloss.Color(colorHelpKeyBg)).
			Foreground(lipgloss.Color(colorAccent)).
			Padding(0, 1)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorBlur)).
			Padding(0, 1)
)

// renderWithMargin wraps content in the standard margin used by all screens.
func renderWithMargin(content string) string {
	return lipgloss.NewStyle().Margin(1, 2).Render(content)
}

// renderFooter returns the footer text styled in the footer color.
func renderFooter(text string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(colorFooter)).Render(text)
}

// signerNoticeLines returns how many terminal rows the signer-error notice will
// occupy when rendered at the given terminal width. This matches the exact same
// Width() call used in renderFooterBox so both the layout reservation and the
// rendered output always agree.
func signerNoticeLines(signerError string, termWidth int) int {
	if signerError == "" {
		return 0
	}
	noticeWidth := termWidth - 6
	if noticeWidth < 20 {
		noticeWidth = 20
	}
	wrapped := lipgloss.NewStyle().Width(noticeWidth).Render("Notice: " + signerError)
	return strings.Count(wrapped, "\n") + 1
}

// renderFooterBox wraps the footer in a rich info box if the user requests help in read-only mode.
func renderFooterBox(m model) string {
	baseFooter := renderFooter(m.footer)
	var signerNotice string

	if m.signerError != "" {
		signerNoticeWidth := m.width - 6
		if signerNoticeWidth < 20 {
			signerNoticeWidth = 20
		}
		wrappedErr := lipgloss.NewStyle().Width(signerNoticeWidth).Render("Notice: " + m.signerError)
		signerNotice = lipgloss.NewStyle().Foreground(lipgloss.Color(colorBlur)).Italic(true).Render(wrappedErr)
	}

	if m.readOnly && m.showHelp {
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorReadOnly)).
			Padding(1, 2).
			MarginTop(1)

		title := lipgloss.NewStyle().Foreground(lipgloss.Color(colorReadOnly)).Bold(true).Render("Read-only mode")
		explainWidth := m.width - 10
		if explainWidth < 20 {
			explainWidth = 20
		}
		explain := lipgloss.NewStyle().Foreground(lipgloss.Color(colorRegularText)).Width(explainWidth).Render("You are currently in read-only mode.")
		if m.signerError != "" {
			explain = lipgloss.NewStyle().Foreground(lipgloss.Color(colorRegularText)).Width(explainWidth).Render(m.signerError)
		}
		tip := lipgloss.NewStyle().Foreground(lipgloss.Color(colorFocus)).Render("Tip: You can still use the TUI to navigate and view all rules.")
		fix := lipgloss.NewStyle().Foreground(lipgloss.Color(colorSubtext)).Render("Fix steps:\n  - Run: " + lipgloss.NewStyle().Foreground(lipgloss.Color(colorFocus)).Bold(true).Render("gittuf trust init") + "\n  - Ensure your GPG/SSH/Fulcio key is correctly configured in Git")

		inner := lipgloss.JoinVertical(lipgloss.Left, title, "", explain, tip, "", fix, "", baseFooter)
		return box.Render(inner)
	}

	if signerNotice != "" {
		// The status bar already shows the "Read-only" badge, so the footer text
		// is redundant here. Use a double newline so there is one clear blank line
		// of breathing room between the help key bar and the notice.
		return "\n\n" + signerNotice
	}

	return baseFooter
}

// renderErrorMsg returns error messages styled in the error color.
func renderErrorMsg(text string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(colorErrorMsg)).Render(text)
}

// renderStatusBar renders the top status bar showing screen name and current mode.
func renderStatusBar(screenName string, readOnly bool, width int) string {
	if width == 0 {
		width = 80
	}
	title := statusTitleStyle.Render("gittuf")
	screen := statusScreenStyle.Render("› " + screenName)
	left := lipgloss.JoinHorizontal(lipgloss.Top, title, screen)

	var modeTag string
	if readOnly {
		modeTag = statusReadOnlyStyle.Render(" Read-only ")
	} else {
		modeTag = statusEditModeStyle.Render(" Edit Mode ")
	}

	// Calculate remaining space for the gap. bar has Padding(0, 1) = 2 chars total.
	gapWidth := width - lipgloss.Width(left) - lipgloss.Width(modeTag) - 2
	if gapWidth < 0 {
		gapWidth = 0
	}

	gap := lipgloss.NewStyle().
		Background(lipgloss.Color(colorStatusBg)).
		Width(gapWidth).
		Render("")

	return lipgloss.NewStyle().
		Background(lipgloss.Color(colorStatusBg)).
		Foreground(lipgloss.Color(colorRegularText)).
		Padding(0, 1).
		Width(width).
		Render(lipgloss.JoinHorizontal(lipgloss.Top, left, gap, modeTag))
}

// renderHelpKey renders a single styled key + description pair.
func renderHelpKey(key, desc string) string {
	k := helpKeyStyle.Render(key)
	d := helpDescStyle.Render(desc)
	return lipgloss.JoinHorizontal(lipgloss.Top, k, d)
}

// renderStyledHelp renders the full help bar from a list of key/desc pairs.
func renderStyledHelp(pairs [][2]string) string {
	parts := make([]string, 0, len(pairs))
	for _, p := range pairs {
		parts = append(parts, renderHelpKey(p[0], p[1]))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

// renderFormScreen renders a form screen with a title, input fields, help text, and footer.
func (m model) renderFormScreen(formTitle string, breadcrumb string) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(formTitle) + "\n\n")
	for _, input := range m.inputs {
		b.WriteString(input.View() + "\n")
	}
	b.WriteString("\n" + "Press Tab to advance, Enter to advance/submit, and Esc to go back." + "\n")
	b.WriteString(renderFooterBox(m))
	b.WriteString(renderErrorMsg(m.errorMsg))
	return lipgloss.JoinVertical(lipgloss.Left,
		renderStatusBar(breadcrumb, m.readOnly, m.width),
		renderWithMargin(b.String()),
	)
}

// renderActionHints returns the consistent action hints requested for the bottom of screens.
func renderActionHints(readOnly bool) string {
	if readOnly {
		return "\n" + renderStyledHelp([][2]string{
			{"h", "help"},
			{"esc", "back"},
			{"q", "quit"},
		})
	}
	return "\n" + renderStyledHelp([][2]string{
		{"a", "add"},
		{"e", "edit"},
		{"d", "delete"},
		{"h", "help"},
		{"esc", "back"},
		{"q", "quit"},
	})
}

// renderDeleteOverlay renders the delete confirmation prompt.
func renderDeleteOverlay(target string) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF0000")).
		Bold(true).
		Render(fmt.Sprintf("Delete rule %q? [y/n]", target))
}

// renderScreen provides unified boilerplate layout containing the title, central visual box, and footers.
func (m model) renderScreen(title string, listContent string, overlays string) string {
	h, v := lipgloss.NewStyle().Margin(1, 2).GetFrameSize()

	boxWidth := m.width - h - 2
	if boxWidth < 0 {
		boxWidth = 0
	}

	heightOffset := 7
	if m.readOnly {
		heightOffset = 9
		if m.signerError != "" {
			// Dynamic: fixed overhead (7) + actual notice lines so the box
			// perfectly fills any terminal regardless of width or screen size.
			// Fixed overhead accounts for: 2 blank rows (between box and helpkeys),
			// 1 helpkeys row, 1 blank gap (between helpkeys and notice).
			heightOffset = 7 + signerNoticeLines(m.signerError, m.width)
		}
	}
	boxHeight := m.height - v - heightOffset
	if boxHeight < 0 {
		boxHeight = 0
	}

	content := screenBoxStyle.Width(boxWidth).Height(boxHeight).Render(listContent)

	return lipgloss.JoinVertical(lipgloss.Left,
		renderStatusBar(title, m.readOnly, m.width),
		renderWithMargin(
			content+"\n"+
				overlays+
				renderFooterBox(m)+
				renderErrorMsg(m.errorMsg),
		),
	)
}

// renderListOrEmpty generates dynamic content evaluating array length against fallback empty states.
func (m model) renderListOrEmpty(l list.Model, length int, emptyTitleText string) string {
	if length > 0 {
		return l.View()
	}

	h, v := lipgloss.NewStyle().Margin(1, 2).GetFrameSize()

	emptyTitle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorRegularText)).Bold(true).Render(emptyTitleText)
	emptyDesc := lipgloss.NewStyle().Foreground(lipgloss.Color(colorSubtext)).Render("Get started by adding your first rule.\n\nPress 'a' to add rule")
	emptyState := lipgloss.JoinVertical(lipgloss.Center, emptyTitle, "\n", emptyDesc)

	boxWidth := m.width - h - 2
	heightOffset := 8
	if m.readOnly {
		heightOffset = 10
		if m.signerError != "" {
			heightOffset = 12
		}
	}
	boxHeight := m.height - v - heightOffset
	if boxWidth < 0 {
		boxWidth = 0
	}
	if boxHeight < 0 {
		boxHeight = 0
	}

	return lipgloss.Place(boxWidth, boxHeight, lipgloss.Center, lipgloss.Center, emptyState)
}

// View renders the TUI.
func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return m.spinner.View() + " Loading TUI...\n"
	}

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
		return m.renderScreen("Home", m.choiceList.View(), renderActionHints(m.readOnly))

	case screenPolicy:
		return m.renderScreen("Home › Policy", m.policyScreenList.View(), renderActionHints(m.readOnly))

	case screenTrust:
		return m.renderScreen("Home › Trust", m.trustScreenList.View(), renderActionHints(m.readOnly))

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

		listView := m.renderListOrEmpty(m.ruleList, len(m.rules), "No rules configured")
		overlays := overlay + renderActionHints(m.readOnly) + hint

		return m.renderScreen("Home › Policy › Rules", listView, overlays)

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

		listView := m.renderListOrEmpty(m.globalRuleList, len(m.globalRules), "No global rules configured")
		overlays := overlay + renderActionHints(m.readOnly) + hint

		return m.renderScreen("Home › Trust › Global Rules", listView, overlays)

	case screenPolicyAddRule:
		return m.renderFormScreen("Add Rule", "Home › Policy › Rules › Add")

	case screenPolicyEditRule:
		return m.renderFormScreen("Edit Rule", "Home › Policy › Rules › Edit")

	case screenTrustAddGlobalRule:
		return m.renderFormScreen("Add Global Rule", "Home › Trust › Global Rules › Add")

	case screenTrustEditGlobalRule:
		return m.renderFormScreen("Edit Global Rule", "Home › Trust › Global Rules › Edit")

	case screenHelp:
		h, v := lipgloss.NewStyle().Margin(1, 2).GetFrameSize()
		boxWidth := m.width - h - 2
		boxHeight := m.height - v - 8
		if boxWidth < 0 {
			boxWidth = 0
		}
		if boxHeight < 0 {
			boxHeight = 0
		}

		title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorFocus)).Render("Help")
		helpContent := title + "\n\n" +
			"- ↑ ↓ : Navigate\n" +
			"- Enter : Select\n" +
			"- a : Add rule\n" +
			"- e : Edit\n" +
			"- d : Delete\n" +
			"- esc : Back\n" +
			"- q : Quit\n"

		centeredContent := lipgloss.Place(boxWidth, boxHeight, lipgloss.Center, lipgloss.Center, helpContent)
		return m.renderScreen("Help", centeredContent, "")

	default:
		return "Unknown screen"
	}
}
