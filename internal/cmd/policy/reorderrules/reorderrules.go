// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package reorderrules

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
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
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	o.ruleNames = append(o.ruleNames, args...)

	repo, err := gittuf.LoadRepository()
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	err = repo.ReorderDelegations(cmd.Context(), signer, o.policyName, o.ruleNames, true)
	return err
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "reorder-rules",
		Short:             "Reorder rules in the specified policy file",
		Long:              "This command allows users to reorder rules in the specified policy file. By default, the main policy file is selected. The rule names need to be passed as arguments, in the new order they must appear in, starting from the first to the last rule. Rule names may contain spaces, so they should be enclosed in quotes if necessary.",
		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}

	o.AddFlags(cmd)

	return cmd
}
