// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package listglobalrules

import (
	"fmt"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/policy"
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

	rules, err := repo.ListGlobalRules(cmd.Context(), policy.PolicyRef)
	if len(rules) == 0 {
		fmt.Println("The rules slice is empty")
	}
	if err != nil {
		return err
	}

	for _, curRule := range rules {
		fmt.Printf("GlobalRule: %v\n", curRule.GetName())

		switch rule := curRule.(type) {
		case tuf.GlobalRuleThreshold:
			fmt.Println(indentString + "GlobalRule Type: GlobalRuleThreshold")
			fmt.Printf(indentString+"Refs affected: %v\n", rule.GetProtectedNamespaces())
			fmt.Printf(indentString+"Threshold: %d\n", rule.GetThreshold())

		case tuf.GlobalRuleBlockForcePushes:
			fmt.Println(indentString + "GlobalRule Type: GlobalRuleBlockForcePushes")
			fmt.Printf(indentString+"Refs affected: %v\n", rule.GetProtectedNamespaces())

		default:
			return tuf.ErrUnknownGlobalRuleType
		}
	}

	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "list-globalrules",
		Short:             "List global rules for the current state",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
