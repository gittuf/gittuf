// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"bufio"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

func isFormScreen(s screen) bool {
	switch s {
	case screenPolicyAddRule, screenPolicyEditRule,
		screenPolicyPrincipalsForm,
		screenTrustAddGlobalRule, screenTrustEditGlobalRule,
		screenTrustKeyForm, screenTrustThresholdForm,
		screenTrustAddHookForm, screenTrustUpdateHookForm, screenTrustRemoveHookForm,
		screenTrustAddPropagationForm, screenTrustUpdatePropagationForm, screenTrustRemovePropagationForm,
		screenTrustAddGitHubAppForm, screenTrustGitHubAppActionForm,
		screenTrustRepoForm, screenTrustRepoLocationForm,
		screenVerifyRefForm, screenVerifyMergeableForm:
		return true
	default:
		return false
	}
}

// Update updates the model based on the message received.
func (m model) updateInternal(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case initDoneMsg:
		if msg.err != nil {
			m.readOnly = true
			m.openErrorDialog("Initialization Failed", msg.err.Error())
			m.screen = screenChoice
			return m, nil
		}
		m.repo = msg.repo
		m.signer = msg.signer
		m.signerError = msg.signerError
		m.policyRulesScreen.rules = msg.rules
		m.trustGlobalRulesScreen.globalRules = msg.globalRules
		m.readOnly = msg.readOnly
		m.footer = msg.footer
		m.applyReadOnlyMenus()
		m.policyRulesScreen.updateRuleList()
		m.trustGlobalRulesScreen.updateGlobalRuleList()
		// Resize all lists now that readOnly/signerError are known — the earlier
		// WindowSizeMsg fired before these flags were set, so sizes must be corrected.
		m.resizeLists()
		m.screen = screenChoice
		return m, nil

	case policyLifecycleResultMsg:
		if msg.err != nil {
			m.openErrorDialog(msg.action+" Failed", msg.err.Error())
		} else {
			m.closeErrorDialog()
			m.footer = msg.msg
		}
		return m, nil

	case spinner.TickMsg:
		if m.screen == screenLoading || m.verifying {
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeLists()
		if msg.Width > 4 {
			m.logViewport.Width = msg.Width - 4
		} else {
			m.logViewport.Width = msg.Width
		}
		if msg.Height > 6 {
			m.logViewport.Height = msg.Height - 6
		} else {
			m.logViewport.Height = msg.Height
		}
		return m, nil

	case logBatchMsg:
		if len(msg.lines) > 0 {
			for _, line := range msg.lines {
				m.logsBuf.WriteString(line)
				m.logsBuf.WriteByte('\n')
			}
			m.logViewport.SetContent(m.logsBuf.String())
			m.logViewport.GotoBottom()
		}
		if m.verifying {
			return m, logFlushTick(m.logCh)
		}
		return m, nil

	case verifyResultMsg:
		m.verifying = false
		if m.logCh != nil {
			remaining := drainLogChannel(m.logCh)
			for _, line := range remaining {
				m.logsBuf.WriteString(line)
				m.logsBuf.WriteByte('\n')
			}
			if len(remaining) > 0 {
				m.logViewport.SetContent(m.logsBuf.String())
				m.logViewport.GotoBottom()
			}
			m.logCh = nil
		}
		if msg.err != nil {
			m.errorMsg = fmt.Sprintf("Verification failed: %v", msg.err)
		} else {
			m.errorMsg = ""
			m.footer = msg.successMsg
			m.screen = screenVerify
		}
		return m, nil

	case tea.KeyMsg:
		if m.verifying {
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			m.logViewport, cmd = m.logViewport.Update(msg)
			return m, cmd
		}
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.errorDialog != nil {
			switch msg.String() {
			case "q":
				return m, tea.Quit
			case "enter", "esc":
				m.closeErrorDialog()
			}
			return m, nil
		}

		switch msg.String() {
		case "q":
			if !isFormScreen(m.screen) {
				return m, tea.Quit
			}
		case "h":
			if !isFormScreen(m.screen) {
				m.footer = ""
				m.errorMsg = ""
				if m.screen == screenHelp {
					m.screen = m.helpScreen.previousScreen
					return m, nil
				}
				m.helpScreen.previousScreen = m.screen
				m.screen = screenHelp
				return m, nil
			}
		case "esc":
			m.footer = ""
			m.errorMsg = ""
			switch m.screen {
			case screenPolicy, screenTrust, screenVerify:
				m.screen = screenChoice
			case screenPolicyRules:
				if m.policyRulesScreen.confirmDelete {
					m.policyRulesScreen.confirmDelete = false
					m.policyRulesScreen.deleteTarget = ""
				} else {
					m.screen = screenPolicy
				}
			case screenPolicyLifecycle:
				m.screen = screenPolicy
			case screenPolicyLifecycleForm:
				m.screen = screenPolicyLifecycle
			case screenPolicyAddRule, screenPolicyEditRule:
				m.screen = screenPolicyRules
			case screenPolicyPrincipalsForm:
				m.screen = screenPolicyPrincipals
				if m.policyPrincipalsFormScreen.action == "Add Person" || m.policyPrincipalsFormScreen.action == "Add Standalone Key(s)" {
					m.policyPrincipalsScreen.addChoice = true
				}
			case screenPolicyPrincipals:
				if m.policyPrincipalsScreen.addChoice {
					m.policyPrincipalsScreen.addChoice = false
				} else {
					m.screen = screenPolicy
				}
			case screenHelp:
				m.screen = m.helpScreen.previousScreen
			case screenTrustGlobalRules, screenTrustAddGlobalRule, screenTrustEditGlobalRule:
				m.trustGlobalRulesScreen.handleEsc(&m)
			case screenTrustKeysThresholds:
				m.screen = screenTrust
			case screenTrustKeyForm, screenTrustThresholdForm:
				m.screen = screenTrustKeysThresholds
			case screenTrustAddHookForm, screenTrustUpdateHookForm, screenTrustRemoveHookForm:
				m.screen = screenTrustHooks
			case screenTrustAddPropagationForm, screenTrustUpdatePropagationForm, screenTrustRemovePropagationForm:
				m.screen = screenTrustPropagation
			case screenTrustAddGitHubAppForm, screenTrustGitHubAppActionForm:
				m.screen = screenTrustGitHubApp
			case screenTrustRepoForm, screenTrustRepoLocationForm:
				m.screen = screenTrustRepoNetwork
			case screenTrustLifecycle, screenTrustHooks, screenTrustGitHubApp, screenTrustRepoNetwork:
				m.screen = screenTrust
			case screenTrustPropagation:
				m.screen = screenTrust
			case screenVerifyRefForm, screenVerifyMergeableForm:
				m.screen = screenVerify
			}
			return m, nil
		}

		switch m.screen {
		case screenChoice:
			return m.homeScreen.Update(msg, &m)
		case screenHelp:
			return m.helpScreen.Update(msg, &m)
		case screenTrust:
			return m.trustScreen.Update(msg, &m)
		case screenPolicyLifecycle, screenPolicyLifecycleForm:
			return m.policyLifecycleScreen.Update(msg, &m)
		case screenTrustKeysThresholds, screenTrustKeyForm, screenTrustThresholdForm:
			return m.trustKeysScreen.Update(msg, &m)
		case screenTrustHooks, screenTrustAddHookForm, screenTrustUpdateHookForm, screenTrustRemoveHookForm:
			return m.trustHookScreen.Update(msg, &m)
		case screenPolicyRules, screenPolicyAddRule, screenPolicyEditRule:
			return m.policyRulesScreen.Update(msg, &m)
		case screenTrustGlobalRules, screenTrustAddGlobalRule, screenTrustEditGlobalRule:
			return m.trustGlobalRulesScreen.Update(msg, &m)
		case screenPolicyPrincipals:
			return m.policyPrincipalsScreen.Update(msg, &m)
		case screenPolicyPrincipalsForm:
			return m.policyPrincipalsFormScreen.Update(msg, &m)
		case screenTrustLifecycle:
			return m.trustLifecycleScreen.Update(msg, &m)
		case screenTrustPropagation, screenTrustAddPropagationForm, screenTrustUpdatePropagationForm, screenTrustRemovePropagationForm:
			return m.trustPropagationScreen.Update(msg, &m)
		case screenTrustGitHubApp, screenTrustAddGitHubAppForm, screenTrustGitHubAppActionForm:
			return m.trustGitHubAppScreen.Update(msg, &m)
		case screenTrustRepoNetwork, screenTrustRepoForm, screenTrustRepoLocationForm:
			return m.trustRepoNetworkScreen.Update(msg, &m)
		case screenVerify:
			return m.verifyScreen.Update(msg, &m)
		case screenVerifyRefForm:
			return m.verifyRefScreen.Update(msg, &m)
		case screenVerifyMergeableForm:
			return m.verifyMergeableScreen.Update(msg, &m)
		}
	}

	switch m.screen {
	case screenChoice:
		return m.homeScreen.Update(msg, &m)
	case screenHelp:
		return m.helpScreen.Update(msg, &m)
	case screenPolicy:
		return m.policyScreen.Update(msg, &m)
	case screenPolicyLifecycle, screenPolicyLifecycleForm:
		return m.policyLifecycleScreen.Update(msg, &m)
	case screenTrust:
		return m.trustScreen.Update(msg, &m)
	case screenTrustKeysThresholds, screenTrustKeyForm, screenTrustThresholdForm:
		return m.trustKeysScreen.Update(msg, &m)
	case screenTrustHooks, screenTrustAddHookForm, screenTrustUpdateHookForm, screenTrustRemoveHookForm:
		return m.trustHookScreen.Update(msg, &m)
	case screenPolicyRules, screenPolicyAddRule, screenPolicyEditRule:
		return m.policyRulesScreen.Update(msg, &m)
	case screenTrustGlobalRules, screenTrustAddGlobalRule, screenTrustEditGlobalRule:
		return m.trustGlobalRulesScreen.Update(msg, &m)
	case screenPolicyPrincipals:
		return m.policyPrincipalsScreen.Update(msg, &m)
	case screenPolicyPrincipalsForm:
		return m.policyPrincipalsFormScreen.Update(msg, &m)
	case screenTrustLifecycle:
		return m.trustLifecycleScreen.Update(msg, &m)
	case screenTrustPropagation, screenTrustAddPropagationForm, screenTrustUpdatePropagationForm, screenTrustRemovePropagationForm:
		return m.trustPropagationScreen.Update(msg, &m)
	case screenTrustGitHubApp, screenTrustAddGitHubAppForm, screenTrustGitHubAppActionForm:
		return m.trustGitHubAppScreen.Update(msg, &m)
	case screenTrustRepoNetwork, screenTrustRepoForm, screenTrustRepoLocationForm:
		return m.trustRepoNetworkScreen.Update(msg, &m)
	case screenVerify:
		return m.verifyScreen.Update(msg, &m)
	case screenVerifyRefForm:
		return m.verifyRefScreen.Update(msg, &m)
	case screenVerifyMergeableForm:
		return m.verifyMergeableScreen.Update(msg, &m)
	}

	return m, cmd
}

