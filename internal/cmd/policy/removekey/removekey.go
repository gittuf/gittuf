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
		Use:               "remove-key",
		Short:             "Remove a key from a policy file",
		Long:              `The 'remove-key' command removes the specified public key from the specified gittuf policy file. By default, the main policy file (targets) is used, which can be overridden with the '--policy-name' flag.`,
		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
