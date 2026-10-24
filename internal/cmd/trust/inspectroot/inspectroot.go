// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package inspectroot

import (
	"encoding/json"
	"fmt"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/policy"
	policyopts "github.com/gittuf/gittuf/internal/policy/options/policy"
	"github.com/gittuf/gittuf/pkg/githash"
	"github.com/spf13/cobra"
)

type options struct {
	revision string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.revision,
		"revision",
		"",
		"policy revision to inspect (commit ID, short SHA, or ref name; defaults to the current state)",
	)
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	var state *policy.State
	if o.revision != "" {
		var commitID githash.Hash
		commitID, err = common.ResolveRevision(repo.GetStorer(), o.revision)
		if err != nil {
			return err
		}

		state, err = policy.LoadStateFromCommit(repo.GetStorer(), commitID)
	} else {
		state, err = policy.LoadCurrentState(cmd.Context(), repo.GetStorer(), policy.PolicyStagingRef, policyopts.BypassRSL())
	}
	if err != nil {
		return err
	}

	rootMetadata, err := state.GetRootMetadata(false)
	if err != nil {
		return err
	}

	prettyJSON, err := json.MarshalIndent(rootMetadata, "", "  ")
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(prettyJSON))
	return err
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "inspect-root",
		Short:             "Inspect root metadata",
		Long:              "This command displays the root metadata in a human-readable format. By default, the current state is shown; use --revision to inspect the metadata as it was recorded in a specific policy revision (commit ID, short SHA, or ref name).",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
