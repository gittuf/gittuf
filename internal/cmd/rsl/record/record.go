// SPDX-License-Identifier: Apache-2.0

package record

import (
	"fmt"
	"os"

	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/eval"
	"github.com/gittuf/gittuf/internal/repository"
	"github.com/spf13/cobra"
)

const (
	commitFlagName = "commit"
)

type options struct {
	commitID       string
	signingKeyPath string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(
		&o.commitID,
		commitFlagName,
		"c",
		"",
		fmt.Sprintf("commit ID (eval mode only, set %s=1)", eval.EvalModeKey),
	)

	cmd.Flags().StringVarP(
		&o.signingKeyPath,
		"signing-key",
		"k",
		"",
		fmt.Sprintf("path to PEM encoded SSH or GPG signing key (eval mode only, set %s=1)", eval.EvalModeKey),
	)

	cmd.MarkFlagsRequiredTogether("commit", "signing-key")
}

func (o *options) Run(_ *cobra.Command, args []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	if len(o.commitID) > 0 {
		if !eval.InEvalMode() {
			return eval.ErrNotInEvalMode
		}

		signingKeyBytes, err := os.ReadFile(o.signingKeyPath)
		if err != nil {
			return err
		}

		return repo.RecordRSLEntryForReferenceAtCommit(args[0], o.commitID, signingKeyBytes)
	}

	return repo.RecordRSLEntryForReference(args[0], true)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:     "record",
		Short:   "Record latest state of a Git reference in the RSL",
		Args:    cobra.ExactArgs(1),
		PreRunE: checkIfSigningViable,
		RunE:    o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}

func checkIfSigningViable(cmd *cobra.Command, args []string) error {
	commitFlag := cmd.Flag(commitFlagName)
	if commitFlag.Value.String() != "" {
		return nil
	}

	return common.CheckIfSigningViable(cmd, args)
}
