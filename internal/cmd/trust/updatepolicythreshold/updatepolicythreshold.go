// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package updatepolicythreshold

import (
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/gittuf/gittuf/internal/repository"
	"github.com/spf13/cobra"
)

type options struct {
	p         *persistent.Options
	threshold int
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().IntVar(
		&o.threshold,
		"threshold",
		-1,
		"threshold of valid signatures required for main policy",
	)
	cmd.MarkFlagRequired("threshold") //nolint:errcheck
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	signer, err := common.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	return repo.UpdateTopLevelTargetsThreshold(cmd.Context(), signer, o.threshold, true)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:     "update-policy-threshold",
		Short:   "Update Policy threshold in the gittuf root of trust",
		Long:    "This command allows users to update the threshold of valid signatures required for the policy.",
		PreRunE: common.CheckIfSigningViableWithFlag,
		RunE:    o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
