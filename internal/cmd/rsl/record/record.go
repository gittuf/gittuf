// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package record

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	rslopts "github.com/gittuf/gittuf/experimental/gittuf/options/rsl"
	"github.com/spf13/cobra"
)

type options struct {
	dstRef             string
	skipDuplicateCheck bool
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.dstRef,
		"dst-ref",
		"",
		"name of destination reference, if it differs from source reference",
	)

	cmd.Flags().BoolVar(
		&o.skipDuplicateCheck,
		"skip-duplicate-check",
		false,
		"skip check to see if latest entry for reference has same target",
	)
}

func (o *options) Run(_ *cobra.Command, args []string) error {
	repo, err := gittuf.LoadRepository()
	if err != nil {
		return err
	}

	opts := []rslopts.Option{rslopts.WithOverrideRefName(o.dstRef)}
	if o.skipDuplicateCheck {
		opts = append(opts, rslopts.WithSkipCheckForDuplicateEntry())
	}

	return repo.RecordRSLEntryForReference(args[0], true, opts...)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "record",
		Short:             "Record latest state of a Git reference in the RSL",
		Args:              cobra.ExactArgs(1),
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
