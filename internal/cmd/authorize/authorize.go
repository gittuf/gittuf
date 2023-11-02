// SPDX-License-Identifier: Apache-2.0

package authorize

import (
	"github.com/gittuf/gittuf/internal/cmd/authorize/add"
	"github.com/gittuf/gittuf/internal/cmd/authorize/persistent"
	"github.com/gittuf/gittuf/internal/cmd/authorize/revoke"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	o := &persistent.Options{}
	cmd := &cobra.Command{
		Use:   "authorize",
		Short: "Add or revoke a detached authorization",
	}
	o.AddPersistentFlags(cmd)

	cmd.AddCommand(add.New(o))
	cmd.AddCommand(revoke.New(o))

	return cmd
}
