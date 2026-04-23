// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

type options struct{}

func (o *options) AddFlags(_ *cobra.Command) {

}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	return startTUI(cmd.Context(), o)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "setup",
		Short:             "Launch the gittuf setup wizard to quickly get started with gittuf on your repository",
		Long:              "The 'setup' command serves as an alternative to the manual setup process for gittuf, intended for rapid deployment of gittuf on repositories with a basic security policy",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}

// startTUI intitializes a new model for the TUI
func startTUI(ctx context.Context, o *options) error {
	m, err := initialModel(ctx, o)
	if err != nil {
		return err
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()

	return err
}
