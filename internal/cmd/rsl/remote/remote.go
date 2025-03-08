// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package remote

import (
	"github.com/gittuf/gittuf/internal/cmd/rsl/remote/pull"
	"github.com/gittuf/gittuf/internal/cmd/rsl/remote/push"
	"github.com/gittuf/gittuf/internal/cmd/rsl/remote/reconcile"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "remote",
		Short:             "Tools for managing remote RSLs",
		DisableAutoGenTag: true,
	}

	cmd.AddCommand(pull.New())
	cmd.AddCommand(push.New())
	cmd.AddCommand(reconcile.New())

	return cmd
}
