// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addrule

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
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

	authorizedPrincipalIDs := []string{}
	for _, key := range o.authorizedKeys {
		key, err := gittuf.LoadPublicKey(key)
		if err != nil {
			return err
		}

		authorizedPrincipalIDs = append(authorizedPrincipalIDs, key.ID())
	}
	authorizedPrincipalIDs = append(authorizedPrincipalIDs, o.authorizedPrincipalIDs...)

	opts := []trustpolicyopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}
	return repo.AddDelegation(cmd.Context(), signer, o.policyName, o.ruleName, authorizedPrincipalIDs, o.rulePatterns, o.threshold, true, opts...)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:   "add-rule",
		Short: "Add a new rule to a policy file",
		Long: `The 'add-rule' command adds a new delegation rule to a gittuf policy file, enabling fine-grained
authorization based on path patterns and principals.

Each rule defines:
- A name (--rule-name)
- One or more principals (--authorize or --authorize-key) who are allowed to sign within the scope of the rule
- A set of rule patterns (--rule-pattern) defining the namespaces or paths the rule governs
- A signature threshold (--threshold), which is the minimum number of valid signatures required to satisfy the rule

Principal identifiers can include:
- GPG fingerprints ("gpg:<fingerprint>")
- Fulcio Sigstore identities ("fulcio:<identity>::<issuer>")
- Person IDs (if already added to the policy)
- Public key files (deprecated flag: --authorize-key)

By default, the rule is added to the main policy file unless --policy-name is specified. If the --rsl flag is passed,
a Record of State Log (RSL) entry will be added to log this policy change.

Requirements:
- A valid signing key must be provided using --signing-key
- At least one of --authorize or --authorize-key must be specified
- --rule-name and --rule-pattern are required

Usage:
  gittuf policy add-rule --rule-name <name> --authorize <principalID> --rule-pattern <pattern> [flags]`,
		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
