package cmd

import "github.com/spf13/cobra"

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Testing tools for gittuf developers, you probably DON'T want to use this suite of commands",
}

func init() {
	rootCmd.AddCommand(devCmd)
}
