// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package github

import (
	"github.com/gittuf/gittuf/internal/cmd/attest/github/dismissapproval"
	"github.com/gittuf/gittuf/internal/cmd/attest/github/pullrequest"
	"github.com/gittuf/gittuf/internal/cmd/attest/github/recordapproval"
	"github.com/gittuf/gittuf/internal/cmd/attest/persistent"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/spf13/cobra"
)

func New(persistent *persistent.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "github",
		Short:   "Tools to attest about GitHub actions and entities",
		Long:    `The 'github' command provides tools to create attestations for actions and entities associated with GitHub, such as pull requests and approvals. It includes subcommands to record approval of a GitHub pull request. dismiss a previously recorded approval, and attest to metadata related to GitHub pull requests.`,
		PreRunE: common.CheckForSigningKeyFlag,
	}

	cmd.AddCommand(dismissapproval.New(persistent))
	cmd.AddCommand(pullrequest.New(persistent))
	cmd.AddCommand(recordapproval.New(persistent))

	return cmd
}
