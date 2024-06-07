// SPDX-License-Identifier: Apache-2.0

package addgithubappkey

import (
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/gittuf/gittuf/internal/repository"
	"github.com/spf13/cobra"
)

type options struct {
	p      *persistent.Options
	appKey string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.appKey,
		"app-key",
		"",
		"app key to add to root of trust",
	)
	cmd.MarkFlagRequired("app-key") //nolint:errcheck
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	signer, err := common.LoadSigner(o.p.SigningKey)
	if err != nil {
		return err
	}

	appKey, err := common.LoadPublicKey(o.appKey)
	if err != nil {
		return err
	}

	return repo.AddGitHubAppKey(cmd.Context(), signer, appKey, true)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "add-github-app-key",
		Short:             "Add GitHub app key to gittuf root of trust",
		Long:              `This command allows users to add a trusted key for the special GitHub app role. Note that authorized keys can be specified from disk, from the GPG keyring using the "gpg:<fingerprint>" format, or as a Sigstore identity as "fulcio:<identity>::<issuer>".`,
		PreRunE:           common.CheckIfSigningViableWithFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
