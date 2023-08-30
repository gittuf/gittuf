package annotate

import (
	"github.com/gittuf/gittuf/internal/repository"
	"github.com/spf13/cobra"
)

type options struct {
	skip    bool
	message string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(
		&o.skip,
		"skip",
		"s",
		false,
		"mark annotated entries as to be skipped",
	)

	cmd.Flags().StringVarP(
		&o.message,
		"message",
		"m",
		"",
		"annotation message",
	)
	cmd.MarkFlagRequired("message") //nolint:errcheck
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	return repo.RecordRSLAnnotation(args, o.skip, o.message, true)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "annotate",
		Short: "Annotate prior RSL entries",
		Args:  cobra.MinimumNArgs(1),
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
