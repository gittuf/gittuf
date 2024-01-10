// SPDX-License-Identifier: Apache-2.0

package verifycommit

import (
	"fmt"

	"github.com/gittuf/gittuf/internal/repository"
	"github.com/spf13/cobra"
)

type options struct {
	applyFilePolicies bool
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(
		&o.applyFilePolicies,
		"apply-file-policies",
		"a",
		false,
		"check if the file paths modified by the commit are allowed by the policy",
	)
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	status := repo.VerifyCommit(cmd.Context(), o.applyFilePolicies, args...)

	for _, id := range args {
		fmt.Printf("%s: %s\n", id, status[id])
	}

	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "verify-commit",
		Short:             "Verify commit signatures using gittuf metadata",
		Args:              cobra.MinimumNArgs(1),
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
