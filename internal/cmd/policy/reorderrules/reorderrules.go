// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package reorderrules

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/spf13/cobra"
)

type options struct {
	p          *persistent.Options
	policyName string
	ruleNames  []string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.policyName,
		"policy-name",
		"targets",
		"name of policy file to reorder rules in",
	)
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	o.ruleNames = append(o.ruleNames, args...)

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
	err = repo.ReorderDelegations(cmd.Context(), signer, o.policyName, o.ruleNames, true, opts...)
	return err
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:   "reorder-rules",
		Short: "Reorder rules in the specified policy file",
		Long: `The 'reorder-rules' command allows users to reorder the delegation rules within a gittuf policy file.

Users specify the new order of rules by passing the rule names as command-line arguments in the desired sequence, starting from the first to the last.

Rule names containing spaces should be enclosed in quotes.

By default, this command operates on the main policy file (targets), but a different policy file can be specified with --policy-name.

A valid signing key (--signing-key) is required to authorize this operation.

This command also supports adding an RSL (Record of State Log) entry if enabled.

Use this command to update the priority or evaluation order of rules in your policy.`,

		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}

	o.AddFlags(cmd)

	return cmd
}
