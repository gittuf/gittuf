// SPDX-License-Identifier: Apache-2.0

package listrules

import (
	"fmt"

	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/repository"
	"github.com/spf13/cobra"
)

type options struct{}

func (o *options) AddFlags(_ *cobra.Command) {}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	policyRules, policyRoot, err := repo.ListRules(cmd.Context(), policy.PolicyRef)
	if err != nil {
		return err
	}

	policyStagingRules, policyStagingRoot, err := repo.ListRules(cmd.Context(), policy.PolicyStagingRef)
	if err != nil {
		return err
	}

	fmt.Print(policy.GetDiffBetweenPolicyAndStaging(policyRules, policyStagingRules, policyRoot, policyStagingRoot))

	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "list-rules",
		Short:             "List rules for the current state",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
