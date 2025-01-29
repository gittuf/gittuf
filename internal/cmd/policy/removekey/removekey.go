// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package removekey

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/spf13/cobra"
)

type options struct {
	p            *persistent.Options
	policyName   string
	keysToRemove []string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.policyName,
		"policy-name",
		policy.TargetsRoleName,
		"name of policy file to remove key from",
	)

	cmd.Flags().StringArrayVar(
		&o.keysToRemove,
		"public-key",
		[]string{},
		"public key(s) to remove from the policy",
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

	keyIDsToRemove := []string{}
	for _, key := range o.keysToRemove {
		key, err := gittuf.LoadPublicKey(key)
		if err != nil {
			return err
		}

		keyIDsToRemove = append(keyIDsToRemove, key.ID())
	}

	return repo.RemovePrincipalFromTargets(cmd.Context(), signer, o.policyName, keyIDsToRemove, true)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "remove-key",
		Short:             "Remove a key from a policy file",
		Long:              `This command allows users to remove keys from the specified policy file. By default, the main policy file is selected. Note that the keys can be specified from disk, from the GPG keyring using the "gpg:<fingerprint>" format, or as a Sigstore identity as "fulcio:<identity>::<issuer>".`,
		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
