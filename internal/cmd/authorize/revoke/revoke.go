// SPDX-License-Identifier: Apache-2.0

package revoke

import (
	"os"

	"github.com/gittuf/gittuf/internal/cmd/authorize/persistent"
	"github.com/gittuf/gittuf/internal/repository"
	"github.com/spf13/cobra"
)

type options struct {
	p       *persistent.Options
	refName string
	fromID  string
	toID    string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(
		&o.refName,
		"ref",
		"r",
		"",
		"Specify ref for which the authorization is to be removed",
	)
	cmd.MarkFlagRequired("ref") //nolint:errcheck

	cmd.Flags().StringVar(
		&o.fromID,
		"from-ID",
		"",
		"Specify the from ID for which the authorization is to be removed",
	)
	cmd.MarkFlagRequired("from-ID") //nolint:errcheck

	cmd.Flags().StringVar(
		&o.toID,
		"to-ID",
		"",
		"Specify the to ID for which the authorization is to be removed",
	)
	cmd.MarkFlagRequired("to-ID") //nolint:errcheck
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	keyBytes, err := os.ReadFile(o.p.SigningKey)
	if err != nil {
		return err
	}

	return repo.RemoveAuthorization(cmd.Context(), o.refName, o.fromID, o.toID, keyBytes, true)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:   "revoke",
		Short: "Revoke authorization granted for updating the state of a ref",
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
