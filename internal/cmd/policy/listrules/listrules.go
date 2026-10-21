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

	stdOut := cmd.OutOrStdout()

	if len(rules) == 0 {
		if _, err := fmt.Fprintln(stdOut, "No rules are currently defined."); err != nil {
			return err
		}
		return nil
	}

	// Iterate through the rules, they are already in order, and the depth tells us how to indent.
	// The order is a pre-order traversal of the delegation tree, so that the parent is always before the children.

	for i, curRule := range rules {
		if _, err := fmt.Fprintf(stdOut, strings.Repeat("    ", curRule.Depth)+"Rule %s:\n", curRule.Delegation.ID()); err != nil {
			return err
		}
		gitpaths, filepaths := []string{}, []string{}
		for _, path := range curRule.Delegation.GetProtectedNamespaces() {
			if strings.HasPrefix(path, "git:") {
				gitpaths = append(gitpaths, path)
			} else {
				filepaths = append(filepaths, path)
			}
		}
		if len(filepaths) > 0 {
			if _, err := fmt.Fprintln(stdOut, strings.Repeat("    ", curRule.Depth+1)+"Paths affected:"); err != nil {
				return err
			}
			for _, v := range filepaths {
				if _, err := fmt.Fprintf(stdOut, strings.Repeat("    ", curRule.Depth+2)+"%s\n", v); err != nil {
					return err
				}
			}
		}
		if len(gitpaths) > 0 {
			if _, err := fmt.Fprintln(stdOut, strings.Repeat("    ", curRule.Depth+1)+"Refs affected:"); err != nil {
				return err
			}
			for _, v := range gitpaths {
				if _, err := fmt.Fprintf(stdOut, strings.Repeat("    ", curRule.Depth+2)+"%s\n", v); err != nil {
					return err
				}
			}
		}

		if _, err := fmt.Fprintln(stdOut, strings.Repeat("    ", curRule.Depth+1)+"Authorized keys:"); err != nil {
			return err
		}
		for _, key := range curRule.Delegation.GetPrincipalIDs().Contents() {
			if _, err := fmt.Fprintf(stdOut, strings.Repeat("    ", curRule.Depth+2)+"%s\n", key); err != nil {
				return err
			}
		}

		if _, err := fmt.Fprintln(stdOut, strings.Repeat("    ", curRule.Depth+1)+fmt.Sprintf("Required valid signatures: %d", curRule.Delegation.GetThreshold())); err != nil {
			return err
		}
		if i < len(rules)-1 {
			if _, err := fmt.Fprintln(stdOut); err != nil {
				return err
			}
		}
	}
	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "list-rules",
		Short:             "List rules for the current state",
		Long:              "The 'list-rules' command lists the rules defined in a gittuf policy, shown as a tree in evaluation order. It is used to review which principals are authorized over which namespaces.",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
