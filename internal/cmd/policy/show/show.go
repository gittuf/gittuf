// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package show

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
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	fmt.Println("Principals:")
	principals, err := repo.ListPrincipals(cmd.Context(), o.policyRef, o.policyName)
	if err != nil {
		return err
	}

	count := 0
	for _, principal := range principals {
		fmt.Printf(indentString+"%s:\n", principal.ID())

		fmt.Println(strings.Repeat(indentString, 2) + "Keys:")
		for _, key := range principal.Keys() {
			fmt.Printf(strings.Repeat(indentString, 3)+"%s (%s)\n", key.KeyID, key.KeyType)
		}

		customMetadata := principal.CustomMetadata()
		if len(customMetadata) > 0 {
			fmt.Println(strings.Repeat(indentString, 2) + "Custom Metadata:")
			for key, value := range principal.CustomMetadata() {
				fmt.Printf(strings.Repeat(indentString, 3)+"%s: %s\n", key, value)
			}
		}
		if count < len(principals)-1 {
			fmt.Println()
		}
		count++
	}

	fmt.Println("Rules:")
	rules, err := repo.ListRules(cmd.Context(), o.policyRef)
	if err != nil {
		return err
	}

	// Iterate through the rules, they are already in order, and the depth tells us how to indent.
	// The order is a pre-order traversal of the delegation tree, so that the parent is always before the children.

	for i, curRule := range rules {
		fmt.Printf(strings.Repeat(indentString, curRule.Depth+1)+"Rule %s:\n", curRule.Delegation.ID())
		gitpaths, filepaths := []string{}, []string{}
		for _, path := range curRule.Delegation.GetProtectedNamespaces() {
			if strings.HasPrefix(path, "git:") {
				gitpaths = append(gitpaths, path)
			} else {
				filepaths = append(filepaths, path)
			}
		}
		if len(filepaths) > 0 {
			fmt.Println(strings.Repeat(indentString, curRule.Depth+2) + "Paths affected:")
			for _, v := range filepaths {
				fmt.Printf(strings.Repeat(indentString, curRule.Depth+3)+"%s\n", v)
			}
		}
		if len(gitpaths) > 0 {
			fmt.Println(strings.Repeat(indentString, curRule.Depth+2) + "Refs affected:")
			for _, v := range gitpaths {
				fmt.Printf(strings.Repeat(indentString, curRule.Depth+3)+"%s\n", v)
			}
		}

		fmt.Println(strings.Repeat(indentString, curRule.Depth+2) + "Authorized keys:")
		for _, key := range curRule.Delegation.GetPrincipalIDs().Contents() {
			fmt.Printf(strings.Repeat(indentString, curRule.Depth+3)+"%s\n", key)
		}

		fmt.Println(strings.Repeat(indentString, curRule.Depth+2) + fmt.Sprintf("Required valid signatures: %d", curRule.Delegation.GetThreshold()))
		if i < len(rules)-1 {
			fmt.Println()
		}
	}

	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "show",
		Short:             "Show policy metadata",
		Long:              "This command displays gittuf's policy metadata for the specified policy file.",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
