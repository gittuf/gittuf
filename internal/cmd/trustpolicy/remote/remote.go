// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package remote

import (
	"github.com/gittuf/gittuf/internal/cmd/trustpolicy/remote/pull"
	"github.com/gittuf/gittuf/internal/cmd/trustpolicy/remote/push"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "remote",
		Short:             "Tools for managing remote policies",
		Long:              "The 'remote' subcommand provides tools for pulling and pushing gittuf policy to and from remote repositories.",
		DisableAutoGenTag: true,
	}

	cmd.AddCommand(pull.New())
	cmd.AddCommand(push.New())

	return cmd
}
