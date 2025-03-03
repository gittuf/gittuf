// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package hooks

import (
	"github.com/gittuf/gittuf/internal/cmd/trust/hooks/add"
	"github.com/gittuf/gittuf/internal/cmd/trust/hooks/listhooks"
	"github.com/gittuf/gittuf/internal/cmd/trust/hooks/remove"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/spf13/cobra"
)

func New(o *persistent.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "hooks",
		Short:             "Tools to manage gittuf hooks",
		DisableAutoGenTag: true,
	}
	o.AddPersistentFlags(cmd)

	cmd.AddCommand(add.New(o))
	cmd.AddCommand(remove.New(o))
	cmd.AddCommand(listhooks.New())

	return cmd
}
