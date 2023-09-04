package clone

import (
	"context"
	"strings"

	"github.com/gittuf/gittuf/internal/repository"
	"github.com/spf13/cobra"
)

type options struct {
	defaultRef string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(
		&o.defaultRef,
		"branch",
		"b",
		"refs/heads/main",
		"Specify branch to check out",
	)
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	var dir string
	if len(args) > 1 {
		dir = args[1]
	} else {
		sp := strings.Split(args[0], "/")
		dir = sp[len(sp)-1]
	}
	return repository.Clone(context.Background(), args[0], dir, o.defaultRef)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "clone",
		Short: "Clone repository and its gittuf references",
		Args:  cobra.MinimumNArgs(1),
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
