package verifyref

import (
	"context"

	"github.com/adityasaky/gittuf/internal/repository"
	"github.com/spf13/cobra"
)

type options struct {
	full bool
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(
		&o.full,
		"full",
		"f",
		false,
		"perform verification from the start of the RSL",
	)
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}
	return repo.VerifyRef(context.Background(), args[0], o.full)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "verify-ref",
		Short: "Tools for verifying gittuf policies",
		Args:  cobra.ExactArgs(1),
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
