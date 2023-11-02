// SPDX-License-Identifier: Apache-2.0

package root

import (
	"github.com/gittuf/gittuf/internal/cmd/authorize"
	"github.com/gittuf/gittuf/internal/cmd/clone"
	"github.com/gittuf/gittuf/internal/cmd/policy"
	"github.com/gittuf/gittuf/internal/cmd/rsl"
	"github.com/gittuf/gittuf/internal/cmd/trust"
	"github.com/gittuf/gittuf/internal/cmd/verifycommit"
	"github.com/gittuf/gittuf/internal/cmd/verifyref"
	"github.com/gittuf/gittuf/internal/cmd/verifytag"
	"github.com/gittuf/gittuf/internal/cmd/version"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gittuf",
		Short: "A security layer for Git repositories, powered by TUF",
	}

	cmd.AddCommand(authorize.New())
	cmd.AddCommand(clone.New())
	cmd.AddCommand(trust.New())
	cmd.AddCommand(policy.New())
	cmd.AddCommand(rsl.New())
	cmd.AddCommand(verifycommit.New())
	cmd.AddCommand(verifyref.New())
	cmd.AddCommand(verifytag.New())
	cmd.AddCommand(version.New())

	return cmd
}
