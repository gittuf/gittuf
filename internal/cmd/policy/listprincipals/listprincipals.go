// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package listprincipals

import (
	"fmt"
	"strings"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/spf13/cobra"
)

const indentString = "    "

type options struct {
	policyRef  string
	policyName string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.policyRef,
		"policy-ref",
		"policy",
		"specify which policy ref should be inspected",
	)

	cmd.Flags().StringVar(
		&o.policyName,
		"policy-name",
		tuf.TargetsRoleName,
		"specify rule file to list principals for",
	)
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	principals, err := repo.ListPrincipals(cmd.Context(), o.policyRef, o.policyName)
	if err != nil {
		return err
	}

	for _, principal := range principals {
		fmt.Printf("Principal %s:\n", principal.ID())

		fmt.Printf(indentString + "Keys:\n")
		for _, key := range principal.Keys() {
			fmt.Printf(strings.Repeat(indentString, 2)+"%s (%s)\n", key.KeyID, key.KeyType)
		}

		customMetadata := principal.CustomMetadata()
		if len(customMetadata) > 0 {
			fmt.Printf(indentString + "Custom Metadata:\n")
			for key, value := range principal.CustomMetadata() {
				fmt.Printf(strings.Repeat(indentString, 2)+"%s: %s\n", key, value)
			}
		}
	}
	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "list-principals",
		Short:             "List principals for the current policy in the specified rule file",
		Long:              `The 'list-principals' command lists all trusted principals defined in a gittuf policy rule file. By default, the main policy file (targets) is used, which can be overridden with the '--policy-name' flag.`,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
