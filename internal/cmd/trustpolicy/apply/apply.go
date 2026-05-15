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
		"indicate that the policy must be committed into the RSL locally",
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

	return repo.ApplyPolicy(cmd.Context(), remoteName, o.localOnly, true)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Validate and apply the staged policy proposal",
		Long:  "The 'apply' command validates and promotes the proposed policy from policy-staging to the live policy. It is used to make a staged proposal effective. Apply is non-selective — it always promotes the full staged proposal; use `gittuf policy stage --policy-name` beforehand to control which target envelopes are part of the proposal.",
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
