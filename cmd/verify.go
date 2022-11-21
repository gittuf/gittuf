package cmd

import (
	"fmt"

	"github.com/adityasaky/gittuf/gittuf"
	"github.com/spf13/cobra"
)

var verifyCmd = &cobra.Command{
	Use:   "verify <target>",
	Short: "Verifies a target's hash matches signed TUF metadata",
	Run:   runVerify,
	Args:  cobra.ExactArgs(1),
}

func init() {
	rootCmd.AddCommand(verifyCmd)
}

func runVerify(cmd *cobra.Command, args []string) {
	err := gittuf.Verify(args[0])
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Println("Target", args[0], "verified successfully!")
	}
}
