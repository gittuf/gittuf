// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package inspect

import (
	"encoding/json"
	"fmt"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/policy"
	policyopts "github.com/gittuf/gittuf/internal/policy/options/policy"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/spf13/cobra"
)

type options struct {
	policyName string
	revision   string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.policyName,
		"policy-name",
		policy.TargetsRoleName,
		"name of policy file to inspect",
	)

	cmd.Flags().StringVar(
		&o.revision,
		"revision",
		"",
		"commit ID of the gittuf policy-staging ref to inspect (defaults to the current state)",
	)
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	var state *policy.State
	if o.revision != "" {
		var commitID gitinterface.Hash
		commitID, err = gitinterface.NewHash(o.revision)
		if err != nil {
			return err
		}

		state, err = policy.LoadStateFromCommit(repo.GetGitRepository(), commitID)
	} else {
		state, err = policy.LoadCurrentState(cmd.Context(), repo.GetGitRepository(), policy.PolicyStagingRef, policyopts.BypassRSL())
	}
	if err != nil {
		return err
	}

	targetsMetadata, err := state.GetTargetsMetadata(o.policyName, false)
	if err != nil {
		return err
	}

	prettyJSON, err := json.MarshalIndent(targetsMetadata, "", "  ")
	if err != nil {
		return err
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(prettyJSON))
	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "inspect",
		Short:             "Inspect policy metadata",
		Long:              "This command displays a gittuf policy (rule) file's metadata in a human-readable format. Use --policy-name to select which policy file to display (defaults to the primary 'targets' file), and --revision to inspect the metadata as it was recorded in a specific policy commit.",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
