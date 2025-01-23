// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package annotate

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
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

func (o *options) Run(cmd *cobra.Command, args []string) error {
	repo, err := gittuf.LoadRepository()
	if err != nil {
		return err
	}

	return repo.RecordRSLAnnotation(cmd.Context(), args, o.skip, o.message, true)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "annotate",
		Short:             "Annotate prior RSL entries",
		Args:              cobra.MinimumNArgs(1),
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
