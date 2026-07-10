// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package removepolicykey

import (
	"strings"

	"github.com/gittuf/gittuf/experimental/gittuf"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
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
		"ID of the policy key to remove from the root of trust",
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
		Long:              "The 'remove-policy-key' command removes a trusted key for the primary policy file from the repository's root of trust. It is used to revoke a key's authorization to sign policy metadata.",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
