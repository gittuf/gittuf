// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package hat

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	attestopts "github.com/gittuf/gittuf/experimental/gittuf/options/attest"
	"github.com/gittuf/gittuf/internal/cmd/attest/persistent"
	"github.com/spf13/cobra"
)

type options struct {
	p         *persistent.Options
	targetRef string
	teamID    string
}

func (o *options) AddFlag(cmd *cobra.Command) {
	cmd.Flags().StringVarP(
		&o.targetRef,
		"target-ref",
		"",
		"",
		"ref that the commit in question was made on",
	)

	cmd.Flags().StringVarP(
		&o.teamID,
		"team-ID",
		"",
		"",
		"team ID to perform the operation on behalf of",
	)
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

	return repo.TODO
}
