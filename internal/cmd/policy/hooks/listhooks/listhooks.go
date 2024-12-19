// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package listhooks

import (
	"fmt"
	"strings"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/spf13/cobra"
)

const indentString = "    "

type options struct {
	targetRef  string
	policyName string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.targetRef,
		"target-ref",
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
	repo, err := gittuf.LoadRepository()
	if err != nil {
		return err
	}

	hookStages, err := repo.ListHooks(cmd.Context(), o.targetRef, o.policyName)
	if err != nil {
		return err
	}

	for _, stage := range hookStages {
		fmt.Printf("Stage %s:\n", stage)
		for _, hook := range stage {
			fmt.Printf("Hook '%s':\n", hook.ID())

			fmt.Printf(indentString + "Principal IDs:\n")
			for _, id := range hook.GetPrincipalIDs().Contents() {
				fmt.Printf(strings.Repeat(indentString, 2)+"%s (%s)\n", id)
			}

			fmt.Printf(indentString + "Hashes:\n")
			for algo, hash := range hook.GetHashes() {
				fmt.Printf(strings.Repeat(indentString, 2)+"%s: %s\n", algo, hash)
			}

			fmt.Printf(indentString + "Environment:\n")
			fmt.Printf(strings.Repeat(indentString, 2)+"%s", hook.GetEnvironment())
		}
	}

	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "list-hooks",
		Short:             "List hooks for the current policy in the specified rule file",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
