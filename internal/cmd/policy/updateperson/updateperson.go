// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package updateperson

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
		"name of policy file to update person in",
	)

	cmd.Flags().StringVar(
		&o.personID,
		"person-ID",
		"",
		"person ID to update",
	)
	cmd.MarkFlagRequired("person-ID") //nolint:errcheck

	cmd.Flags().StringArrayVar(
		&o.publicKeys,
		"public-key",
		[]string{},
		"public keys for person",
	)

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
		"custom metadata in the form KEY=VALUE",
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

	// Process public keys
	publicKeys := make(map[string]*tufv02.Key)
	for _, key := range o.publicKeys {
		publicKey, err := gittuf.LoadPublicKey(key)
		if err != nil {
			return err
		}
		tufKey := publicKey.(*tufv02.Key)
		publicKeys[tufKey.ID()] = tufKey
	}

	// Process associated identities
	associatedIdentities := make(map[string]string)
	for _, associatedIdentity := range o.associatedIdentities {
		split := strings.Split(associatedIdentity, "::")
		if len(split) != 2 {
			return fmt.Errorf("invalid format for associated identity '%s'", associatedIdentity)
		}
		associatedIdentities[split[0]] = split[1]
	}

	// Process custom metadata
	custom := make(map[string]string)
	for _, customEntry := range o.customMetadata {
		split := strings.Split(customEntry, "=")
		if len(split) != 2 {
			return fmt.Errorf("invalid format for custom metadata '%s'", customEntry)
		}
		custom[split[0]] = split[1]
	}

	// Create a new person with updated fields
	newPerson := &tufv02.Person{
		PersonID:             o.personID,
		PublicKeys:           publicKeys,
		AssociatedIdentities: associatedIdentities,
		Custom:               custom,
	}

	// Update the person in the policy
	opts := []trustpolicyopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}

	return repo.UpdatePrincipalInTargets(cmd.Context(), signer, o.policyName, newPerson, true, opts...)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "update-person",
		Short:             "Update a person in a policy file",
		Long:              `This command allows users to update a person's information in the specified policy file. By default, the main policy file is selected. The command replaces the person's existing information with the new values provided. If a field is not specified, its existing value is not preserved.`,
		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
