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
	targetRef string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.targetRef,
		"target-ref",
		"policy",
		"specify which policy ref should be inspected",
	)
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	state, err := policy.LoadCurrentState(cmd.Context(), repo.GetGitRepository(), o.targetRef, policyopts.BypassRSL())
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
		Short:             "Print root metadata in JSON format",
		Long:              "This command prints the root metadata in a pretty-printed JSON format. By default, it inspects the policy ref, but you can specify a different policy ref using --target-ref.",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
