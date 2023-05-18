package cmd

import (
	"context"
	"os"

	"github.com/adityasaky/gittuf/internal/policy"
	"github.com/adityasaky/gittuf/internal/repository"
	"github.com/spf13/cobra"
)

var policyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Tools to manage gittuf policies.",
}

var policyInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize policy file.",
	RunE:  runPolicyInit,
}

var policyAddRuleCmd = &cobra.Command{
	Use:   "add-rule",
	Short: "Add a new rule to a policy file.",
	RunE:  runPolicyAddRule,
}

var policyRemoveRuleCmd = &cobra.Command{
	Use:   "remove-rule",
	Short: "Remove rule from a policy file.",
	RunE:  runPolicyRemoveRule,
}

var (
	targetsFileName    string
	ruleName           string
	authorizedKeyFiles []string
	rulePatterns       []string
)

// init for gittuf policy init
func init() {
	policyInitCmd.Flags().StringVar(
		&targetsFileName,
		"policy-name",
		policy.TargetsRoleName,
		"Policy file to create.",
	)

	policyCmd.AddCommand(policyInitCmd)
}

// init for gittuf policy add-rule
func init() {
	policyAddRuleCmd.Flags().StringVar(
		&targetsFileName,
		"policy-name",
		policy.TargetsRoleName,
		"Policy file to add rule to.",
	)

	policyAddRuleCmd.Flags().StringVar(
		&ruleName,
		"rule-name",
		"",
		"Name of rule.",
	)

	policyAddRuleCmd.Flags().StringArrayVar(
		&authorizedKeyFiles,
		"authorize-key",
		[]string{},
		"Authorized public key for rule.",
	)

	policyAddRuleCmd.Flags().StringArrayVar(
		&rulePatterns,
		"rule-pattern",
		[]string{},
		"Patterns used to identify namespaces rule applies to.",
	)

	policyAddRuleCmd.MarkFlagRequired("rule-name")     //nolint:errcheck
	policyAddRuleCmd.MarkFlagRequired("authorize-key") //nolint:errcheck
	policyAddRuleCmd.MarkFlagRequired("rule-pattern")  //nolint:errcheck

	policyCmd.AddCommand(policyAddRuleCmd)
}

// init for gittuf policy remove-rule
func init() {
	policyRemoveRuleCmd.Flags().StringVar(
		&targetsFileName,
		"policy-name",
		policy.TargetsRoleName,
		"Policy file to remove rule from.",
	)

	policyRemoveRuleCmd.Flags().StringVar(
		&ruleName,
		"rule-name",
		"",
		"Name of rule.",
	)

	policyRemoveRuleCmd.MarkFlagRequired("rule-name") //nolint:errcheck

	policyCmd.AddCommand(policyRemoveRuleCmd)
}

// init for gittuf policy
func init() {
	policyCmd.PersistentFlags().StringVarP(
		&signingKeyString,
		"signing-key",
		"k",
		"",
		"Signing key to use to sign policy file.",
	)

	policyCmd.MarkPersistentFlagRequired("signing-key") //nolint:errcheck

	rootCmd.AddCommand(policyCmd)
}

func runPolicyInit(cmd *cobra.Command, args []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	keyBytes, err := os.ReadFile(signingKeyString)
	if err != nil {
		return err
	}

	return repo.InitializeTargets(context.Background(), keyBytes, targetsFileName, true)
}

func runPolicyAddRule(cmd *cobra.Command, args []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	keyBytes, err := os.ReadFile(signingKeyString)
	if err != nil {
		return err
	}

	authorizedKeysBytes := [][]byte{}
	for _, file := range authorizedKeyFiles {
		kb, err := os.ReadFile(file)
		if err != nil {
			return err
		}

		authorizedKeysBytes = append(authorizedKeysBytes, kb)
	}

	return repo.AddDelegation(context.Background(), keyBytes, targetsFileName, ruleName, authorizedKeysBytes, rulePatterns, true)
}

func runPolicyRemoveRule(cmd *cobra.Command, args []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	keyBytes, err := os.ReadFile(signingKeyString)
	if err != nil {
		return err
	}

	return repo.RemoveDelegation(context.Background(), keyBytes, targetsFileName, ruleName, true)
}
