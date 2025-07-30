// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package common //nolint:revive

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"
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

// CheckForSigningKeyFlag checks if a signing key was specified via the
// "signing-key" flag
func CheckForSigningKeyFlag(cmd *cobra.Command, _ []string) error {
	signingKeyFlag := cmd.Flags().Lookup("signing-key")

	// Check if a signing key was specified via the "signing-key" flag
	if signingKeyFlag.Value.String() == "" {
		return ErrSigningKeyNotSet
	}

	return nil
}
