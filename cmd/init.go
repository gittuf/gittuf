package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/adityasaky/gittuf/internal/common"
	"github.com/adityasaky/gittuf/pkg/tuf"
	"github.com/adityasaky/gittuf/policy"
	"github.com/adityasaky/gittuf/rsl"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "gittuf's initialization workflows for a repository",
}

var initPreSignCmd = &cobra.Command{
	Use:   "pre-sign",
	Short: "Create or add to unsigned Root metadata using out-of-band information",
	Run:   runInitPreSign,
}

var (
	rootPublicKeyPath     string
	rootThreshold         int
	rootExpiryTimestamp   string
	targetsPublicKeyPaths []string
	targetsThreshold      int
)

func init() {
	initPreSignCmd.Flags().StringVar(
		&rootPublicKeyPath,
		"root-public-key",
		"",
		"Public key of Root role's key",
	)

	initPreSignCmd.Flags().IntVar(
		&rootThreshold,
		"root-threshold",
		-1,
		"Threshold of keys that must sign the Root role",
	)

	initPreSignCmd.Flags().StringVar(
		&rootExpiryTimestamp,
		"root-expiry",
		"",
		"Expiry timestamp in ISO 8601 format",
	)

	initPreSignCmd.Flags().StringSliceVar(
		&targetsPublicKeyPaths,
		"targets-public-key",
		[]string{},
		"Public keys to use for Targets role",
	)

	initPreSignCmd.Flags().IntVar(
		&targetsThreshold,
		"targets-threshold",
		-1,
		"Threshold of keys that must sign the Targets role",
	)

	initPreSignCmd.MarkFlagRequired("root-public-key")
	initPreSignCmd.MarkFlagRequired("root-threshold")
	initPreSignCmd.MarkFlagRequired("root-expiry")
	initPreSignCmd.MarkFlagRequired("targets-public-key")

	initCmd.AddCommand(initPreSignCmd)

	rootCmd.AddCommand(initCmd)
}

func runInitPreSign(cmd *cobra.Command, args []string) {
	if err := common.InitializeGittufNamespace(); err != nil {
		fmt.Println("Error creating gittuf namespace:", err)
		os.Exit(1)
	}

	if err := rsl.InitializeNamespace(); err != nil {
		fmt.Println("Error creating gittuf's RSL namespace:", err)
		os.Exit(1)
	}

	if err := policy.InitializeNamespace(); err != nil {
		fmt.Println("Error creating gittuf's policy namespace:", err)
		os.Exit(1)
	}

	keyContents, err := os.ReadFile(rootPublicKeyPath)
	if err != nil {
		fmt.Println("Error reading root public key:", err)
		os.Exit(1)
	}

	rootPublicKey, err := tuf.LoadKeyFromBytes(keyContents)
	if err != nil {
		fmt.Println("Error reading root public key:", err)
		os.Exit(1)
	}

	targetsPublicKeys := map[string]tuf.Key{}
	for _, p := range targetsPublicKeyPaths {
		keyContents, err := os.ReadFile(p)
		if err != nil {
			fmt.Println("Error reading targets key:", err)
			os.Exit(1)
		}

		k, err := tuf.LoadKeyFromBytes(keyContents)
		if err != nil {
			fmt.Println("Error reading targets key:", err)
			os.Exit(1)
		}
		targetsPublicKeys[k.ID()] = k
	}

	rootMetadata, err := policy.GenerateOrAppendToUnsignedRootMetadata(rootPublicKey, rootThreshold, rootExpiryTimestamp, targetsPublicKeys, targetsThreshold)
	if err != nil {
		fmt.Println("Error creating unsigned Root metadata:", err)
		os.Exit(1)
	}

	rootMetadataJson, err := json.MarshalIndent(rootMetadata, "", "\t")
	if err != nil {
		fmt.Println("Error creating unsigned Root metadata:", err)
		os.Exit(1)
	}
	fmt.Println("Here's the root of trust generated for your gittuf repository:")
	fmt.Println(string(rootMetadataJson))

	prompt := promptui.Select{
		Label: "Proceed?",
		Items: []string{"Yes", "No, abort"},
	}

	selection, _, err := prompt.Run()
	if err != nil {
		fmt.Println("Error creating unsigned Root metadata:", err)
		os.Exit(1)
	}

	if selection == 1 {
		fmt.Println("Aborting generation of Root!")
		os.Exit(0)
	}

	if err := policy.StageUnsignedRootMetadata(rootPublicKey, rootMetadata); err != nil {
		fmt.Println("Error staging unsigned Root:", err)
		os.Exit(1)
	}
}
