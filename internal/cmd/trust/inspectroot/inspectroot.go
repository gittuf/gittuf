// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package inspectroot

import (
	"encoding/json"
	"fmt"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/policy"
	policyopts "github.com/gittuf/gittuf/internal/policy/options/policy"
	"github.com/spf13/cobra"
)

type options struct {
	policyRef string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.policyRef,
		"policy-ref",
		policy.PolicyStagingRef,
		"policy reference to inspect",
	)
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	state, err := policy.LoadCurrentState(cmd.Context(), repo.GetGitRepository(), o.policyRef, policyopts.BypassRSL())
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

	fmt.Println(string(prettyJSON))
	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "inspect-root",
		Short:             "Inspect root metadata",
		Long:              "This command displays the root metadata in a human-readable format. By default, it inspects the policy-staging ref, but you can specify a different policy ref using --policy-ref.",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
