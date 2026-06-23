// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package attestgithub

import (
	"github.com/gittuf/gittuf/internal/cmd/attest/github/pullrequest"
	"github.com/gittuf/gittuf/internal/cmd/attest/persistent"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	p := &persistent.Options{WithRSLEntry: true}
	cmd := pullrequest.New(p)
	cmd.Flags().StringVarP(
		&p.SigningKey,
		"signing-key",
		"k",
		"",
		"specify key to sign attestation with",
	)
	cmd.MarkFlagRequired("signing-key") //nolint:errcheck
	cmd.Deprecated = "switch to \"gittuf attest github pull-request\""
	return cmd
}
