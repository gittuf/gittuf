package cmd

import (
	"github.com/adityasaky/gittuf/internal/common"
	"github.com/adityasaky/gittuf/rsl"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/spf13/cobra"
)

var rslCmd = &cobra.Command{
	Use:   "rsl",
	Short: "Testing tools for gittuf's implementation of RSLs",
}

var rslInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize RSL namespace",
	RunE:  runRslInit,
}

var rslAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add entry to RSL",
	Args:  cobra.ExactArgs(1),
	RunE:  runRslAdd,
}

func init() {
	devCmd.AddCommand(rslCmd)

	rslCmd.AddCommand(rslInitCmd)
	rslCmd.AddCommand(rslAddCmd)
}

func runRslInit(cmd *cobra.Command, args []string) error {
	if err := common.InitializeGittufNamespace("."); err != nil {
		return err
	}
	return rsl.InitializeNamespace()
}

func runRslAdd(cmd *cobra.Command, args []string) error {
	return rsl.AddEntry(args[0], plumbing.ZeroHash, true)
}
