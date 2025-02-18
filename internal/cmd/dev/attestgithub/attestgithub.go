// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package attestgithub

import (
	"github.com/gittuf/gittuf/internal/cmd/attest/github/pullrequest"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	cmd := pullrequest.New()
	cmd.Deprecated = "switch to \"gittuf attest github pull-request\""
	return cmd
}
