// SPDX-License-Identifier: Apache-2.0

package addpolicykey

import (
	"os"

	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/gittuf/gittuf/internal/repository"
	"github.com/spf13/cobra"
)

type options struct {
	p          *persistent.Options
	targetsKey string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.targetsKey,
		"policy-key",
		"",
		"policy key to add to root of trust",
	)
	cmd.MarkFlagRequired("policy-key") //nolint:errcheck
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	rootKeyBytes, err := os.ReadFile(o.p.SigningKey)
	if err != nil {
		return err
	}

	targetsKeyBytes, err := common.ReadKeyBytes(o.targetsKey)
	if err != nil {
		return err
	}

	return repo.AddTopLevelTargetsKey(cmd.Context(), rootKeyBytes, targetsKeyBytes, true)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:   "add-policy-key",
		Short: "Add Policy key to gittuf root of trust",
		Long:  `This command allows users to add a new trusted key for the main policy file. Note that authorized keys can be specified from disk using the custom securesystemslib format, from the GPG keyring using the "gpg:<fingerprint>" format, or as a Sigstore identity as "fulcio:<identity>::<issuer>".`,
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
