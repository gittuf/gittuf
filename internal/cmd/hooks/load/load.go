// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package load

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
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

	return repo.LoadHooks()
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "load",
		Short:             "load hooks files from metadata",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}

	return cmd
}
