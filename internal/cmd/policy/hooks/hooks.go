// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package hooks

import (
	"github.com/gittuf/gittuf/internal/cmd/policy/hooks/add"
	"github.com/gittuf/gittuf/internal/cmd/policy/hooks/listhooks"
	"github.com/gittuf/gittuf/internal/cmd/policy/hooks/remove"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	o := &persistent.Options{}
	cmd := &cobra.Command{
		Use:               "hooks",
		Short:             "Tools to manage git hooks",
		DisableAutoGenTag: true,
	}
	o.AddPersistentFlags(cmd)

	cmd.AddCommand(add.New(o))
	cmd.AddCommand(remove.New(o))
	cmd.AddCommand(listhooks.New())

	return cmd
}
