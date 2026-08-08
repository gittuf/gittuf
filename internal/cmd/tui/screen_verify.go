// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"bufio"
	"io"
	"log/slog"
	"os"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type verifyScreen struct {
	choiceList list.Model
}

func (s *verifyScreen) Update(msg tea.Msg, m *model) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.String() == "enter" {
			if sel, ok := s.choiceList.SelectedItem().(item); ok {
				switch sel.title {
				case "Verify Reference":
					m.screen = screenVerifyRefForm
					m.verifyRefScreen.reset()
				case "Verify Mergeability":
					m.screen = screenVerifyMergeableForm
					m.verifyMergeableScreen.reset()
				case "Verify Network":
					m.verifying = true
					m.logs = ""
					m.logViewport.SetContent("")
					m.loadingMsg = "Verifying network repositories..."

					pr, pw := io.Pipe()
					logger := slog.New(slog.NewTextHandler(pw, &slog.HandlerOptions{Level: slog.LevelDebug}))
					slog.SetDefault(logger)

					scanner := bufio.NewScanner(pr)

					return *m, tea.Batch(
						m.spinner.Tick,
						waitForLog(scanner),
						func() tea.Msg {
							defer pw.Close()
							err := m.repo.VerifyNetwork(m.ctx)

							slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

							return verifyResultMsg{err: err, successMsg: "Network verification successful!"}
						},
					)
				}
			}
			return *m, nil
		}
	}

	s.choiceList, cmd = s.choiceList.Update(msg)
	return *m, cmd
}

func (s *verifyScreen) View(m *model) string {
	return m.renderScreen("Home › Verify", s.choiceList.View(), renderActionHints(m.readOnly))
}
