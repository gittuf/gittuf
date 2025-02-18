// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addgithubapproval

import (
	"github.com/gittuf/gittuf/internal/cmd/attest/github/recordapproval"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	cmd := recordapproval.New()
	cmd.Deprecated = "switch to \"gittuf attest github record-approval\""
	return cmd
}
