package cmd

import (
	"context"
	"os"

	"github.com/adityasaky/gittuf/internal/repository"
	"github.com/spf13/cobra"
)

var trustCmd = &cobra.Command{
	Use:   "trust",
	Short: "Tools for gittuf's root of trust.",
}

var trustInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize gittuf root of trust for repository.",
	RunE:  runTrustInit,
}

var trustAddPolicyKeyCmd = &cobra.Command{
	Use:   "add-policy-key",
	Short: "Add Policy key to gittuf root of trust.",
	RunE:  runTrustAddPolicyKey,
}

var trustRemovePolicyKeyCmd = &cobra.Command{
	Use:   "remove-policy-key",
	Short: "Remove Policy key from gittuf root of trust.",
	RunE:  runTrustRemovePolicyKey,
}

var (
	signingKeyString string
	targetsKeyString string
	targetsKeyID     string
)

// init for gittuf trust init
func init() {
	trustCmd.AddCommand(trustInitCmd)
}

// init for gittuf trust add-policy-key
func init() {
	trustAddPolicyKeyCmd.Flags().StringVar(
		&targetsKeyString,
		"policy-key",
		"",
		"Policy key to add to root of trust.",
	)

	trustAddPolicyKeyCmd.MarkFlagRequired("policy-key") //nolint:errcheck

	trustCmd.AddCommand(trustAddPolicyKeyCmd)
}

// init for gittuf trust remove-policy-key
func init() {
	trustRemovePolicyKeyCmd.Flags().StringVar(
		&targetsKeyID,
		"policy-key-ID",
		"",
		"ID of Policy key to be removed from root of trust.",
	)

	trustRemovePolicyKeyCmd.MarkFlagRequired("policy-key-ID") //nolint:errcheck

	trustCmd.AddCommand(trustRemovePolicyKeyCmd)
}

func init() {
	trustCmd.PersistentFlags().StringVarP(
		&signingKeyString,
		"signing-key",
		"k",
		"",
		"Signing key to use to sign root of trust.",
	)

	trustCmd.MarkPersistentFlagRequired("signing-key") //nolint:errcheck

	rootCmd.AddCommand(trustCmd)
}

func runTrustInit(cmd *cobra.Command, args []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	keyBytes, err := os.ReadFile(signingKeyString)
	if err != nil {
		return err
	}

	return repo.InitializeRoot(context.Background(), keyBytes, true)
}

func runTrustAddPolicyKey(cmd *cobra.Command, args []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	rootKeyBytes, err := os.ReadFile(signingKeyString)
	if err != nil {
		return err
	}

	targetsKeyBytes, err := os.ReadFile(targetsKeyString)
	if err != nil {
		return err
	}

	return repo.AddTopLevelTargetsKey(context.Background(), rootKeyBytes, targetsKeyBytes, true)
}

func runTrustRemovePolicyKey(cmd *cobra.Command, args []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	rootKeyBytes, err := os.ReadFile(signingKeyString)
	if err != nil {
		return err
	}

	return repo.RemoveTopLevelTargetsKey(context.Background(), rootKeyBytes, targetsKeyID, true)
}
