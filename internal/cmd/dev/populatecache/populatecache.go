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
	repo, err := gittuf.LoadRepository(".")
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
		Long:  fmt.Sprintf(`The 'populate-cache' command generates and populates the local persistent cache for a gittuf repository, intended to improve performance of gittuf operations. This cache is local-only and is not synchronzied with the remote. It requires developer mode to be enabled by setting the environment variable %s=1.`, dev.DevModeKey),
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
