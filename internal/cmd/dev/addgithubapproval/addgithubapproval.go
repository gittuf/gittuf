// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addgithubapproval

import (
	"github.com/gittuf/gittuf/internal/cmd/attest/github/recordapproval"
	"github.com/gittuf/gittuf/internal/cmd/attest/persistent"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	p := &persistent.Options{WithRSLEntry: true}
	cmd := recordapproval.New(p)
	cmd.Flags().StringVarP(
		&p.SigningKey,
		"signing-key",
		"k",
		"",
		"specify key to sign attestation with",
	)
	cmd.MarkFlagRequired("signing-key") //nolint:errcheck
	cmd.Deprecated = "switch to \"gittuf attest github record-approval\""
	return cmd
}
