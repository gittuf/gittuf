package cmd

import (
	"github.com/adityasaky/gittuf/internal/common"
	"github.com/adityasaky/gittuf/internal/rsl"
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
	Args:  cobra.MinimumNArgs(1),
	RunE:  runRslAdd,
}

var (
	annotation bool
	skip       bool
	message    string
)

func init() {
	rslAddCmd.Flags().BoolVar(
		&annotation,
		"annotate",
		false,
		"Indicate annotation type",
	)

	rslAddCmd.Flags().BoolVar(
		&skip,
		"skip",
		false,
		"Indicate if annotation entry is skip type",
	)

	rslAddCmd.Flags().StringVarP(
		&message,
		"message",
		"m",
		"",
		"Message to be included in entry",
	)

	rslCmd.AddCommand(rslInitCmd)
	rslCmd.AddCommand(rslAddCmd)

	devCmd.AddCommand(rslCmd)
}

func runRslInit(cmd *cobra.Command, args []string) error {
	if err := common.InitializeGittufNamespace("."); err != nil {
		return err
	}
	return rsl.InitializeNamespace()
}

func runRslAdd(cmd *cobra.Command, args []string) error {
	if !annotation {
		return rsl.NewEntry(args[0], plumbing.ZeroHash).Commit(true)
	}

	entryIDs := []plumbing.Hash{}
	for _, r := range args {
		entryIDs = append(entryIDs, plumbing.NewHash(r))
	}
	return rsl.NewAnnotation(entryIDs, skip, message).Commit(true)
}
