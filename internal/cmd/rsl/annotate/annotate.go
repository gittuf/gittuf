// SPDX-License-Identifier: Apache-2.0

package annotate

import (
	repository "github.com/gittuf/gittuf/gittuf"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/spf13/cobra"
)

type options struct {
	skip    bool
	message string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(
		&o.skip,
		"skip",
		"s",
		false,
		"mark annotated entries as to be skipped",
	)

	cmd.Flags().StringVarP(
		&o.message,
		"message",
		"m",
		"",
		"annotation message",
	)
	cmd.MarkFlagRequired("message") //nolint:errcheck
}

func (o *options) Run(_ *cobra.Command, args []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	return repo.RecordRSLAnnotation(args, o.skip, o.message, true)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "annotate",
		Short:             "Annotate prior RSL entries",
		Args:              cobra.MinimumNArgs(1),
		PreRunE:           common.CheckIfSigningViable,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
