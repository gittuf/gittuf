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
		Long: fmt.Sprintf(`The 'populate-cache' command generates and populates the local persistent cache for a gittuf repository.

This command is intended for development and debugging purposes. It may be useful when inspecting or modifying the cache layer during the development of gittuf internals.

Warning: This command bypasses certain security checks and is not safe for production use. It requires developer mode to be enabled by setting the environment variable %s=1.`, dev.DevModeKey),
		RunE: o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
