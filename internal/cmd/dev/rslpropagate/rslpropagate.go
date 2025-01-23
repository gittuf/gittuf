// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package rslpropagate

import (
	"fmt"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/dev"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	"github.com/spf13/cobra"
)

type options struct {
	localRef         string
	localDir         string
	remoteRepository string
	remoteRef        string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.localRef,
		"local-ref",
		"",
		"local reference to update with contents of remote repository",
	)
	cmd.MarkFlagRequired("local-ref") //nolint:errcheck

	cmd.Flags().StringVar(
		&o.localDir,
		"local-directory",
		"",
		"directory to store remote repository contents in local repository",
	)
	cmd.MarkFlagRequired("local-directory") //nolint:errcheck

	cmd.Flags().StringVar(
		&o.remoteRepository,
		"remote-repository",
		"",
		"location of remote repository",
	)
	cmd.MarkFlagRequired("remote-repository") //nolint:errcheck
}

func (o *options) Run(_ *cobra.Command, _ []string) error {
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	repo, err := gittuf.LoadRepository()
	if err != nil {
		return err
	}

	details := []*tufv01.Propagation{{
		UpstreamRepository:  o.remoteRepository,
		UpstreamReference:   o.remoteRef,
		DownstreamReference: o.localRef,
		DownstreamPath:      o.localDir,
	}}

	return repo.PropagateChanges(details, true)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "rsl-propagate",
		Short:             fmt.Sprintf("Copy contents of remote repository to local repository (developer mode only, set %s=1)", dev.DevModeKey),
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