// Update updates the model based on the message received, ensuring list sizes are
// dynamically recalculated to fit any runtime error messages or notice overlays.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	resModel, cmd := m.updateInternal(msg)
	if typedModel, ok := resModel.(model); ok {
		typedModel.resizeLists()
		return typedModel, cmd
	}
	return resModel, cmd
}

// splitAndTrim splits a comma-separated string and trims whitespace.
func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// collectLogsCmd reads lines from scanner into a buffered channel.
// It runs as a goroutine and does not send a tea.Msg per line.
func collectLogsCmd(scanner *bufio.Scanner, ch chan<- string) tea.Cmd {
	return func() tea.Msg {
		for scanner.Scan() {
			ch <- scanner.Text()
		}
		close(ch)
		return nil
	}
}

// drainLogChannel reads all available lines from ch without blocking.
func drainLogChannel(ch <-chan string) []string {
	var lines []string
	for {
		select {
		case line, ok := <-ch:
			if !ok {
				return lines
			}
			lines = append(lines, line)
		default:
			return lines
		}
	}
}

// logFlushTick sends a logBatchMsg every 100ms by draining the log channel.
func logFlushTick(ch <-chan string) tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(_ time.Time) tea.Msg {
		return logBatchMsg{lines: drainLogChannel(ch)}
	})
}
