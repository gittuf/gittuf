// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package dismissgithubapproval

import (
	"github.com/gittuf/gittuf/internal/cmd/attest/github/dismissapproval"
	"github.com/gittuf/gittuf/internal/cmd/attest/persistent"
	"github.com/spf13/cobra"
)

type options struct {
	signingKey string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(
		&o.signingKey,
		"signing-key",
		"k",
		"",
		"specify key to sign attestation with",
	)
	cmd.MarkFlagRequired("signing-key") //nolint:errcheck
}

func New() *cobra.Command {
	o := &options{}
	cmd := dismissapproval.New(&persistent.Options{SigningKey: o.signingKey})
	o.AddFlags(cmd)
	cmd.Deprecated = "switch to \"gittuf attest github dismiss-approval\""
	return cmd
}
