// SPDX-License-Identifier: Apache-2.0

package updatepolicythreshold

import (
	"fmt"

	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/gittuf/gittuf/internal/dev"
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
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	signer, err := common.LoadSigner(o.p.SigningKey)
	if err != nil {
		return err
	}

	return repo.UpdateTopLevelTargetsThreshold(cmd.Context(), signer, o.threshold, true)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:   "update-policy-threshold",
		Short: fmt.Sprintf("Update Policy threshold in the gittuf root of trust (developer mode only, set %s=1)", dev.DevModeKey),
		Long: fmt.Sprintf(`This command allows users to update the threshold of valid signatures required for the policy.

DO NOT USE until policy-staging is working, so that multiple developers can sequentially sign the policy metadata.
Until then, this command is available in developer mode only, set %s=1 to use.`, dev.DevModeKey),
		PreRunE: common.CheckIfSigningViableWithFlag,
		RunE:    o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
