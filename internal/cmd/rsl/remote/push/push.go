// SPDX-License-Identifier: Apache-2.0

package push

import (
	"github.com/gittuf/gittuf/internal/repository"
	"github.com/spf13/cobra"
	"log/slog"
)

type options struct {
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	err = repo.PushRSL(cmd.Context(), args[0])
	if err != nil {
		return err
	}
	slog.Info("RSL pushed to", "remote", args[0])

	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "push <remote>",
		Short: "Push RSL to the specified remote",
		Args:  cobra.ExactArgs(1),
		RunE:  o.Run,
	}

	return cmd
}
