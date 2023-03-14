package cmd

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"

	"github.com/adityasaky/gittuf/internal/common"
	"github.com/adityasaky/gittuf/internal/policy"
	"github.com/adityasaky/gittuf/internal/signers.go"
	"github.com/adityasaky/gittuf/internal/tuf"
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

var initSignRootCmd = &cobra.Command{
	Use:   "sign-root",
	Short: "Sign generated Root metadata",
	Run:   runInitSignRoot,
}

var (
	rootPublicKeyPath     string
	rootThreshold         int
	rootExpiryTimestamp   string
	targetsPublicKeyPaths []string
	targetsThreshold      int
	rootSigningKeyPath    string
)

// init for pre-sign
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

	initPreSignCmd.MarkFlagRequired("root-public-key")    //nolint:errcheck
	initPreSignCmd.MarkFlagRequired("root-threshold")     //nolint:errcheck
	initPreSignCmd.MarkFlagRequired("root-expiry")        //nolint:errcheck
	initPreSignCmd.MarkFlagRequired("targets-public-key") //nolint:errcheck
	initPreSignCmd.MarkFlagRequired("targets-threshold")  //nolint:errcheck
}

// init for sifn-root
func init() {
	initSignRootCmd.Flags().StringVar(
		&rootSigningKeyPath,
		"root-signing-key",
		"",
		"Signing key to be used",
	)

	initSignRootCmd.MarkFlagRequired("root-signing-key")
}

// final init
func init() {
	initCmd.AddCommand(initPreSignCmd)
	initCmd.AddCommand(initSignRootCmd)

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
		fmt.Println("Error reading Root public key:", err)
		os.Exit(1)
	}

	rootPublicKey, err := tuf.LoadKeyFromBytes(keyContents)
	if err != nil {
		fmt.Println("Error reading Root public key:", err)
		os.Exit(1)
	}

	targetsPublicKeys := map[string]tuf.Key{}
	for _, p := range targetsPublicKeyPaths {
		keyContents, err := os.ReadFile(p)
		if err != nil {
			fmt.Println("Error reading Targets public key:", err)
			os.Exit(1)
		}

		k, err := tuf.LoadKeyFromBytes(keyContents)
		if err != nil {
			fmt.Println("Error reading Targets public key:", err)
			os.Exit(1)
		}
		targetsPublicKeys[k.ID()] = k
	}

	rootMetadataEnv, err := policy.GenerateOrAppendToUnsignedRootMetadata(rootPublicKey, rootThreshold, rootExpiryTimestamp, targetsPublicKeys, targetsThreshold)
	if err != nil {
		fmt.Println("Error creating unsigned Root metadata:", err)
		os.Exit(1)
	}

	decodedPayload, err := base64.StdEncoding.DecodeString(rootMetadataEnv.Payload)
	if err != nil {
		fmt.Println("Error creating unsigned Root metadata:", err)
		os.Exit(1)
	}

	var prettyPrint bytes.Buffer
	if err := json.Indent(&prettyPrint, decodedPayload, "", "\t"); err != nil {
		fmt.Println("Error creating unsigned Root metadata:", err)
		os.Exit(1)
	}

	fmt.Println("Here's the root of trust generated for your gittuf repository:")
	fmt.Println(prettyPrint.String())

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

	if err := policy.StageUnsignedRootMetadata(rootPublicKey, rootMetadataEnv); err != nil {
		fmt.Println("Error staging unsigned Root:", err)
		os.Exit(1)
	}
}

func runInitSignRoot(cmd *cobra.Command, args []string) {
	signingKeyContents, err := os.ReadFile(rootSigningKeyPath)
	if err != nil {
		fmt.Println("Error reading signing key:", err)
		os.Exit(1)
	}

	rootMetadataEnv, err := policy.LoadStagedUnsignedRootMetadata()
	if err != nil {
		fmt.Println("Error loading unsigned Root metadata:", err)
		os.Exit(1)
	}

	signedRootMetadata, err := signers.SignEnvelope(rootMetadataEnv, signingKeyContents)
	if err != nil {
		fmt.Println("Error signing Root metadata:", err)
		os.Exit(1)
	}

	if err := policy.StageSignedRootMetadata(signedRootMetadata); err != nil {
		fmt.Println("Error staging signed Root metadata:", err)
		os.Exit(1)
	}
}
