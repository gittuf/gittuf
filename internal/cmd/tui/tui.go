// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
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

func (o *options) Run(_ *cobra.Command, _ []string) error {
	return startTUI(o)
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

// startTUI intitializes a new model for the TUI
func startTUI(o *options) error {
	m, err := initialModel(o)
	if err != nil {
		return err
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()

	return err
}
