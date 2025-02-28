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

type options struct{}

func (o *options) AddFlags(cmd *cobra.Command) {}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := gittuf.LoadRepository()
	if err != nil {
		return err
	}

	rules, err := repo.ListGlobalRules(cmd.Context())
	if len(rules) == 0 {
		fmt.Println("No global rules are currently defined.")
	}
	if err != nil {
		return err
	}

	thresholdRules := []tuf.GlobalRule{}
	blockForcePushesRules := []tuf.GlobalRule{}
	for _, curRule := range rules {
		switch globalRule := curRule.(type) {
		case tuf.GlobalRuleThreshold:
			thresholdRules = append(thresholdRules, globalRule)
		case tuf.GlobalRuleBlockForcePushes:
			blockForcePushesRules = append(blockForcePushesRules, globalRule)
		}
	}
	rules = append(thresholdRules, blockForcePushesRules...)

	for _, curRule := range rules {
		fmt.Printf("Global Rule: %v\n", curRule.GetName())
		switch rule := curRule.(type) {
		case tuf.GlobalRuleThreshold:
			fmt.Println(indentString + "Type: " + tuf.GlobalRuleThresholdType)
			gitpaths, filepaths := []string{}, []string{}
			for _, path := range rule.GetProtectedNamespaces() {
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
			fmt.Printf(indentString+"Threshold: %d\n", rule.GetThreshold())
		case tuf.GlobalRuleBlockForcePushes:
			fmt.Println(indentString + "Type: " + tuf.GlobalRuleBlockForcePushesType)
			gitpaths, filepaths := []string{}, []string{}
			for _, path := range rule.GetProtectedNamespaces() {
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

		default:
			return tuf.ErrUnknownGlobalRuleType
		}
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
