package addgitbranchrule

import (
	"context"
	"fmt"
	"os"

	"github.com/adityasaky/gittuf/internal/cmd/common"
	"github.com/adityasaky/gittuf/internal/cmd/policy/persistent"
	"github.com/adityasaky/gittuf/internal/policy"
	"github.com/adityasaky/gittuf/internal/repository"
	"github.com/adityasaky/gittuf/internal/tuf"
	"github.com/spf13/cobra"
)

type options struct {
	p              *persistent.Options
	policyName     string
	ruleName       string
	authorizedKeys []string
	gitRefPatterns []string
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
		&o.gitRefPatterns,
		"git-pattern",
		[]string{},
		"patterns used to identify Git branches the rule applies to",
	)
	cmd.MarkFlagRequired("git-pattern") //nolint:errcheck
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

	rulePatterns := make([]string, 0, len(o.gitRefPatterns))
	for _, p := range o.gitRefPatterns {
		ap := common.AbsoluteBranch(p)
		rulePatterns = append(rulePatterns, fmt.Sprintf("%s%s", tuf.GitPatternPrefix, ap))
	}

	return repo.AddDelegation(context.Background(), keyBytes, o.policyName, o.ruleName, authorizedKeysBytes, rulePatterns, true)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:   "add-git-branch-rule",
		Short: "Add a new rule for Git branches to a policy file",
		Long:  `This command allows users to add a new rule protecting Git branches to the specified policy file. By default, the main policy file is selected. Note that authorized keys can be specified from disk using the custom securesystemslib format or from the GPG keyring using the "gpg:<fingerprint>" format.`,
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
