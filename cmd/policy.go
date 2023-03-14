package cmd

import (
	"fmt"
	"os"

	"github.com/adityasaky/gittuf/internal/policy"
	"github.com/spf13/cobra"
)

var policyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Policy management utilities",
}

var policyApplyStagedCmd = &cobra.Command{
	Use:   "apply-staged",
	Short: "Apply staged policies",
	Run:   runPolicyApplyStaged,
}

func init() {
	policyCmd.AddCommand(policyApplyStagedCmd)

	rootCmd.AddCommand(policyCmd)
}

func runPolicyApplyStaged(cmd *cobra.Command, args []string) {
	if err := policy.ApplyStagedPolicy(); err != nil {
		fmt.Println("Error applying staged policies:", err)
		os.Exit(1)
	}
}
