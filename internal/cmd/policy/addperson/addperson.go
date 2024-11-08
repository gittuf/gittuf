// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addperson

import (
	"fmt"
	"strings"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/gittuf/gittuf/internal/dev"
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
		"identities on code review platforms in the form 'providerID::identity' (e.g., 'https://github.com::<username>')",
	)

	cmd.Flags().StringArrayVar(
		&o.customMetadata,
		"custom",
		[]string{},
		"additional custom metadata in the form KEY=VALUE",
	)
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	if !tufv02.AllowV02Metadata() {
		return fmt.Errorf("developer mode and v0.2 policy metadata must be enabled, set %s=1 and %s=1", dev.DevModeKey, tufv02.AllowV02MetadataKey)
	}

	repo, err := gittuf.LoadRepository()
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

	return repo.AddPrincipalToTargets(cmd.Context(), signer, o.policyName, []tuf.Principal{person}, true)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "add-person",
		Short:             fmt.Sprintf("Add a trusted person to a policy file (requires developer mode and v0.2 policy metadata to be enabled, set %s=1 and %s=1)", dev.DevModeKey, tufv02.AllowV02MetadataKey),
		Long:              `This command allows users to add a trusted person to the specified policy file. By default, the main policy file is selected. Note that the person's keys can be specified from disk, from the GPG keyring using the "gpg:<fingerprint>" format, or as a Sigstore identity as "fulcio:<identity>::<issuer>".`,
		PreRunE:           common.CheckIfSigningViableWithFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
