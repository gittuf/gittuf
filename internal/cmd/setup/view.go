// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	colorRegularText = "#FFFFFF"
	colorFocus       = "#007AFF"
	colorFooter      = "#11ff00"
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
)

// renderWithMargin wraps content in the standard margin used by all screens.
// It also word-wraps content to fit the max width (defined by terminal width or const)
// to prevent overflow.
func (m model) renderWithMargin(content string) string {
	style := lipgloss.NewStyle().Margin(1, 2)
	if m.width > 0 {
		h, _ := style.GetFrameSize()
		content = ansi.Wordwrap(content, m.width-h, "")
	}
	return style.Render(content)
}

// renderFooter returns the footer text styled in the footer color.
func renderFooter(text string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(colorFooter)).Render(text)
}

// renderErrorMsg returns error messages styled in the error color.
func renderErrorMsg(text string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(colorErrorMsg)).Render(text)
}

// View renders the TUI
func (m model) View() string {
	switch m.screen {
	case screenChoice:
		return m.renderWithMargin("Welcome! This wizard will help you setup gittuf on your repository." + "\n" + m.choiceList.View())
	case screenMaintainerSelections:
		content := "Hello maintainer! Let's get you started with gittuf.\n\n"
		if m.rootExists == true {
			content += "Looks like gittuf is already setup on your repository. Please select where you would like to be added.\n"
		} else {
			content += "Looks like gittuf isn't initialized yet. Let's initialize it and add you to the policy.\n"
		}
		for i, item := range m.rootChoices {
			cursor := " "
			if m.rootCursor == i {
				cursor = ">"
			}
			checked := "[ ]"
			if m.rootSelected[i] {
				checked = "[x]"
			}
			content += cursor + checked + " " + item + "\n"
		}
		// submit button
		content += "\n"
		if m.rootCursor == len(m.rootChoices) {
			content += "> [ Submit ]\n"
		} else {
			content += "  [ Submit ]\n"
		}
		return m.renderWithMargin(content)

	case screenTransportConfirm:
		content := "gittuf can automatically manage your metadata in the background.\n" +
			"See https://gittuf.dev/documentation/contributors for more information.\n\n" +
			"Would you like to enable automatic push recording? [y/n]"
		return m.renderWithMargin(content)
	case screenTransport:
		content := ""
		if m.transportRunning {
			content += m.spinner.View() + " Setting up transport..."
		}
		if m.errorMsg != "" {
			content += "\n" + renderErrorMsg(m.errorMsg) + "\n"
		}
		for _, step := range m.transportSteps {
			content += "✔︎ " + step + "\n"
		}
		content += renderFooter("\n" + m.footer)
		return m.renderWithMargin(content)
	case screenAbort:
		return m.renderWithMargin(renderErrorMsg("gittuf setup has been cancelled. Any changes made to your repository have been reverted."))
	case screenConclusion:
		return m.renderWithMargin("Setup complete!" + "\n" + "Please see https://gittuf.dev/ for further documentation.\n\n" + "Press Q to quit.")
	default:
		return "Unknown screen"
	}
}
