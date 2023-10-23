// SPDX-License-Identifier: Apache-2.0

package record

import (
	"fmt"

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

		return repo.RecordRSLEntryForReferenceAtCommit(args[0], o.commitID, true)
	}

	return repo.RecordRSLEntryForReference(args[0], true)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "record",
		Short: "Record latest state of a Git reference in the RSL",
		Args:  cobra.ExactArgs(1),
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
