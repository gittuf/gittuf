// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package populatecache

import (
	"fmt"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/spf13/cobra"
)

type options struct{}

func (o *options) AddFlags(_ *cobra.Command) {}

func (o *options) Run(_ *cobra.Command, _ []string) error {
	repo, err := gittuf.LoadRepository()
	if err != nil {
		return err
	}

	return repo.PopulateCache()
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "populate-cache",
		Short: fmt.Sprintf("Populate persistent cache (developer mode only, set %s=1)", dev.DevModeKey),
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
