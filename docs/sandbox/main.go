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

	"github.com/gittuf/gittuf/internal/luasandbox"
	"github.com/spf13/cobra"
)

var (
	dir string
	cmd = &cobra.Command{
		Use:   "gendoc",
		Short: "Generate sandbox docs",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			type apiRecord struct {
				name string
				api  luasandbox.API
			}
			allAPIs := []*apiRecord{}
			for name, API := range luasandbox.RegisterAPIs {
				allAPIs = append(allAPIs, &apiRecord{name: name, api: API})
			}

			sort.Slice(allAPIs, func(i, j int) bool {
				return allAPIs[i].name < allAPIs[j].name
			})

			allLines := []string{
				"# Lua Sandbox APIs",
				"",
			}
			for _, record := range allAPIs {
				allLines = append(allLines, fmt.Sprintf("## %s", record.name))
				allLines = append(allLines, "")
				allLines = append(allLines, fmt.Sprintf("**Signature:** `%s`", record.api.GetSignature()))
				allLines = append(allLines, "")
				allLines = append(allLines, record.api.GetHelp())

				for index, example := range record.api.GetExamples() {
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
