// SPDX-License-Identifier: Apache-2.0

package addbranchrule

import (
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/repository"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/spf13/cobra"
)

type options struct {
	p              *persistent.Options
	policyName     string
	ruleName       string
	authorizedKeys []string
	branches       []string
	threshold      int
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
	cmd.MarkFlagRequired("authorize-key") //nolint:errcheck

	cmd.Flags().StringArrayVar(
		&o.branches,
		"branch",
		[]string{},
		"branches rule applies to",
	)
	cmd.MarkFlagRequired("branch") //nolint:errcheck

	cmd.Flags().IntVar(
		&o.threshold,
		"threshold",
		1,
		"threshold of required valid signatures",
	)
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	signer, err := common.LoadSigner(o.p.SigningKey)
	if err != nil {
		return err
	}

	authorizedKeys := []*tuf.Key{}
	for _, key := range o.authorizedKeys {
		key, err := common.LoadPublicKey(key)
		if err != nil {
			return err
		}

		authorizedKeys = append(authorizedKeys, key)
	}

	rulePatterns := []string{}

	for _, branch := range o.branches {
		rulePatterns = append(rulePatterns, "git:refs/heads/"+branch)
	}

	return repo.AddDelegation(cmd.Context(), signer, o.policyName, o.ruleName, authorizedKeys, rulePatterns, o.threshold, true)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "add-branch-rule",
		Short:             "Add a new branch protection rule to a policy file",
		Long:              `This command allows users to add a new branch protection rule to the specified policy file. By default, the main policy file is selected. Note that authorized keys can be specified from disk, from the GPG keyring using the "gpg:<fingerprint>" format, or as a Sigstore identity as "fulcio:<identity>::<issuer>".`,
		PreRunE:           common.CheckIfSigningViableWithFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}

	o.AddFlags(cmd)

	return cmd
}
