// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package init

import (
	"log/slog"

	"github.com/gittuf/gittuf/experimental/gittuf"
	rootopts "github.com/gittuf/gittuf/experimental/gittuf/options/root"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/spf13/cobra"
)

type options struct {
	p        *persistent.Options
	location string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.location,
		"location",
		"",
		"location of repository",
	)
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

	opts := []rootopts.Option{rootopts.WithRepositoryLocation(o.location)}
	if o.p.WithRSLEntry {
		opts = append(opts, rootopts.WithRSLEntry())
	}
	if err := repo.InitializeRoot(cmd.Context(), signer, true, opts...); err != nil {
		return err
	}

	// Enable and populate the persistent cache by default after trust init.
	// Non-fatal - if it fails, we log and continue.
	if repo.IsCacheEnabled() {
		slog.Debug("Enabling persistent cache after trust init...")
		if err := repo.EnableCacheConfig(); err != nil {
			slog.Debug("Warning: could not enable cache config", "error", err)
		} else if err := repo.PopulateCache(); err != nil {
			slog.Debug("Warning: could not populate cache", "error", err)
		}
	}

	return nil
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "init",
		Short:             "Initialize gittuf root of trust for repository",
		Long:              "The 'init' command initializes the gittuf root of trust for a repository. It is used to initialize gittuf metadata with the user invoking the command trusted for root operations, and must be run before any other gittuf metadata command can be run.",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
