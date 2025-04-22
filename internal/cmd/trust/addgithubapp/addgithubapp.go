// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addgithubapp

import (
	"fmt"

	"github.com/gittuf/gittuf/experimental/gittuf"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/spf13/cobra"
)

type options struct {
	p       *persistent.Options
	appName string
	appKey  string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.appName,
		"app-name",
		tuf.GitHubAppRoleName,
		"name of app to add to root of trust",
	)

	cmd.Flags().StringVar(
		&o.appKey,
		"app-key",
		"",
		fmt.Sprintf("app key to add to root of trust (path to SSH key, \"%s<identity>::<issuer>\" for Sigstore, \"%s<fingerprint>\" for GPG key)", gittuf.FulcioPrefix, gittuf.GPGKeyPrefix),
	)
	cmd.MarkFlagRequired("app-key") //nolint:errcheck
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

	appKey, err := gittuf.LoadPublicKey(o.appKey)
	if err != nil {
		return err
	}

	opts := []trustpolicyopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}
	return repo.AddGitHubApp(cmd.Context(), signer, o.appName, appKey, true, opts...)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "add-github-app",
		Short:             "Add GitHub app to gittuf root of trust",
		Long:              `This command allows users to add a trusted key for the special GitHub app role. This key is used to verify signatures on GitHub pull request approval attestations. Note that authorized keys can be specified from disk, from the GPG keyring using the "gpg:<fingerprint>" format, or as a Sigstore identity as "fulcio:<identity>::<issuer>".`,
		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
