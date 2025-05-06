// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package removepolicykey

import (
	"strings"

	"github.com/gittuf/gittuf/experimental/gittuf"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
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
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	opts := []trustpolicyopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}
	return repo.RemoveTopLevelTargetsKey(cmd.Context(), signer, strings.ToLower(o.targetsKeyID), true, opts...)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "remove-policy-key",
		Short:             "Remove Policy key from gittuf root of trust",
		Long:              "This command allows users to remove a policy key from the gittuf root of trust. The policy key ID must be specified.",
		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
