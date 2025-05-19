// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addkey

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/cmd/common"
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
		"authorized public key",
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
		Use:   "add-key",
		Short: "Add a trusted key to a policy file",
		Long: `The 'add-key' command adds one or more trusted public keys to a gittuf trust policy file.

This command is used to define which keys are authorized to sign commits, references, or policy changes
according to the repository's trust model. It supports various key formats and sources, including:

- Local PEM-encoded public key files
- GPG keys from a local keyring using the "gpg:<fingerprint>" syntax
- Sigstore identities using the "fulcio:<identity>::<issuer>" syntax

By default, the main policy file (targets) is used, but you can override this with the --policy-name flag.
This command also supports generating Record of State Log (RSL) entries when the --rsl flag is enabled.

Requirements:
- A valid signing key must be provided via --signing-key
- At least one public key must be supplied using --public-key

Usage:
  gittuf policy add-key --public-key <path|gpg:fingerprint|fulcio:identity::issuer> [--policy-name <name>] [--signing-key <path>]`,
		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
