// SPDX-License-Identifier: Apache-2.0

package remote

import (
	"github.com/gittuf/gittuf/internal/cmd/rsl/remote/check"
	"github.com/gittuf/gittuf/internal/cmd/rsl/remote/pull"
	"github.com/gittuf/gittuf/internal/cmd/rsl/remote/push"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "remote",
		Short:             "Tools for managing remote RSLs",
		DisableAutoGenTag: true,
	}

	cmd.AddCommand(check.New())
	cmd.AddCommand(pull.New())
	cmd.AddCommand(push.New())

	return cmd
}
