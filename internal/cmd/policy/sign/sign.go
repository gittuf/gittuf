// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package sign

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
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.policyName,
		"policy-name",
		policy.TargetsRoleName,
		"name of policy file to sign",
	)
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
	return repo.SignTargets(cmd.Context(), signer, o.policyName, true, opts...)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:   "sign",
		Short: "Sign policy file",
		Long: `The 'sign' command allows a user to add their cryptographic signature to a gittuf policy file, ensuring trust and integrity.

By default, it operates on the main policy file (targets), but a specific policy file can be provided using the --policy-name flag.

This command requires a valid signing key provided via the --signing-key flag.

If RSL (Record of State Log) tracking is enabled with --rsl-entry, an entry is added to maintain a verifiable audit trail of the signature.

Use this command to approve policy changes and contribute a valid signature as required by the threshold.`,

		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
