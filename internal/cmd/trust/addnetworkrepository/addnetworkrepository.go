// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addnetworkrepository

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/spf13/cobra"
)

type options struct {
	p                     *persistent.Options
	repositoryName        string
	repositoryLocation    string
	initialRootPrincipals []string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.repositoryName,
		"name",
		"",
		"name of network repository",
	)
	cmd.MarkFlagRequired("name") //nolint:errcheck

	cmd.Flags().StringVar(
		&o.repositoryLocation,
		"location",
		"",
		"location of network repository",
	)
	cmd.MarkFlagRequired("location") //nolint:errcheck

	cmd.Flags().StringArrayVar(
		&o.initialRootPrincipals,
		"initial-root-principal",
		[]string{},
		"initial root principals of network repository",
	)
	cmd.MarkFlagRequired("initial-root-principal") //nolint:errcheck
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

	initialRootPrincipals := []tuf.Principal{}
	for _, principalRef := range o.initialRootPrincipals {
		principal, err := gittuf.LoadPublicKey(principalRef)
		if err != nil {
			return err
		}
		initialRootPrincipals = append(initialRootPrincipals, principal)
	}

	opts := []trustpolicyopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}

	return repo.AddNetworkRepository(cmd.Context(), signer, o.repositoryName, o.repositoryLocation, initialRootPrincipals, true, opts...)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "add-network-repository",
		Short:             `Add a network repository`,
		Long:              `The 'add-network-repository' command registers a new network repository in the current repository's gittuf policy. You must specify the repository's name with --name, its location with --location, and one or more initial root principals with --initial-root-principal.`,
		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
