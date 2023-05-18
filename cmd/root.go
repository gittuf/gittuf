package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var signingKeyString string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gittuf",
	Short: "A security layer for Git repositories, powered by TUF.",
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
