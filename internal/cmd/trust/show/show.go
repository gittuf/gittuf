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

	fmt.Println("Repository Location:")
	location, err := repo.GetRepositoryLocation(cmd.Context())
	if err != nil {
		return nil
	}
	if location == "" {
		fmt.Printf("No repository location is currently defined.\n")
	} else {
		fmt.Printf(indentString+"%s\n", location)
	}

	fmt.Println("\nRoot of Trust keys:")

	rootPrincipals, err := repo.GetRootKeys(cmd.Context())
	if err != nil {
		return err
	}

	for _, principal := range rootPrincipals {
		fmt.Printf(indentString+"%s\n", principal.ID())
	}

	rootThreshold, err := repo.GetRootThreshold(cmd.Context())
	if err != nil {
		return err
	}
	fmt.Printf("Root of Trust threshold: %d\n", rootThreshold)

	fmt.Println("\nPolicy keys:")
	targetPrincipals, err := repo.GetPrimaryRuleFilePrincipals(cmd.Context())
	if err != nil {
		return err
	}

	for _, principal := range targetPrincipals {
		fmt.Printf(indentString+"%s\n", principal.ID())
	}

	policyThreshold, err := repo.GetTopLevelTargetsThreshold(cmd.Context())
	if err != nil {
		return err
	}
	fmt.Printf("Policy threshold: %d\n", policyThreshold)

	fmt.Println("\nGlobal Rules:")

	rules, err := repo.ListGlobalRules(cmd.Context(), o.targetRef)
	if err != nil {
		return err
	}
	if len(rules) == 0 {
		fmt.Println("No global rules are currently defined.")
	} else {
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
	}

	fmt.Println("\nPropagation Directives:")

	directives, err := repo.ListPropagationDirectives(cmd.Context(), o.targetRef)
	if err != nil {
		return err
	}
	if len(directives) == 0 {
		fmt.Println("No propagation directives are currently defined.")
	} else {
		// TODO: switch to the display package
		for _, pd := range directives {
			fmt.Printf("Propagation Directive: %s\n", pd.GetName())
			fmt.Printf("  Upstream Repository:   %s\n", pd.GetUpstreamRepository())
			fmt.Printf("  Upstream Reference:    %s\n", pd.GetUpstreamReference())
			fmt.Printf("  Upstream Path:         %s\n", pd.GetUpstreamPath())
			fmt.Printf("  Downstream Reference:  %s\n", pd.GetDownstreamReference())
			fmt.Printf("  Downstream Path:       %s\n", pd.GetDownstreamPath())
		}
	}

	fmt.Println("\nHooks:")

	hookStages, err := repo.ListHooks(cmd.Context(), o.targetRef)
	if err != nil {
		return err
	}

	for stage, data := range hookStages {
		fmt.Printf(indentString+"Stage %s:\n", stage.String())
		if len(data) == 0 {
			fmt.Println(strings.Repeat(indentString, 2) + "No hooks are currently defined for this stage.")
		} else {
			for _, hook := range data {
				fmt.Printf(strings.Repeat(indentString, 2)+"Hook '%s':\n", hook.ID())

				fmt.Printf("%sPrincipal IDs:\n", strings.Repeat(indentString, 3))
				for _, id := range hook.GetPrincipalIDs().Contents() {
					fmt.Printf("%s%s\n", strings.Repeat(indentString, 4), id)
				}

				fmt.Printf("%sHashes:\n", strings.Repeat(indentString, 3))
				for algo, hash := range hook.GetHashes() {
					fmt.Printf("%s%s: %s\n", strings.Repeat(indentString, 4), algo, hash)
				}

				fmt.Printf("%sEnvironment:\n", strings.Repeat(indentString, 3))
				fmt.Printf("%s%s\n", strings.Repeat(indentString, 4), hook.GetEnvironment().String())

				fmt.Printf("%sTimeout:\n", strings.Repeat(indentString, 3))
				fmt.Printf("%s%d\n", strings.Repeat(indentString, 4), hook.GetTimeout())
			}
			fmt.Println()
		}
	}

	fmt.Println("\nGitHub App approvals status:")

	approvals, err := repo.AreGitHubAppApprovalsTrusted(cmd.Context())
	if err != nil {
		return err
	}

	keys, err := repo.GetGitHubAppPrincipals(cmd.Context())
	if err != nil {
		return err
	}

	if len(approvals) == 0 {
		fmt.Println("No GitHub App instances are defined.")
	} else {
		for appName, isTrusted := range approvals {
			fmt.Printf(indentString+"Status for %s:\n", appName)
			if isTrusted {
				fmt.Println(strings.Repeat(indentString, 2) + "Approvals: Trusted")
			} else {
				fmt.Println(strings.Repeat(indentString, 2) + "Approvals: Untrusted")
			}
			appKey := keys[appName][0]
			fmt.Printf(strings.Repeat(indentString, 2)+"Signing Key: %s\n", appKey.ID())
		}
	}

	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "show",
		Short:             "Show root metadata",
		Long:              "This command displays gittuf's root metadata for the current repository.",
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
