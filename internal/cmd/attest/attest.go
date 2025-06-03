// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package attest

import (
	"github.com/gittuf/gittuf/internal/cmd/attest/apply"
	"github.com/gittuf/gittuf/internal/cmd/attest/authorize"
	"github.com/gittuf/gittuf/internal/cmd/attest/github"
	"github.com/gittuf/gittuf/internal/cmd/attest/persistent"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	o := &persistent.Options{}
	cmd := &cobra.Command{
		Use:               "attest",
		Short:             "Tools for attesting to code contributions",
		Long:              `The 'attest' command provides tools for attesting to code contributions. It includes subcommands to apply attestations, authorize contributors, and integrate GitHub-based attestations.`,
		DisableAutoGenTag: true,
	}
	o.AddPersistentFlags(cmd)

	cmd.AddCommand(apply.New())
	cmd.AddCommand(authorize.New(o))
	cmd.AddCommand(github.New(o))

	return cmd
}
