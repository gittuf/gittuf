package push

import (
	"github.com/gittuf/gittuf/internal/repository"
	"github.com/spf13/cobra"
)

type options struct{}

func (o *options) AddFlags(cmd *cobra.Command) {}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}
	return repo.Push(args[0], args[1:]...)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "push",
		Short: "Push changes and companion RSL entries to the specified remote",
		Args:  cobra.MinimumNArgs(2),
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
