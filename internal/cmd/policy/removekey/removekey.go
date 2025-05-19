// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package removekey

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/spf13/cobra"
)

type options struct {
	p           *persistent.Options
	policyName  string
	keyToRemove string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.policyName,
		"policy-name",
		policy.TargetsRoleName,
		"name of policy file to remove key from",
	)

	cmd.Flags().StringVar(
		&o.keyToRemove,
		"public-key",
		"",
		"public key ID to remove from the policy",
	)
	cmd.MarkFlagRequired("public-key") //nolint:errcheck
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
	return repo.RemovePrincipalFromTargets(cmd.Context(), signer, o.policyName, o.keyToRemove, true, opts...)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:   "remove-key",
		Short: "Remove a key from a policy file",
		Long: `The 'remove-key' command removes a specified public key from a gittuf policy file.

Users must provide the public key ID to be removed using the --public-key flag.

By default, the command operates on the main policy file (targets), but a different policy file can be specified using --policy-name.

This command requires a valid signing key (--signing-key) to authorize the change.

It also supports adding an RSL (Record of State Log) entry if configured.

Use this command to revoke trust from a public key in the policy, thereby preventing it from participating in future signing operations.`,

		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
