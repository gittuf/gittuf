// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/spf13/cobra"
)

type options struct {
	p          *persistent.Options
	policyName string
	targetRef  string
	readOnly   bool
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
		"name of policy file to make changes to",
	)

	cmd.Flags().BoolVar(
		&o.readOnly,
		"read-only",
		false,
		"interact with the TUI in read-only mode",
	)
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	return startTUI(cmd.Context(), o)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "tui",
		Short:             "Start the TUI for gittuf",
		Long:              "This command starts a terminal-based interface to view and/or manage gittuf metadata. If a signing key is provided, mutating operations are enabled and signed. Without a signing key, the TUI runs in read-only mode. Changes to the policy files in the TUI are staged immediately without further confirmation and users are required to run `gittuf policy apply` to commit the changes.",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	persistent.AddPersistentFlags(cmd)
	o.AddFlags(cmd)

	return cmd
}

// loadedMsg is sent when the actual model has finished loading.
type loadedMsg struct {
	model tea.Model
	err   error
}

// loadingModel wraps a spinner shown while the real model loads.
type loadingModel struct {
	spinner spinner.Model
	ctx     context.Context
	opts    *options
	loaded  tea.Model
	err     error
	done    bool
}

func newLoadingModel(ctx context.Context, o *options) loadingModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return loadingModel{
		spinner: s,
		ctx:     ctx,
		opts:    o,
	}
}

func (m loadingModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			model, err := initialModel(m.ctx, m.opts)
			return loadedMsg{model: model, err: err}
		},
	)
}

func (m loadingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case loadedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.done = true
			return m, tea.Quit
		}
		// Switch to the real model and initialize it
		return msg.model, msg.model.Init()
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m loadingModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error loading: %v\n", m.err)
	}
	return fmt.Sprintf("%s Loading gittuf policy data...\n", m.spinner.View())
}

// startTUI initializes a new model for the TUI
func startTUI(ctx context.Context, o *options) error {
	m := newLoadingModel(ctx, o)

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	// Check if the loading model encountered an error
	if lm, ok := finalModel.(loadingModel); ok && lm.err != nil {
		return lm.err
	}

	return nil
}
