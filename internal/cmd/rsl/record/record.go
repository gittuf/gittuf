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
	remoteName         string
	localOnly          bool
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

	opts := []rslopts.RecordOption{
		rslopts.WithOverrideRefName(o.dstRef),
		rslopts.WithRecordRemote(o.remoteName),
	}
	if o.skipDuplicateCheck {
		opts = append(opts, rslopts.WithSkipCheckForDuplicateEntry())
	}
	if o.localOnly {
		opts = append(opts, rslopts.WithRecordLocalOnly())
	}

	return repo.RecordRSLEntryForReference(cmd.Context(), args[0], true, opts...)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "record",
		Short: "Record latest state of a Git reference in the RSL",
		Long: `The 'record' command adds a new entry to the Repository Signing Log (RSL) reflecting the latest state of a Git reference.
It can override the reference name, skip duplicate checks, and choose to record locally or push to a remote.
This helps keep the RSL up-to-date with important repository state changes.`,

		Args:              cobra.ExactArgs(1),
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
