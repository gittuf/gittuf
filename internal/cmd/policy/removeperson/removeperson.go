// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package removeperson

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/spf13/cobra"
)

type options struct {
	p          *persistent.Options
	policyName string
	personID   string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.policyName,
		"policy-name",
		policy.TargetsRoleName,
		"name of policy file to remove person from",
	)

	cmd.Flags().StringVar(
		&o.personID,
		"person-ID",
		"",
		"person ID",
	)
	cmd.MarkFlagRequired("person-ID") //nolint:errcheck
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
	return repo.RemovePrincipalFromTargets(cmd.Context(), signer, o.policyName, o.personID, true, opts...)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:   "remove-person",
		Short: "Remove a person from a policy file",
		Long: `The 'remove-person' command removes a trusted person from a gittuf policy file.

Users must provide the ID of the person to be removed using the --person-ID flag.

By default, the command operates on the main policy file (targets), but a different policy file can be specified with --policy-name.

This command requires a valid signing key (--signing-key) to authorize the removal.

It also supports adding an RSL (Record of State Log) entry if configured.

Use this command to revoke a person's trust and prevent them from participating in signing operations in the policy.`,

		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
