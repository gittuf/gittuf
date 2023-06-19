package verify

import (
	"context"

	"github.com/adityasaky/gittuf/internal/repository"
	"github.com/spf13/cobra"
)

type options struct{}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}
	return repo.Verify(context.Background(), args[0])
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Tools for verifying gittuf policies",
		Args:  cobra.ExactArgs(1),
		RunE:  o.Run,
	}

	return cmd
}
