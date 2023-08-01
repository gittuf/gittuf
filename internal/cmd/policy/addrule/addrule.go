package addrule

import (
	"context"
	"os"

	"github.com/adityasaky/gittuf/internal/cmd/common"
	"github.com/adityasaky/gittuf/internal/cmd/policy/persistent"
	"github.com/adityasaky/gittuf/internal/policy"
	"github.com/adityasaky/gittuf/internal/repository"
	"github.com/spf13/cobra"
)

type options struct {
	p              *persistent.Options
	policyName     string
	ruleName       string
	authorizedKeys []string
	rulePatterns   []string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.policyName,
		"policy-name",
		policy.TargetsRoleName,
		"policy file to add rule to",
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
		&o.rulePatterns,
		"rule-pattern",
		[]string{},
		"patterns used to identify namespaces rule applies to",
	)
	cmd.MarkFlagRequired("rule-pattern") //nolint:errcheck
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	keyBytes, err := os.ReadFile(o.p.SigningKey)
	if err != nil {
		return err
	}

	authorizedKeysBytes := [][]byte{}
	for _, key := range o.authorizedKeys {
		kb, err := common.ReadKeyBytes(key)
		if err != nil {
			return err
		}

		authorizedKeysBytes = append(authorizedKeysBytes, kb)
	}

	return repo.AddDelegation(context.Background(), keyBytes, o.policyName, o.ruleName, authorizedKeysBytes, o.rulePatterns, true)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:   "add-rule",
		Short: "Add a new rule to a policy file",
		Long:  `This command allows users to add a new rule to the specified policy file. By default, the main policy file is selected. Note that authorized keys can be specified from disk using the custom securesystemslib format, from the GPG keyring using the "gpg:<fingerprint>" format, or as a Sigstore identity as "fulcio:<identity>::<issuer>".`,
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
