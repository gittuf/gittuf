// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addkey

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
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
	repo, err := gittuf.LoadRepository()
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

	return repo.AddPrincipalToTargets(cmd.Context(), signer, o.policyName, authorizedKeys, true)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "add-key",
		Short:             "Add a trusted key to a policy file",
		Long:              `This command allows users to add trusted keys to the specified policy file. By default, the main policy file is selected. Note that the keys can be specified from disk, from the GPG keyring using the "gpg:<fingerprint>" format, or as a Sigstore identity as "fulcio:<identity>::<issuer>".`,
		PreRunE:           common.CheckIfSigningViableWithFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
