// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package removeglobalrule

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/spf13/cobra"
)

type options struct {
	p        *persistent.Options
	ruleName string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.ruleName,
		"rule-name",
		"",
		"name of rule",
	)
	cmd.MarkFlagRequired("rule-name") //nolint:errcheck
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
	return repo.RemoveGlobalRule(cmd.Context(), signer, o.ruleName, true, opts...)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}

	cmd := &cobra.Command{
		Use:               "remove-global-rule",
		Short:             `Remove a global rule from root of trust`,
		Long:              "The 'remove-global-rule' command removes an existing global rule from the repository's root of trust. It is used to lift a repository-wide constraint that is no longer needed.",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
