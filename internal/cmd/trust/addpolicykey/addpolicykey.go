// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addpolicykey

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
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
		"policy key to add (path to SSH public key, \"gpg:<fingerprint>\" for GPG, or \"fulcio:<identity>::<issuer>\" for Sigstore)",
	)
	cmd.MarkFlagRequired("policy-key") //nolint:errcheck
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

	targetsKey, err := gittuf.LoadPublicKey(o.targetsKey)
	if err != nil {
		return err
	}

	opts := []trustpolicyopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}
	return repo.AddTopLevelTargetsKey(cmd.Context(), signer, targetsKey, true, opts...)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "add-policy-key",
		Short:             "Add Policy key to gittuf root of trust",
		Long:              "The 'add-policy-key' command adds a new trusted key for the primary policy file to the repository's root of trust. It is used to authorize additional keys to sign the main policy metadata.",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
