// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"github.com/gittuf/gittuf/internal/cmd/utils/getgithubuserid"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "utils",
		Short:             "Supporting tools for gittuf's operation",
		DisableAutoGenTag: true,
	}

	cmd.AddCommand(getgithubuserid.New())
	return cmd
}
