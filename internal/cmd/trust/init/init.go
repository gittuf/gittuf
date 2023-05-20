package init

import (
	"context"
	"os"

	"github.com/adityasaky/gittuf/internal/cmd/trust/persistent"
	"github.com/adityasaky/gittuf/internal/repository"
	"github.com/spf13/cobra"
)

type options struct {
	p *persistent.Options
}

func (o *options) AddFlags(cmd *cobra.Command) {
	// This method currently does nothing but maintaining an empty body for
	// consistency.
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	keyBytes, err := os.ReadFile(o.p.SigningKey)
	if err != nil {
		return err
	}

	return repo.InitializeRoot(context.Background(), keyBytes, true)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize gittuf root of trust for repository",
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
