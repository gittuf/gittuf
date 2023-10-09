// SPDX-License-Identifier: Apache-2.0

package remote

import (
	"github.com/gittuf/gittuf/internal/cmd/rsl/remote/check"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remote",
		Short: "Tools for managing remote RSLs",
	}

	cmd.AddCommand(check.New())

	return cmd
}
