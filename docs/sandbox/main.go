//go:generate go run .

// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/luasandbox"
	"github.com/spf13/cobra"
)

var (
	dir string
	cmd = &cobra.Command{
		Use:   "gendoc",
		Short: "Generate sandbox docs",
		Long: `The 'gendoc' command generates documentation for all available Lua sandbox APIs used within the gittuf CLI.

This includes API signatures, descriptions, and examples, making it easier for developers to understand and utilize available sandbox functionality.

The generated documentation is saved to the specified directory, defaulting to the current working directory if not provided.`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			repository, err := gitinterface.LoadRepository(".")
			if err != nil {
				return err
			}

			environment, err := luasandbox.NewLuaEnvironment(cmd.Context(), repository)
			if err != nil {
				return err
			}

			allAPIs := environment.GetAPIs()
			sort.Slice(allAPIs, func(i, j int) bool {
				return allAPIs[i].GetName() < allAPIs[j].GetName()
			})

			allLines := []string{
				"# Lua Sandbox APIs",
				"",
			}
			for _, api := range allAPIs {
				allLines = append(allLines, fmt.Sprintf("## %s", api.GetName()))
				allLines = append(allLines, "")
				allLines = append(allLines, fmt.Sprintf("**Signature:** `%s`", api.GetSignature()))
				allLines = append(allLines, "")
				allLines = append(allLines, api.GetHelp())

				for index, example := range api.GetExamples() {
					allLines = append(allLines, "") // we don't have a new line after help, this also adds spacing between examples
					allLines = append(allLines, fmt.Sprintf("### Example %d", index+1))
					allLines = append(allLines, "")
					allLines = append(allLines, "```")
					allLines = append(allLines, strings.TrimSpace(example))
					allLines = append(allLines, "```")
				}

				allLines = append(allLines, "") // trailing new line between records
			}

			completeDocBytes := []byte(strings.Join(allLines, "\n"))
			docPath := filepath.Join(dir, "README.md")
			return os.WriteFile(docPath, completeDocBytes, 0o600)
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
