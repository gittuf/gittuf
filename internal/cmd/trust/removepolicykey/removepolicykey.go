// SPDX-License-Identifier: Apache-2.0

package removepolicykey

import (
	"os"
	"strings"

	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/gittuf/gittuf/internal/repository"
	"github.com/spf13/cobra"
)

type options struct {
	p            *persistent.Options
	targetsKeyID string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.targetsKeyID,
		"policy-key-ID",
		"",
		"ID of Policy key to be removed from root of trust",
	)
	cmd.MarkFlagRequired("policy-key-ID") //nolint:errcheck
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	rootKeyBytes, err := os.ReadFile(o.p.SigningKey)
	if err != nil {
		return err
	}

	return repo.RemoveTopLevelTargetsKey(cmd.Context(), rootKeyBytes, strings.ToLower(o.targetsKeyID), true)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:     "remove-policy-key",
		Short:   "Remove Policy key from gittuf root of trust",
		PreRunE: common.CheckIfSigningViable,
		RunE:    o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
