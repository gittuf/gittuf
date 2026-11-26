// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package common //nolint:revive

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var ErrSigningKeyNotSet = errors.New("required flag \"signing-key\" not set")

// PublicKeys is a custom type to represent a list of paths
type PublicKeys []string

// String implements part of the pflag.Value interface.
func (p *PublicKeys) String() string {
	return strings.Join(*p, ", ")
}

// Set implements part of the pflag.Value interface.
func (p *PublicKeys) Set(value string) error {
	*p = append(*p, value)
	return nil
}

// Type implements part of the pflag.Value interface.
func (p *PublicKeys) Type() string {
	return "public-keys"
}

// SubcommandRequired is the Args function for commands that only group other
// commands, like "gittuf policy". Cobra only checks for an unknown subcommand
// on the root command, so without this "gittuf policy aplly" prints the help
// text and exits 0. With no arguments, the help text is still shown.
//
// Cobra checks Args before running the root command's PersistentPreRunE, so
// neither case goes through gittuf's startup code. Cobra only checks Args on a
// runnable command, so these commands also set RunE to ShowHelp.
func SubcommandRequired(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return pflag.ErrHelp
	}

	return cobra.NoArgs(cmd, args)
}

// ShowHelp prints the help text of the command. It is the RunE of commands
// that use SubcommandRequired.
func ShowHelp(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}
