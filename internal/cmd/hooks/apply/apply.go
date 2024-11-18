// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package apply

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

	return repo.ApplyHooks()
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "apply",
		Short:             "secure hooks metadata file by embedding and committing with Targets",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}

	return cmd
}
