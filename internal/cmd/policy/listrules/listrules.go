// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package listrules

import (
	"fmt"
	"strings"

	"github.com/gittuf/gittuf/experimental/gittuf"
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

	rules, err := repo.ListRules(cmd.Context(), o.targetRef)
	if err != nil {
		return err
	}

	// Iterate through the rules, they are already in order, and the depth tells us how to indent.
	// The order is a pre-order traversal of the delegation tree, so that the parent is always before the children.

	for _, curRule := range rules {
		fmt.Printf(strings.Repeat("    ", curRule.Depth)+"Rule %s:\n", curRule.Delegation.ID())
		gitpaths, filepaths := []string{}, []string{}
		for _, path := range curRule.Delegation.GetProtectedNamespaces() {
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
		for _, key := range curRule.Delegation.GetPrincipalIDs().Contents() {
			fmt.Printf(strings.Repeat("    ", curRule.Depth+2)+"%s\n", key)
		}

		fmt.Println(strings.Repeat("    ", curRule.Depth+1) + fmt.Sprintf("Required valid signatures: %d", curRule.Delegation.GetThreshold()))
	}
	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
<<<<<<< HEAD
		Use:               "list-rules",
		Short:             "List rules for the current state",
		Long:              `The 'list-rules' command displays all policy rules defined in the current gittuf policy. By default, the main policy file (targets) is used, which can be overridden with the '--policy-name' flag.`,
=======
		Use:   "list-rules",
		Short: "List rules for the current state",
		Long: `List all policy rules in the current repository at the given Git reference.

This command displays rules defined by your repository's policy in a clear, hierarchical format.
It performs a pre-order traversal of the delegation tree so that parent rules appear before their children,
indenting sub-delegations accordingly.

For each rule, the output includes:
  • Rule ID
  • Paths affected (file/directory)
  • Git refs affected
  • Authorized principal IDs (keys)
  • Required signature threshold (number of valid signatures required)

This helps you visually inspect access control hierarchy and verify which principals are authorized to sign
changes under each rule.

Flags:
  • --target-ref string   Git reference where the policy is stored (default: "policy")

Example:
  # List rules defined in the 'main' branch
  gittuf list-rules --target-ref main

  # Use this in CI to inspect rules in the current directory
  gittuf list-rules
`,

>>>>>>> 8380fc8 (docs: enhance list-rules Long description)
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
