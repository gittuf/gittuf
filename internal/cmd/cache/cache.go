// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	i "github.com/gittuf/gittuf/internal/cmd/cache/init"
	"github.com/gittuf/gittuf/internal/cmd/cache/reset"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "cache",
		Short:             "Manage gittuf's caching functionality",
		DisableAutoGenTag: true,
	}

	cmd.AddCommand(i.New())
	cmd.AddCommand(reset.New())

	return cmd
}
