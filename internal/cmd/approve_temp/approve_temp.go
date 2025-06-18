// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package approve_temp

import (
	"fmt"

	"github.com/spf13/cobra"
)

// New returns the cobra.Command for the `approve-temp` subcommand.
func New() *cobra.Command {
	var action string
	var duration string

	cmd := &cobra.Command{
		Use:   "approve-temp",
		Short: "Generate a temporary approval attestation",
		Long: `Creates a signed, time-limited approval attestation for a PR or commit.
Currently this is just a stub; full implementation will be added in later phases.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Stub: would approve action '%s' for duration '%s'\n", action, duration)
		},
	}

	cmd.Flags().StringVarP(&action, "action", "a", "", "Target action to approve (e.g. PR-123, commit SHA)")
	cmd.Flags().StringVarP(&duration, "duration", "d", "24h", "Approval validity duration (e.g. 2h, 1d)")
	cmd.MarkFlagRequired("action")

	return cmd
}
