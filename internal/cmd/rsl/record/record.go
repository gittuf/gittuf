package record

import (
	"github.com/adityasaky/gittuf/internal/repository"
	"github.com/spf13/cobra"
)

type options struct{}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	return repo.RecordRSLEntryForReference(args[0], true)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "record",
		Short: "Record latest state of a Git reference in the RSL",
		Args:  cobra.ExactArgs(1),
		RunE:  o.Run,
	}

	return cmd
}
