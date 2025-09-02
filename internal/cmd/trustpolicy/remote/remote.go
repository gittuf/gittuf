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
		Use:   "remote",
		Short: "Tools for managing remote policies",
		Long: `The 'remote' command provides tools for synchronizing trust policies
between the local repository and remotes. Use 'pull' to fetch policy updates
from a remote, or 'push' to upload the local policy to a remote.`,

		DisableAutoGenTag: true,
	}

	cmd.AddCommand(pull.New())
	cmd.AddCommand(push.New())

	return cmd
}
