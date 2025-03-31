// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package listglobalrules

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

	rules, err := repo.ListGlobalRules(cmd.Context(), o.targetRef)
	if len(rules) == 0 {
		fmt.Println("No global rules are currently defined.")
	}
	if err != nil {
		return err
	}

	thresholdRules := []tuf.GlobalRuleThreshold{}
	blockForcePushesRules := []tuf.GlobalRuleBlockForcePushes{}
	for _, curRule := range rules {
		switch globalRule := curRule.(type) {
		case tuf.GlobalRuleThreshold:
			thresholdRules = append(thresholdRules, globalRule)
		case tuf.GlobalRuleBlockForcePushes:
			blockForcePushesRules = append(blockForcePushesRules, globalRule)
		}
	}

	for _, curRule := range thresholdRules {
		fmt.Printf("Global Rule: %v\n", curRule.GetName())
		fmt.Println(indentString + "Type: " + tuf.GlobalRuleThresholdType)
		printNamespaces(curRule.GetProtectedNamespaces())
		fmt.Printf(indentString+"Threshold: %d\n", curRule.GetThreshold())
	}

	for _, curRule := range blockForcePushesRules {
		fmt.Printf("Global Rule: %v\n", curRule.GetName())
		fmt.Println(indentString + "Type: " + tuf.GlobalRuleBlockForcePushesType)
		printNamespaces(curRule.GetProtectedNamespaces())
	}

	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "list-global-rules",
		Short:             "List global rules for the current state",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}

func printNamespaces(namespaces []string) {
	gitpaths, filepaths := []string{}, []string{}
	for _, path := range namespaces {
		if strings.HasPrefix(path, "git:") {
			gitpaths = append(gitpaths, path)
		} else {
			filepaths = append(filepaths, path)
		}
	}
	if len(filepaths) > 0 {
		fmt.Println(indentString + "Paths affected:")
		for _, path := range filepaths {
			fmt.Println(strings.Repeat(indentString, 2) + path)
		}
	}
	if len(gitpaths) > 0 {
		fmt.Println(indentString + "Refs affected:")
		for _, path := range gitpaths {
			fmt.Println(strings.Repeat(indentString, 2) + path)
		}
	}
}
