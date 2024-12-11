// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package apply

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/spf13/cobra"
)

type options struct {
	p *persistent.Options
}

func (o *options) Run(cmd *cobra.Command, _ []string) (err error) {
	repo, err := gittuf.LoadRepository()
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	return repo.ApplyHooks(cmd.Context(), signer)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "apply",
		Short:             "secure hooks metadata file by embedding and committing with Targets",
		RunE:              o.Run,
		PreRunE:           common.CheckForSigningKeyFlag,
		DisableAutoGenTag: true,
	}

	return cmd
}
