// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addrule

import (
	"fmt"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/spf13/cobra"
)

type options struct {
	p                      *persistent.Options
	policyName             string
	ruleName               string
	authorizedKeys         []string
	authorizedPrincipalIDs []string
	rulePatterns           []string
	threshold              int
	access                 string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.policyName,
		"policy-name",
		policy.TargetsRoleName,
		"name of policy file to add rule to",
	)

	cmd.Flags().StringVar(
		&o.ruleName,
		"rule-name",
		"",
		"name of rule",
	)
	cmd.MarkFlagRequired("rule-name") //nolint:errcheck

	cmd.Flags().StringArrayVar(
		&o.authorizedKeys,
		"authorize-key",
		[]string{},
		"authorized public key for rule",
	)
	cmd.Flags().MarkDeprecated("authorize-key", "use --authorize instead") //nolint:errcheck

	cmd.Flags().StringArrayVar(
		&o.authorizedPrincipalIDs,
		"authorize",
		[]string{},
		"authorize the principal IDs for the rule",
	)
	cmd.MarkFlagsOneRequired("authorize", "authorize-key")

	cmd.Flags().StringArrayVar(
		&o.rulePatterns,
		"rule-pattern",
		[]string{},
		"patterns used to identify namespaces rule applies to",
	)
	cmd.MarkFlagRequired("rule-pattern") //nolint:errcheck

	cmd.Flags().IntVar(
		&o.threshold,
		"threshold",
		1,
		"threshold of required valid signatures",
	)

	cmd.Flags().StringVar(
		&o.access,
		"access",
		"write",
		"specify access level: read or write",
	)
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := gittuf.LoadRepository()
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	if o.access != "read" && o.access != "write" {
		return fmt.Errorf("invalid access level: must be 'read' or 'write'")
	}

	authorizedPrincipalIDs := []string{}
	for _, key := range o.authorizedKeys {
		key, err := gittuf.LoadPublicKey(key)
		if err != nil {
			return err
		}

		authorizedPrincipalIDs = append(authorizedPrincipalIDs, key.ID())
	}
	authorizedPrincipalIDs = append(authorizedPrincipalIDs, o.authorizedPrincipalIDs...)

	return repo.AddDelegation(cmd.Context(), signer, o.policyName, o.ruleName, o.access, authorizedPrincipalIDs, o.rulePatterns, o.threshold, true)
	//NEED TO SOMEWAY FIGURE OUT WHERE THIS IS AND ADD ACCESS TO IT
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "add-rule",
		Short:             "Add a new rule to a policy file",
		Long:              `This command allows users to add a new rule to the specified policy file. By default, the main policy file is selected. Note that authorized keys can be specified from disk, from the GPG keyring using the "gpg:<fingerprint>" format, or as a Sigstore identity as "fulcio:<identity>::<issuer>".`,
		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
