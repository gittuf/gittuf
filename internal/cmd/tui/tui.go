// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
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
		Long:              "The 'tui' command starts a terminal-based interface to view and manage gittuf policy metadata. It is used to inspect or modify policy files interactively. A signing key must be provided to enable write operations; without one the TUI runs in read-only mode. Changes made in the TUI are staged immediately and require running 'gittuf policy apply' to take effect.",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	persistent.AddPersistentFlags(cmd)
	o.AddFlags(cmd)

	return cmd
}

// startTUI initializes a new model for the TUI
func startTUI(ctx context.Context, o *options) error {
	m := initialModel(ctx, o)

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()

	return err
}
