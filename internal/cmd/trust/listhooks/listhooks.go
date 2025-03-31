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
	repo, err := gittuf.LoadRepository()
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

			fmt.Printf(strings.Repeat(indentString, 2) + "Principal IDs:\n")
			for _, id := range hook.GetPrincipalIDs().Contents() {
				fmt.Printf(strings.Repeat(indentString, 3)+"%s\n", id)
			}

			fmt.Printf(strings.Repeat(indentString, 2) + "Hashes:\n")
			for algo, hash := range hook.GetHashes() {
				fmt.Printf(strings.Repeat(indentString, 3)+"%s: %s\n", algo, hash)
			}

			fmt.Printf(strings.Repeat(indentString, 2) + "Environment:\n")
			fmt.Printf(strings.Repeat(indentString, 3)+"%s\n", hook.GetEnvironment().String())

			if hook.GetEnvironment() == tuf.HookEnvironmentLua {
				fmt.Printf(strings.Repeat(indentString, 2) + "Lua modules:\n")
				for _, module := range hook.GetModules() {
					fmt.Printf(strings.Repeat(indentString, 3)+"%s\n", module)
				}
			}
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
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
