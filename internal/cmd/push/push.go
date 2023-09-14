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

	return repo.Push(cmd.Context(), args[0], args[1:]...)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "push",
		Short: "Push repository contents to remote",
		Args:  cobra.MinimumNArgs(2), // TODO: this doesn't support -u for defaults
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
