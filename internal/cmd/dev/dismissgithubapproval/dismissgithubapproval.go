// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package dismissgithubapproval

import (
	"github.com/gittuf/gittuf/internal/cmd/attest/github/dismissapproval"
	"github.com/gittuf/gittuf/internal/cmd/attest/persistent"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	p := &persistent.Options{WithRSLEntry: true}
	cmd := dismissapproval.New(p)
	cmd.Flags().StringVarP(
		&p.SigningKey,
		"signing-key",
		"k",
		"",
		"specify key to sign attestation with",
	)
	cmd.MarkFlagRequired("signing-key") //nolint:errcheck
	cmd.Deprecated = "switch to \"gittuf attest github dismiss-approval\""
	return cmd
}
