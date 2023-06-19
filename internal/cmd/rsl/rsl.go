package rsl

import (
	"github.com/adityasaky/gittuf/internal/cmd/rsl/record"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rsl",
		Short: "Tools to manage the repository's reference state log",
	}

	cmd.AddCommand(record.New())

	return cmd
}
