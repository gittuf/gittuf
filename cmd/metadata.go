package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/adityasaky/gittuf/internal/gitstore"
	"github.com/spf13/cobra"
)

var metadataCmd = &cobra.Command{
	Use:   "metadata",
	Short: "Inspect gittuf metadata",
}

var metadataCatCmd = &cobra.Command{
	Use:   "cat",
	Short: "Print specified file on standard output",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runMetadataCat,
}

var metadataAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add specified file to gittuf namespace",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runMetadataAdd,
}

var metadataInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize gittuf namespace",
	RunE:  runMetadataInit,
}

func init() {
	metadataCmd.AddCommand(metadataCatCmd)
	metadataCmd.AddCommand(metadataAddCmd)
	metadataCmd.AddCommand(metadataInitCmd)
	rootCmd.AddCommand(metadataCmd)
}

func runMetadataCat(cmd *cobra.Command, args []string) error {
	repo, err := gitstore.LoadRepository(".")
	if err != nil {
		return err
	}

	for _, n := range args {
		if !strings.HasSuffix(n, ".json") {
			n = n + ".json"
		}
		fmt.Println(repo.GetCurrentFileString(n))
	}

	return nil
}

func runMetadataAdd(cmd *cobra.Command, args []string) error {
	repo, err := gitstore.LoadRepository(".")
	if err != nil {
		return err
	}

	metadata := map[string][]byte{}

	for _, n := range args {
		c, err := os.ReadFile(n)
		if err != nil {
			return err
		}
		metadata[n] = c
	}
	return repo.StageAndCommitMultiple(metadata)
}

func runMetadataInit(cmd *cobra.Command, args []string) error {
	return gitstore.InitNamespace(".")
}
