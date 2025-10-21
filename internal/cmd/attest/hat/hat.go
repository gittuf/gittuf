// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package hat

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	attestopts "github.com/gittuf/gittuf/experimental/gittuf/options/attest"
	"github.com/gittuf/gittuf/internal/cmd/attest/persistent"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/spf13/cobra"
)

type options struct {
	p         *persistent.Options
	targetRef string
	teamID    string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(
		&o.targetRef,
		"target-ref",
		"",
		"",
		"ref that the commit in question was made on",
	)
	cmd.MarkFlagRequired("target-ref")

	cmd.Flags().StringVarP(
		&o.teamID,
		"team-ID",
		"",
		"",
		"team ID to perform the operation on behalf of",
	)
	cmd.MarkFlagRequired("team-ID")
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	opts := []attestopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, attestopts.WithRSLEntry())
	}

	return repo.AddHatAttestation(cmd.Context(), signer, o.targetRef, o.teamID, true, opts...)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "hat",
		Short:             "Add (todo: or revoke) hat attestation",
		Long:              "This command creates a hat attestation that attests the hat a user has worn for a commit or tag.",
		Args:              cobra.MinimumNArgs(2),
		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
