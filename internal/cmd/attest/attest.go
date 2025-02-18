// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package attest

import (
	"github.com/gittuf/gittuf/internal/cmd/attest/authorize"
	"github.com/gittuf/gittuf/internal/cmd/attest/github"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "attest",
		Short:             "Tools for attesting to code contributions",
		DisableAutoGenTag: true,
	}

	cmd.AddCommand(authorize.New())
	cmd.AddCommand(github.New())

	return cmd
}
