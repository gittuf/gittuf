// SPDX-License-Identifier: Apache-2.0

package reorderrules

import (
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/gittuf/gittuf/internal/repository"
	"github.com/spf13/cobra"
)

type options struct {
	p          *persistent.Options
	policyName string
	ruleNames  []string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.policyName,
		"policy-name",
		"targets",
		"name of policy file to reorder rules in",
	)

	cmd.Flags().StringSliceVar(
		&o.ruleNames,
		"rule-order",
		[]string{},
		"a space-separated list of rule names, in the new order that they should appear in, from first to last",
	)
	cmd.MarkFlagRequired("rule-order") //nolint:errcheck
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	o.ruleNames = append(o.ruleNames, args...)

	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	signer, err := common.LoadSigner(o.p.SigningKey)
	if err != nil {
		return err
	}

	err = repo.ReorderDelegations(cmd.Context(), signer, o.policyName, o.ruleNames, true)
	if err != nil {
		return err
	}

	return nil
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "reorder-rules",
		Short:             "Reorder rules in the specified policy file",
		Long:              `This command allows users to reorder rules in the specified policy file. Specify the names of rules in the new order they should appear in, starting from first to last rule. By default, the main policy file is selected. Note that authorized keys can be specified from disk, from the GPG keyring using the "gpg:<fingerprint>" format, or as a Sigstore identity as "fulcio:<identity>::<issuer>".`,
		PreRunE:           common.CheckIfSigningViableWithFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}

	o.AddFlags(cmd)

	return cmd
}
