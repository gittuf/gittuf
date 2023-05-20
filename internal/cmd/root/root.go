package root

import (
	"github.com/adityasaky/gittuf/internal/cmd/policy"
	"github.com/adityasaky/gittuf/internal/cmd/trust"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gittuf",
		Short: "A security layer for Git repositories, powered by TUF",
	}

	cmd.AddCommand(trust.New())
	cmd.AddCommand(policy.New())

	return cmd
}
