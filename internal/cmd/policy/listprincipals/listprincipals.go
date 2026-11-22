// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package listprincipals

import (
	"fmt"
	"strings"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/spf13/cobra"
)

const indentString = "    "

type options struct {
	policyRef  string
	policyName string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.policyRef,
		"policy-ref",
		"policy",
		"specify which policy ref should be inspected",
	)

	cmd.Flags().StringVar(
		&o.policyName,
		"policy-name",
		tuf.TargetsRoleName,
		"specify rule file to list principals for",
	)
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	principals, err := repo.ListPrincipals(cmd.Context(), o.policyRef, o.policyName)
	if err != nil {
		return err
	}

	stdOut := cmd.OutOrStdout()

	if len(principals) == 0 {
		if _, err := fmt.Fprintln(stdOut, "No principals are currently defined."); err != nil {
			return err
		}
		return nil
	}

	count := 0
	for _, principal := range principals {
		if _, err := fmt.Fprintf(stdOut, "Principal %s:\n", principal.ID()); err != nil {
			return err
		}

		if _, err := fmt.Fprintf(stdOut, indentString+"Keys:\n"); err != nil {
			return err
		}
		for _, key := range principal.Keys() {
			if _, err := fmt.Fprintf(stdOut, strings.Repeat(indentString, 2)+"%s (%s)\n", key.KeyID, key.KeyType); err != nil {
				return err
			}
		}

		customMetadata := principal.CustomMetadata()
		if len(customMetadata) > 0 {
			if _, err := fmt.Fprintf(stdOut, indentString+"Custom Metadata:\n"); err != nil {
				return err
			}
			for key, value := range principal.CustomMetadata() {
				if _, err := fmt.Fprintf(stdOut, strings.Repeat(indentString, 2)+"%s: %s\n", key, value); err != nil {
					return err
				}
			}
		}
		if count < len(principals)-1 {
			if _, err := fmt.Fprintln(stdOut); err != nil {
				return err
			}
		}
		count++
	}
	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "list-principals",
		Short:             "List principals for the current policy in the specified rule file",
		Long:              "The 'list-principals' command lists the trusted principals defined in a gittuf policy file. It is used to review which keys and persons are recognized before referencing them in rules.",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
