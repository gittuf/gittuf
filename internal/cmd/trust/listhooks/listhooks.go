// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package listhooks

import (
	"fmt"
	"strings"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/spf13/cobra"
)

const indentString = "    "

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

	hookStages, err := repo.ListHooks(cmd.Context(), o.targetRef)
	if err != nil {
		return err
	}

	for stage, data := range hookStages {
		fmt.Printf("Stage %s:\n", stage.String())
		for _, hook := range data {
			fmt.Printf(indentString+"Hook '%s':\n", hook.ID())

			fmt.Printf("%sPrincipal IDs:\n", strings.Repeat(indentString, 2))
			for _, id := range hook.GetPrincipalIDs().Contents() {
				fmt.Printf("%s%s\n", strings.Repeat(indentString, 3), id)
			}

			fmt.Printf("%sHashes:\n", strings.Repeat(indentString, 2))
			for algo, hash := range hook.GetHashes() {
				fmt.Printf("%s%s: %s\n", strings.Repeat(indentString, 3), algo, hash)
			}

			fmt.Printf("%sEnvironment:\n", strings.Repeat(indentString, 2))
			fmt.Printf("%s%s\n", strings.Repeat(indentString, 3), hook.GetEnvironment().String())

			fmt.Printf("%sTimeout:\n", strings.Repeat(indentString, 2))
			fmt.Printf("%s%d\n", strings.Repeat(indentString, 3), hook.GetTimeout())
		}
		fmt.Println()
	}

	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "list-hooks",
		Short:             "List gittuf hooks for the current policy state",
		Long:              "The 'list-hooks' command displays all gittuf hooks defined in the current repository's gittuf policy, including each hook's stage, ID, authorized principals, hashes, environment, and timeout.",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
