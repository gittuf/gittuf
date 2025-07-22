// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	i "github.com/gittuf/gittuf/internal/cmd/cache/init"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage gittuf's caching functionality",
		Long: `The 'cache' command group contains subcommands to manage gittuf's local persistent cache.
This cache helps improve performance by storing metadata locally. The cache is local-only and is not synchronized with remote repositories.`,

		DisableAutoGenTag: true,
	}

	cmd.AddCommand(i.New())

	return cmd
}
