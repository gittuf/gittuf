// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package hooks

import (
	"github.com/gittuf/gittuf/internal/cmd/hooks/add"
	"github.com/gittuf/gittuf/internal/cmd/hooks/apply"
	i "github.com/gittuf/gittuf/internal/cmd/hooks/init"
	"github.com/gittuf/gittuf/internal/cmd/hooks/load"
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

	cmd.AddCommand(i.New(o))
	cmd.AddCommand(add.New())
	cmd.AddCommand(apply.New())
	cmd.AddCommand(load.New())

	return cmd
}
