// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package stage

import (
	"fmt"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/spf13/cobra"
)

type options struct {
	localOnly bool
	remote    string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(
		&o.localOnly,
		"local-only",
		false,
		"indicate that the policy must be committed into the RSL locally",
	)
	cmd.Flags().StringVar(
		&o.remote,
		"remote",
		"",
		"remote to push the staged proposal to (ignored when --local-only is set)",
	)
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	selectedTargets, err := parseTargets(args)
	if err != nil {
		return err
	}

	return repo.StagePolicy(cmd.Context(), o.remote, selectedTargets, o.localOnly, true)
}

// parseTargets validates the positional arguments and returns the slice to
// pass to StagePolicy. Mixing the stage-all sentinel with explicit names is
// rejected because the intent is ambiguous; otherwise args is passed through
// unchanged (so the sentinel reaches StagePolicy verbatim).
func parseTargets(args []string) ([]string, error) {
	for _, a := range args {
		if a == gittuf.StageAllSentinel {
			if len(args) != 1 {
				return nil, fmt.Errorf("'%s' cannot be combined with policy names; pass either '%s' to stage everything, or one or more specific policy names", gittuf.StageAllSentinel, gittuf.StageAllSentinel)
			}
			return args, nil
		}
	}

	return args, nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "stage (<policy-name>... | .)",
		Short: "Stage local policy changes for proposal",
		Long:  "The 'stage' command promotes policy changes from policy-index (the local scratchpad of pending mutations) into policy-staging (the official proposal). Pass one or more policy target names to selectively stage just those envelopes, or pass '.' to promote the entire policy-index tip. Stage records an RSL entry and (unless --local-only) pushes to the remote specified by --remote. The proposed policy can then be reviewed, co-signed, and finally applied with `gittuf policy apply`.",
		Args:  cobra.MinimumNArgs(1),
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
