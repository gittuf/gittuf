// SPDX-License-Identifier: Apache-2.0

package dev

import (
	"fmt"

	"github.com/gittuf/gittuf/internal/cmd/authorize"
	"github.com/gittuf/gittuf/internal/cmd/rsl/rslrecordat"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dev",
		Short: "Developer mode commands",
		Long:  fmt.Sprintf("These commands are meant to be used to aid gittuf development, and are not expected to be used during standard workflows. If used, they can undermine repository security. To proceed, set %s=1.", dev.DevModeKey),
	}

	cmd.AddCommand(authorize.New())
	cmd.AddCommand(rslrecordat.New())

	return cmd
}
