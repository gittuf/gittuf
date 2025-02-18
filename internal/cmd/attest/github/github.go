// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package github

import (
	"github.com/gittuf/gittuf/internal/cmd/attest/github/dismissapproval"
	"github.com/gittuf/gittuf/internal/cmd/attest/github/pullrequest"
	"github.com/gittuf/gittuf/internal/cmd/attest/github/recordapproval"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "github",
		Short: "Tools to attest about GitHub actions and entities",
	}

	cmd.AddCommand(dismissapproval.New())
	cmd.AddCommand(pullrequest.New())
	cmd.AddCommand(recordapproval.New())

	return cmd
}
