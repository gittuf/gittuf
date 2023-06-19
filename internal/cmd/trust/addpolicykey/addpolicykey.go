package addpolicykey

import (
	"context"
	"os"

	"github.com/adityasaky/gittuf/internal/cmd/common"
	"github.com/adityasaky/gittuf/internal/cmd/trust/persistent"
	"github.com/adityasaky/gittuf/internal/repository"
	"github.com/spf13/cobra"
)

type options struct {
	p          *persistent.Options
	targetsKey string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.targetsKey,
		"policy-key",
		"",
		"policy key to add to root of trust",
	)
	cmd.MarkFlagRequired("policy-key") //nolint:errcheck
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	rootKeyBytes, err := os.ReadFile(o.p.SigningKey)
	if err != nil {
		return err
	}

	targetsKeyBytes, err := common.ReadKeyBytes(o.targetsKey)
	if err != nil {
		return err
	}

	return repo.AddTopLevelTargetsKey(context.Background(), rootKeyBytes, targetsKeyBytes, true)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:   "add-policy-key",
		Short: "Add Policy key to gittuf root of trust",
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
