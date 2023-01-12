package cmd

import (
	"github.com/adityasaky/gittuf/gittuf"
	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Pushes changes to specified remote",
	RunE:  runPush,
	Args:  cobra.ExactArgs(2),
}

func init() {
	rootCmd.AddCommand(pushCmd)
}

func runPush(cmd *cobra.Command, args []string) error {
	store, err := getGitStore()
	if err != nil {
		return err
	}
	return gittuf.Push(store, args[0], args[1])
}
