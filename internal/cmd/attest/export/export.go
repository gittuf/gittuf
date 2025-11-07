// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package export

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd/attest/persistent"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/spf13/cobra"
)

type options struct {
	target                 string
	attestationsExportPath string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(
		&o.target,
		"target",
		"t",
		"",
		"commit to export related attestations from",
	)
	cmd.MarkFlagRequired("target")

	cmd.Flags().StringVar(
		&o.attestationsExportPath,
		"export-attestations",
		"",
		"path to export attestations used in verification",
	)
	cmd.MarkFlagRequired("export-attestations")
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	hash, err := gitinterface.NewHash(o.target)
	if err != nil {
		return err
	}

	return repo.ExportAttestationsForRevision(cmd.Context(), hash, o.attestationsExportPath)
}

func New(_ *persistent.Options) *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "export",
		Short:             "Export the attestations for a certain revision of the repository",
		Long:              "Export the attestations related to a certain revision of the repository. This is useful for scenarios where manual inspection of metadata is desired, or for proving compliance with standards such as the SLSA Source Track.",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}

	o.AddFlags(cmd)

	return cmd
}
