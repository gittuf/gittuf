// SPDX-License-Identifier: Apache-2.0

package record

import (
	"fmt"
	"log/slog"

	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/repository"
	"github.com/spf13/cobra"
)

type options struct {
	commitID string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(
		&o.commitID,
		"commit",
		"c",
		"",
		fmt.Sprintf("commit ID (eval mode only, set %s=1)", common.EvalModeKey),
	)
}

func (o *options) Run(_ *cobra.Command, args []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	if len(o.commitID) > 0 {
		if !common.EvalMode() {
			return common.ErrNotInEvalMode
		}

		err = repo.RecordRSLEntryForReferenceAtCommit(args[0], o.commitID, true)
		if err != nil {
			return err
		}
		slog.Info("Added the RSL entry for Git reference", "commitID", o.commitID)

		return nil
	}

	err = repo.RecordRSLEntryForReference(args[0], true)
	if err != nil {
		return err
	}
	slog.Info("Added the RSL entry for Git reference", "refName", args[0])

	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:     "record",
		Short:   "Record latest state of a Git reference in the RSL",
		Args:    cobra.ExactArgs(1),
		PreRunE: common.CheckIfSigningViable,
		RunE:    o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
