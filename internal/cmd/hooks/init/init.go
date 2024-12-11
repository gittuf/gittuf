// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package init

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/spf13/cobra"
)

type options struct {
	p              *persistent.Options
	policyName     string
	authorizedKeys []string
	threshold      int
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

	// initialize policy
	// add rule for protecting refs/gittuf/hooks

	return repo.InitializeHooks(cmd.Context(), signer)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "init",
		Short:             "Initialize hooks ref",
		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}

	return cmd
}
