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
	if err != nil {
		return err
	}
	if len(rules) == 0 {
		_, err := fmt.Fprintln(stdOut, "No global rules are currently defined.")
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
		if _, err := fmt.Fprintf(stdOut, "Global Rule: %v\n", curRule.GetName()); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(stdOut, indentString+"Type: "+tuf.GlobalRuleThresholdType); err != nil {
			return err
		}
		if err := printNamespaces(stdOut, curRule.GetProtectedNamespaces()); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(stdOut, indentString+"Threshold: %d\n", curRule.GetThreshold()); err != nil {
			return err
		}
	}

	for _, curRule := range blockForcePushesRules {
		if _, err := fmt.Fprintf(stdOut, "Global Rule: %v\n", curRule.GetName()); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(stdOut, indentString+"Type: "+tuf.GlobalRuleBlockForcePushesType); err != nil {
			return err
		}
		if err := printNamespaces(stdOut, curRule.GetProtectedNamespaces()); err != nil {
			return err
		}
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

func printNamespaces(stdOut io.Writer, namespaces []string) error {
	gitpaths, filepaths := []string{}, []string{}
	for _, path := range namespaces {
		if strings.HasPrefix(path, "git:") {
			gitpaths = append(gitpaths, path)
		} else {
			filepaths = append(filepaths, path)
		}
	}
	if len(filepaths) > 0 {
		if _, err := fmt.Fprintln(stdOut, indentString+"Paths affected:"); err != nil {
			return err
		}
		for _, path := range filepaths {
			if _, err := fmt.Fprintln(stdOut, strings.Repeat(indentString, 2)+path); err != nil {
				return err
			}
		}
	}
	if len(gitpaths) > 0 {
		if _, err := fmt.Fprintln(stdOut, indentString+"Refs affected:"); err != nil {
			return err
		}
		for _, path := range gitpaths {
			if _, err := fmt.Fprintln(stdOut, strings.Repeat(indentString, 2)+path); err != nil {
				return err
			}
		}
	}
	return nil
}
