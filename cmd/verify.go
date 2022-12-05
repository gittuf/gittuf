package cmd

import (
	"fmt"

	"github.com/adityasaky/gittuf/gittuf"
	"github.com/spf13/cobra"
)

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify the repository",
}

var verifyStateCmd = &cobra.Command{
	Use:   "state",
	Short: "Verifies a target's hash matches signed TUF metadata",
	Run:   runVerifyState,
	Args:  cobra.ExactArgs(1),
}

func init() {
	verifyCmd.AddCommand(verifyStateCmd)
	rootCmd.AddCommand(verifyCmd)
}

func runVerifyState(cmd *cobra.Command, args []string) {
	repo, err := getGittufRepo()
	if err != nil {
		fmt.Println("Error:", err)
	}
	err = gittuf.VerifyState(repo, args[0])
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Println("Target", args[0], "verified successfully!")
	}
}
