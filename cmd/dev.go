package cmd

import (
	"fmt"

	"github.com/adityasaky/gittuf/gittuf"
	"github.com/spf13/cobra"
)

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Useful utilities for development / testing",
}

var devUndoCommitCmd = &cobra.Command{
	Use:   "undo-commit",
	Short: "Undo last commit",
	Run:   runDevUndoCommit,
}

func init() {
	devCmd.AddCommand(devUndoCommitCmd)

	rootCmd.AddCommand(devCmd)
}

func runDevUndoCommit(cmd *cobra.Command, args []string) {
	cause := fmt.Errorf("dummy error for testing")
	err := gittuf.UndoLastCommit(cause)
	if err != cause {
		fmt.Println("Error:", err)
	}
}
