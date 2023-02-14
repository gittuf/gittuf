package cmd

import (
	"fmt"

	"github.com/adityasaky/gittuf/internal/gitinterface"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Test git config seen by gittuf",
	RunE:  runConfig,
}

func init() {
	devCmd.AddCommand(configCmd)
}

func runConfig(cmd *cobra.Command, args []string) error {
	fmt.Println("Detected Git Config:")

	gitConfig, err := gitinterface.GetConfig() //nolint:staticcheck
	if err != nil {
		return err
	}

	for k, v := range gitConfig {
		fmt.Println(k, v)
	}

	return nil
}
