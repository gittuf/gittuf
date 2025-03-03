// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package persistent

import "github.com/spf13/cobra"

type Options struct {
	SigningKey   string
	WithRSLEntry bool
}

func (o *Options) AddPersistentFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(
		&o.SigningKey,
		"signing-key",
		"k",
		"",
		"signing key to use to sign policy file",
	)

	cmd.PersistentFlags().BoolVar(
		&o.WithRSLEntry,
		"create-rsl-entry",
		false,
		"create RSL entry for policy change immediately (note: the RSL will not be synced with the remote)",
	)
}
