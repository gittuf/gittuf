// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package listglobalrules

import (
	"fmt"
	"io"
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
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	stdOut := cmd.OutOrStdout()

	rules, err := repo.ListGlobalRules(cmd.Context(), o.targetRef)
	if len(rules) == 0 {
		fmt.Fprintln(stdOut, "No global rules are currently defined.")
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
		fmt.Fprintf(stdOut, "Global Rule: %v\n", curRule.GetName())
		fmt.Fprintln(stdOut, indentString+"Type: "+tuf.GlobalRuleThresholdType)
		printNamespaces(stdOut, curRule.GetProtectedNamespaces())
		fmt.Fprintf(stdOut, indentString+"Threshold: %d\n", curRule.GetThreshold())
	}

	for _, curRule := range blockForcePushesRules {
		fmt.Fprintf(stdOut, "Global Rule: %v\n", curRule.GetName())
		fmt.Fprintln(stdOut, indentString+"Type: "+tuf.GlobalRuleBlockForcePushesType)
		printNamespaces(stdOut, curRule.GetProtectedNamespaces())
	}

	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "list-global-rules",
		Short:             "List global rules for the current state",
		Long:              "The 'list-global-rules' command lists the global rules currently defined in the repository's root of trust. It is used to review the repository-wide constraints in effect, with output grouped by rule type.",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}

func printNamespaces(stdOut io.Writer, namespaces []string) {
	gitpaths, filepaths := []string{}, []string{}
	for _, path := range namespaces {
		if strings.HasPrefix(path, "git:") {
			gitpaths = append(gitpaths, path)
		} else {
			filepaths = append(filepaths, path)
		}
	}
	if len(filepaths) > 0 {
		fmt.Fprintln(stdOut, indentString+"Paths affected:")
		for _, path := range filepaths {
			fmt.Fprintln(stdOut, strings.Repeat(indentString, 2)+path)
		}
	}
	if len(gitpaths) > 0 {
		fmt.Fprintln(stdOut, indentString+"Refs affected:")
		for _, path := range gitpaths {
			fmt.Fprintln(stdOut, strings.Repeat(indentString, 2)+path)
		}
	}
}
