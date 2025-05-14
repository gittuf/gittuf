//go:generate go run .

// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	"github.com/gittuf/gittuf/internal/cmd/root"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var (
	dir string
	cmd = &cobra.Command{
		Use:   "gendoc",
		Short: "Generate Markdown documentation for all commands in gittuf",
		Long:  `The 'gendoc' command generates Markdown documentation for all available commands in the gittuf CLI. The generated documentation will be saved to the specified directory, which defaults to the current working directory if not provided.`,
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return doc.GenMarkdownTree(root.New(), dir)
		},
	}
)

func init() {
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "Path to directory in which to generate docs")
}

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
