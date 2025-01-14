// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package remove

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/spf13/cobra"
)

type options struct {
	p          *persistent.Options
	policyName string
	hookName   string
	stageName  string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.policyName,
		"policy-name",
		policy.TargetsRoleName,
		"name of policy file to remove hook from",
	)

	cmd.Flags().StringVar(
		&o.stageName,
		"stage",
		"",
		"stage of hook",
	)
	cmd.MarkFlagRequired("stage") //nolint:errcheck

	cmd.Flags().StringVar(
		&o.hookName,
		"hook-name",
		"",
		"name of hook",
	)
	cmd.MarkFlagRequired("rule-name") //nolint:errcheck
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

	return repo.RemoveHook(cmd.Context(), signer, o.policyName, o.hookName, o.stageName, true)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "remove",
		Short:             "remove a hook specified in the policy",
		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
