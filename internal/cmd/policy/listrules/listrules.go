// SPDX-License-Identifier: Apache-2.0

package listrules

import (
	"fmt"
	"strings"

	"github.com/gittuf/gittuf/internal/repository"
	"github.com/spf13/cobra"
)

type options struct{}

func (o *options) AddFlags(_ *cobra.Command) {}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	rules, err := repo.ListRules(cmd.Context())
	if err != nil {
		return err
	}

	// Iterate through the rules, they are already in order, and the depth tells us how to indent.
	// The order is a pre-order traversal of the delegation tree, so that the parent is always before the children.

	for _, curRule := range rules {
		fmt.Printf(strings.Repeat("    ", curRule.Depth)+"Rule %s:\n", curRule.Delegation.Name)
		gitpaths, filepaths := []string{}, []string{}
		for _, path := range curRule.Delegation.Paths {
			if strings.HasPrefix(path, "git:") {
				gitpaths = append(gitpaths, path)
			} else {
				filepaths = append(filepaths, path)
			}
		}
		if len(filepaths) > 0 {
			fmt.Println(strings.Repeat("    ", curRule.Depth+1) + "Paths affected:")
			for _, v := range filepaths {
				fmt.Printf(strings.Repeat("    ", curRule.Depth+2)+"%s\n", v)
			}
		}
		if len(gitpaths) > 0 {
			fmt.Println(strings.Repeat("    ", curRule.Depth+1) + "Refs affected:")
			for _, v := range gitpaths {
				fmt.Printf(strings.Repeat("    ", curRule.Depth+2)+"%s\n", v)
			}
		}

		fmt.Println(strings.Repeat("    ", curRule.Depth+1) + "Authorized keys:")
		for _, key := range curRule.Delegation.Role.KeyIDs {
			fmt.Printf(strings.Repeat("    ", curRule.Depth+2)+"%s\n", key)
		}
	}
	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "list-rules",
		Short:             "List rules for the current state",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
