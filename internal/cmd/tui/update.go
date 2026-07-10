// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// Update updates the model based on the message received.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case initDoneMsg:
		if msg.err != nil {
			m.errorMsg = fmt.Sprintf("Initialization failed: %v", msg.err)
			return m, nil
		}
		m.repo = msg.repo
		m.signer = msg.signer
		m.signerError = msg.signerError
		m.policyRulesScreen.rules = msg.rules
		m.trustGlobalRulesScreen.globalRules = msg.globalRules
		m.readOnly = msg.readOnly
		m.footer = msg.footer
		m.policyRulesScreen.updateRuleList()
		m.trustGlobalRulesScreen.updateGlobalRuleList()
		// Resize all lists now that readOnly/signerError are known — the earlier
		// WindowSizeMsg fired before these flags were set, so sizes must be corrected.
		m.resizeLists()
		m.screen = screenChoice
		return m, nil

	case spinner.TickMsg:
		if m.screen == screenLoading {
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeLists()
		return m, nil

	case tea.KeyMsg:
		// Global handlers (quit, back navigation)
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			// Only quit from non-form screens (avoid consuming 'q' in text inputs)
			if m.screen != screenPolicyAddRule && m.screen != screenPolicyEditRule &&
				m.screen != screenTrustAddGlobalRule && m.screen != screenTrustEditGlobalRule &&
				m.screen != screenPolicyPrincipalsForm {
				return m, tea.Quit
			}
		case "h":
			// Toggle help screen if not in form mode
			if m.screen != screenPolicyAddRule && m.screen != screenPolicyEditRule &&
				m.screen != screenTrustAddGlobalRule && m.screen != screenTrustEditGlobalRule &&
				m.screen != screenPolicyPrincipalsForm {
				if m.screen == screenHelp {
					// Toggle back
					m.screen = m.helpScreen.previousScreen
					return m, nil
				}
				// Go to help screen
				m.helpScreen.previousScreen = m.screen
				m.screen = screenHelp
				return m, nil
			}
		case "esc":
			m.footer = ""
			switch m.screen {
			case screenPolicy, screenTrust:
				m.screen = screenChoice
			case screenPolicyRules:
				if m.policyRulesScreen.confirmDelete {
					m.policyRulesScreen.confirmDelete = false
					m.policyRulesScreen.deleteTarget = ""
				} else {
					m.screen = screenPolicy
				}
			case screenPolicyAddRule, screenPolicyEditRule:
				m.screen = screenPolicyRules
			case screenPolicyPrincipalsForm:
				m.screen = screenPolicyPrincipals
			case screenPolicyPrincipals:
				m.screen = screenPolicy
			case screenHelp:
				m.screen = m.helpScreen.previousScreen
			case screenTrustGlobalRules, screenTrustAddGlobalRule, screenTrustEditGlobalRule:
				m.trustGlobalRulesScreen.handleEsc(&m)
			}
			return m, nil
		}

		// Screen-specific input handling
		switch m.screen {
		case screenChoice:
			return m.homeScreen.Update(msg, &m)
		case screenHelp:
			return m.helpScreen.Update(msg, &m)
		case screenTrust:
			return m.trustScreen.Update(msg, &m)
		case screenPolicyRules, screenPolicyAddRule, screenPolicyEditRule:
			return m.policyRulesScreen.Update(msg, &m)
		case screenTrustGlobalRules, screenTrustAddGlobalRule, screenTrustEditGlobalRule:
			return m.trustGlobalRulesScreen.Update(msg, &m)
		case screenPolicyPrincipals:
			return m.policyPrincipalsScreen.Update(msg, &m)
		case screenPolicyPrincipalsForm:
			return m.policyPrincipalsFormScreen.Update(msg, &m)
		}
	}

	// Delegate to active bubbles component per screen
	switch m.screen {
	case screenChoice:
		return m.homeScreen.Update(msg, &m)
	case screenHelp:
		return m.helpScreen.Update(msg, &m)
	case screenPolicy:
		return m.policyScreen.Update(msg, &m)
	case screenTrust:
		return m.trustScreen.Update(msg, &m)
	case screenPolicyRules, screenPolicyAddRule, screenPolicyEditRule:
		return m.policyRulesScreen.Update(msg, &m)
	case screenTrustGlobalRules, screenTrustAddGlobalRule, screenTrustEditGlobalRule:
		return m.trustGlobalRulesScreen.Update(msg, &m)
	case screenPolicyPrincipals:
		return m.policyPrincipalsScreen.Update(msg, &m)
	case screenPolicyPrincipalsForm:
		return m.policyPrincipalsFormScreen.Update(msg, &m)
	}

	return m, cmd
}

// splitAndTrim splits a comma-separated string and trims whitespace.
func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}
