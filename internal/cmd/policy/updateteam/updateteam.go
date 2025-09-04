package updateteam

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
		"name of policy file to update team in",
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
		k, err := gittuf.LoadPublicKey(key)
		if err != nil {
			return err
		}
		publicKeys[k.ID()] = k.(*tufv02.Key)
	}

	associatedIdentities := map[string]string{}
	for _, identity := range o.associatedIdentities {
		split := strings.Split(identity, "::")
		if len(split) != 2 {
			return fmt.Errorf("invalid format for associated identity '%s'", identity)
		}
		associatedIdentities[split[0]] = split[1]
	}

	custom := map[string]string{}
	for _, entry := range o.customMetadata {
		split := strings.Split(entry, "=")
		if len(split) != 2 {
			return fmt.Errorf("invalid format for custom metadata '%s'", entry)
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

	return repo.UpdatePrincipalInTargets(cmd.Context(), signer, o.policyName, []tuf.Principal{person}, opts...)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "update-team",
		Short:             "Update an existing trusted team (person) in a policy file",
		Long:              `The 'update-team' command updates an existing trusted team/person in a gittuf policy file. A person is defined by a unique ID ('--person-ID'), authorized public keys ('--public-key'), optional associated identities ('--associated-identity') from external systems, and custom metadata ('--custom'). This command replaces the full person record (keys, identities, metadata) in the policy.`,
		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}

