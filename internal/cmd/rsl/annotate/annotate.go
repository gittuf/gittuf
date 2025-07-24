// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package annotate

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	rslopts "github.com/gittuf/gittuf/experimental/gittuf/options/rsl"
	"github.com/spf13/cobra"
)

type options struct {
	skip       bool
	message    string
	remoteName string
	localOnly  bool
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

	cmd.Flags().StringVar(
		&o.remoteName,
		"remote-name",
		"",
		"remote name",
	)

	cmd.Flags().BoolVar(
		&o.localOnly,
		"local-only",
		false,
		"local only",
	)

	cmd.MarkFlagsOneRequired("remote-name", "local-only")
	cmd.MarkFlagsMutuallyExclusive("remote-name", "local-only")
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	opts := []rslopts.AnnotateOption{rslopts.WithAnnotateRemote(o.remoteName)}
	if o.localOnly {
		opts = append(opts, rslopts.WithAnnotateLocalOnly())
	}

	return repo.RecordRSLAnnotation(cmd.Context(), args, o.skip, o.message, true, opts...)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "annotate",
		Short: "Annotate prior RSL entries",
		Long: `The 'annotate' command adds annotations to prior entries in the Repository State Log (RSL).
These annotations can mark entries to be skipped or add descriptive messages for auditing and review purposes.
The command supports annotating entries locally or on a remote repository.`,

		Args:              cobra.MinimumNArgs(1),
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
