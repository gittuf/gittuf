// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addrootperson

import (
	"fmt"
	"strings"

	"github.com/gittuf/gittuf/experimental/gittuf"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
	"github.com/spf13/cobra"
)

type options struct {
	p                    *persistent.Options
	personID             string
	publicKeys           []string
	associatedIdentities []string
	customMetadata       []string
}

func (o *options) AddFlags(cmd *cobra.Command) {
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
	cmd.MarkFlagRequired("public-key") //nolint:errcheck

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
	return repo.AddRootPerson(cmd.Context(), signer, person, true, opts...)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "add-root-person",
		Short:             "Add Root person to gittuf root of trust",
		Long:              `The 'add-root-person' command allows users to add a new root person to the repository's root of trust. In gittuf, a person definition consists of a unique identifier ('--person-ID'), one or more authorized public keys ('--public-key'), optional associated identities ('--associated-identity') on external platforms (e.g., GitHub, GitLab), and optional custom metadata ('--custom') for tracking additional attributes. Note that the keys can be specified from disk, from the GPG keyring using the "gpg:<fingerprint>" format, or as a Sigstore identity as "fulcio:<identity>::<issuer>". Optionally, the change can be recorded in the RSL.`,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
