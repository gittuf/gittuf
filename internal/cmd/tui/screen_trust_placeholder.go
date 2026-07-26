package tui

import "strings"

func trustPlaceholderMeta(current screen) (string, string) {
	switch current {
	case screenTrustKeysThresholds:
		return "Home › Trust › Keys & Thresholds", "Keys and threshold workflows land here."
	case screenTrustHooks:
		return "Home › Trust › Hooks", "Hook management workflows land here."
	case screenTrustPropagation:
		return "Home › Trust › Propagation", "Propagation directive workflows land here."
	case screenTrustGitHubApp:
		return "Home › Trust › GitHub App", "GitHub App trust workflows land here."
	case screenTrustLifecycle:
		return "Home › Trust › Lifecycle", "Stage, sign, apply, and inspect trust changes here."
	case screenTrustRepoNetwork:
		return "Home › Trust › Repo/Network", "Controller, network, and repository settings land here."
	default:
		return "Home › Trust", ""
	}
}

func (m model) renderTrustPlaceholder(current screen) string {
	title, body := trustPlaceholderMeta(current)

	var b strings.Builder
	b.WriteString(titleStyle.Render("Coming next"))
	b.WriteString("\n\n")
	b.WriteString(body)
	b.WriteString("\n\n")
	b.WriteString("Press Esc to return to the Trust menu.")

	return m.renderScreen(title, b.String(), renderActionHints(true))
}