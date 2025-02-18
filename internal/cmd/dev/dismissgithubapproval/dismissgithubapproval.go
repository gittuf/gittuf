// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package dismissgithubapproval

import (
	"github.com/gittuf/gittuf/internal/cmd/attest/github/dismissapproval"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	cmd := dismissapproval.New()
	cmd.Deprecated = "switch to \"gittuf attest github dismiss-approval\""
	return cmd
}
