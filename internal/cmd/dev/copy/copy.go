// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package copy

import (
	"fmt"
	"os"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/spf13/cobra"
)

type options struct {
	localRef         string
	localDir         string
	remoteRepository string
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

	tmpDir, err := os.MkdirTemp("", "gittuf-copy-remote")
	if err != nil {
		return err
	}
	remoteRepository, err := gitinterface.CloneAndFetchRepository(o.remoteRepository, tmpDir, "", nil)
	if err != nil {
		return err
	}

	remoteTip, err := remoteRepository.GetReference("HEAD")
	if err != nil {
		return err
	}

	return repo.GetGitRepository().CopyTreeFromRepositoryToPathInRef(remoteRepository, remoteTip, map[string]string{o.localRef: o.localDir})
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "copy",
		Short:             fmt.Sprintf("Copy contents of remote repository to local repository (developer mode only, set %s=1)", dev.DevModeKey),
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
