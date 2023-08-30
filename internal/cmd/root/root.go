package root

import (
	"github.com/gittuf/gittuf/internal/cmd/policy"
	"github.com/gittuf/gittuf/internal/cmd/rsl"
	"github.com/gittuf/gittuf/internal/cmd/trust"
	"github.com/gittuf/gittuf/internal/cmd/verifyref"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gittuf",
		Short: "A security layer for Git repositories, powered by TUF",
	}

	cmd.AddCommand(trust.New())
	cmd.AddCommand(policy.New())
	cmd.AddCommand(rsl.New())
	cmd.AddCommand(verifyref.New())

	return cmd
}
