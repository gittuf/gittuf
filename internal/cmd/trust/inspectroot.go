// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package trust

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/policy"
	policyopts "github.com/gittuf/gittuf/internal/policy/options/policy"
	"github.com/spf13/cobra"
)

type inspectRootOptions struct{}

func (o *inspectRootOptions) AddFlags(cmd *cobra.Command) {}

func (o *inspectRootOptions) Run(cmd *cobra.Command, args []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	state, err := policy.LoadCurrentState(context.Background(), repo.GetGitRepository(), policy.PolicyRef, policyopts.BypassRSL())
	if err != nil {
		return err
	}

	rootMetadata, err := state.GetRootMetadata(false)
	if err != nil {
		return err
	}

	// Marshal the root metadata to pretty-printed JSON
	prettyJSON, err := json.MarshalIndent(rootMetadata, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(prettyJSON))
	return nil
}

func NewInspectRootCommand() *cobra.Command {
	o := &inspectRootOptions{}
	cmd := &cobra.Command{
		Use:   "inspect-root",
		Short: "Inspect and print all gittuf root metadata",
		RunE:  o.Run,
	}
	o.AddFlags(cmd)
	return cmd
}
