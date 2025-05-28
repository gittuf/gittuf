// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package apply

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/spf13/cobra"
)

type options struct {
	localOnly bool
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(
		&o.localOnly,
		"local-only",
		false,
		"indicate that the attestation must be committed into the RSL only locally",
	)
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	remoteName := ""
	if len(args) > 0 {
		remoteName = args[0]
	}

	return repo.ApplyAttestations(cmd.Context(), remoteName, o.localOnly, true)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply and push local attestations changes to remote repository",
		Long: `The 'apply' command takes all locally recorded attestations (stored in the
Reference State Log, or RSL) and applies them to the target repository.

By default, the command pushes those attestations to the remote specified
in your Git configuration.  Pass '--local-only' to record the attestation
locally without pushing upstream.

Optionally, you may supply the remote name as the first positional argument.
If omitted, the default remote is used.`,
		RunE: o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
