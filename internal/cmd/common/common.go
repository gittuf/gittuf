// SPDX-License-Identifier: Apache-2.0

package common

import (
	"fmt"
	"strings"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/spf13/cobra"
)

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

// CheckIfSigningViableWithFlag checks if a signing key was specified via the
// "signing-key" flag, and then calls CheckIfSigningViable
func CheckIfSigningViableWithFlag(cmd *cobra.Command, _ []string) error {
	signingKeyFlag := cmd.Flags().Lookup("signing-key")

	// Check if a signing key was specified via the "signing-key" flag
	if signingKeyFlag.Value.String() == "" {
		return fmt.Errorf("required flag \"signing-key\" not set")
	}

	return CheckIfSigningViable(cmd, nil)
}

// CheckIfSigningViable checks if we are able to sign RSL entries given the
// current environment
func CheckIfSigningViable(_ *cobra.Command, _ []string) error {
	repo, err := gitinterface.LoadRepository()
	if err != nil {
		return err
	}

	return repo.CanSign()
}
