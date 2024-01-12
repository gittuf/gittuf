// SPDX-License-Identifier: Apache-2.0

package addkey

import (
	"os"

	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/repository"
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
		"policy file to add rule to",
	)

	cmd.Flags().StringArrayVar(
		&o.authorizedKeys,
		"authorize-key",
		[]string{},
		"authorized public key for rule",
	)
	cmd.MarkFlagRequired("authorize-key") //nolint:errcheck
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	keyBytes, err := os.ReadFile(o.p.SigningKey)
	if err != nil {
		return err
	}
	signer, err := common.LoadSigner(keyBytes)
	if err != nil {
		return err
	}

	authorizedKeys := []*tuf.Key{}
	for _, key := range o.authorizedKeys {
		key, err := common.LoadPublicKey(key)
		if err != nil {
			return err
		}

		authorizedKeys = append(authorizedKeys, key)
	}

	return repo.AddKeyToTargets(cmd.Context(), signer, o.policyName, authorizedKeys, true)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "add-key",
		Short:             "Add a trusted key to a policy file",
		Long:              `This command allows users to add a trusted key to the specified policy file. By default, the main policy file is selected. Note that the keys can be specified from disk, from the GPG keyring using the "gpg:<fingerprint>" format, or as a Sigstore identity as "fulcio:<identity>::<issuer>".`,
		PreRunE:           common.CheckIfSigningViable,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
