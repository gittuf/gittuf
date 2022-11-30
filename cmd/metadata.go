package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/adityasaky/gittuf/gittuf"
	"github.com/adityasaky/gittuf/internal/gitstore"
	"github.com/spf13/cobra"
)

var metadataCmd = &cobra.Command{
	Use:   "metadata",
	Short: "Inspect gittuf metadata",
}

var metadataInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize gittuf namespace",
	RunE:  runMetadataInit,
}

var metadataLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List current set of gittuf metadata",
	RunE:  runMetadataLs,
}

var metadataAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add specified file to gittuf namespace",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runMetadataAdd,
}

var metadataCatCmd = &cobra.Command{
	Use:   "cat",
	Short: "Print specified file on standard output",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runMetadataCat,
}

var metadataRmCmd = &cobra.Command{
	Use:   "rm",
	Short: "Remove specified files",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runMetadataRm,
}

var (
	long bool
)

func init() {
	metadataLsCmd.Flags().BoolVarP(
		&long,
		"long",
		"l",
		false,
		"Use a long listing format",
	)

	metadataCmd.AddCommand(metadataInitCmd)
	metadataCmd.AddCommand(metadataLsCmd)
	metadataCmd.AddCommand(metadataAddCmd)
	metadataCmd.AddCommand(metadataCatCmd)
	metadataCmd.AddCommand(metadataRmCmd)

	rootCmd.AddCommand(metadataCmd)
}

func runMetadataInit(cmd *cobra.Command, args []string) error {
	dir, err := gittuf.GetRepoRootDir()
	if err != nil {
		return err
	}
	return gitstore.InitNamespace(dir)
}

func runMetadataLs(cmd *cobra.Command, args []string) error {
	repo, err := getGittufRepo()
	if err != nil {
		return err
	}

	currentTree, err := repo.Tree()
	if err != nil {
		return err
	}

	for _, e := range currentTree.Entries {
		if long {
			fmt.Println(e.Mode.String(), e.Hash.String(), e.Name)
		} else {
			fmt.Println(e.Name)
		}
	}
	return nil
}

func runMetadataAdd(cmd *cobra.Command, args []string) error {
	repo, err := getGittufRepo()
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

func runMetadataCat(cmd *cobra.Command, args []string) error {
	repo, err := getGittufRepo()
	if err != nil {
		return err
	}

	for _, n := range args {
		n = strings.TrimSuffix(n, ".json")
		fmt.Println(repo.GetCurrentFileString(n))
	}

	return nil
}

func runMetadataRm(cmd *cobra.Command, args []string) error {
	repo, err := getGittufRepo()
	if err != nil {
		return err
	}

	roles := []string{}

	for _, n := range args {
		roles = append(roles, strings.TrimSuffix(n, ".json"))
	}

	return repo.RemoveFiles(roles)
}
