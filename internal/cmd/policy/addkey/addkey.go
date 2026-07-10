// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addkey

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/spf13/cobra"
)

type options struct {
	p              *persistent.Options
	policyName     string
	authorizedKeys []string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.policyName,
		"policy-name",
		policy.TargetsRoleName,
		"name of policy file to add key to",
	)

	cmd.Flags().StringArrayVar(
		&o.authorizedKeys,
		"public-key",
		[]string{},
		"authorized public key (path to SSH public key, \"gpg:<fingerprint>\" for GPG, or \"fulcio:<identity>::<issuer>\" for Sigstore)",
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

	authorizedKeys := []tuf.Principal{}
	for _, key := range o.authorizedKeys {
		key, err := gittuf.LoadPublicKey(key)
		if err != nil {
			return err
		}

		authorizedKeys = append(authorizedKeys, key)
	}

	opts := []trustpolicyopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}
	return repo.AddPrincipalToTargets(cmd.Context(), signer, o.policyName, authorizedKeys, true, opts...)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "add-key",
		Short:             "Add a trusted key to a policy file",
		Long:              "The 'add-key' command adds one or more trusted public keys to a gittuf policy file. It is used to make keys available for use in policy rules. A key must be added to at least one rule before it can be used to authorize changes.",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
