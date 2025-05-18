// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addperson

import (
	"fmt"
	"strings"

	"github.com/gittuf/gittuf/experimental/gittuf"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
	"github.com/spf13/cobra"
)

type options struct {
	p                    *persistent.Options
	policyName           string
	personID             string
	publicKeys           []string
	associatedIdentities []string
	customMetadata       []string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.policyName,
		"policy-name",
		policy.TargetsRoleName,
		"name of policy file to add key to",
	)

	cmd.Flags().StringVar(
		&o.personID,
		"person-ID",
		"",
		"person ID",
	)
	cmd.MarkFlagRequired("person-ID") //nolint:errcheck

	cmd.Flags().StringArrayVar(
		&o.publicKeys,
		"public-key",
		[]string{},
		"authorized public key for person",
	)
	cmd.MarkFlagRequired("authorize-key") //nolint:errcheck

	cmd.Flags().StringArrayVar(
		&o.associatedIdentities,
		"associated-identity",
		[]string{},
		fmt.Sprintf("identities on code review platforms in the form 'providerID::identity' (e.g., '%s::<username>+<user ID>')", tuf.GitHubAppRoleName),
	)

	cmd.Flags().StringArrayVar(
		&o.customMetadata,
		"custom",
		[]string{},
		"additional custom metadata in the form KEY=VALUE",
	)
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	publicKeys := map[string]*tufv02.Key{}
	for _, key := range o.publicKeys {
		key, err := gittuf.LoadPublicKey(key)
		if err != nil {
			return err
		}

		publicKeys[key.ID()] = key.(*tufv02.Key)
	}

	associatedIdentities := map[string]string{}
	for _, associatedIdentity := range o.associatedIdentities {
		split := strings.Split(associatedIdentity, "::")
		if len(split) != 2 {
			return fmt.Errorf("invalid format for associated identity '%s'", associatedIdentity)
		}
		associatedIdentities[split[0]] = split[1]
	}

	custom := map[string]string{}
	for _, customEntry := range o.customMetadata {
		split := strings.Split(customEntry, "=")
		if len(split) != 2 {
			return fmt.Errorf("invalid format for custom metadata '%s'", customEntry)
		}
		custom[split[0]] = split[1]
	}

	person := &tufv02.Person{
		PersonID:             o.personID,
		PublicKeys:           publicKeys,
		AssociatedIdentities: associatedIdentities,
		Custom:               custom,
	}

	opts := []trustpolicyopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}
	return repo.AddPrincipalToTargets(cmd.Context(), signer, o.policyName, []tuf.Principal{person}, true, opts...)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:   "add-person",
		Short: "Add a trusted person to a policy file",
		Long: `The 'add-person' command adds a trusted person to a gittuf policy file, enabling them to participate
in signing operations governed by the trust policy.

A person consists of:
- A unique identifier (--person-ID)
- One or more authorized public keys (--public-key)
- Optional associated identities on external platforms (e.g., GitHub, GitLab) via --associated-identity
- Optional custom metadata (--custom) for tracking additional attributes

Supported public key formats:
- Local PEM-encoded public key files
- GPG keys using the "gpg:<fingerprint>" syntax
- Sigstore identities using "fulcio:<identity>::<issuer>"

By default, the trusted person is added to the main policy file (targets), unless overridden with --policy-name.
The command also supports adding an RSL (Record of State Log) entry if --rsl is specified.

Requirements:
- A valid signing key must be provided using --signing-key
- The person ID and at least one public key are required

Usage:
  gittuf policy add-person --person-ID <id> --public-key <path|gpg:fingerprint|fulcio:identity::issuer> [flags]`,
		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
