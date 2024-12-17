// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addglobalrule

import (
	"fmt"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/spf13/cobra"
)

type options struct {
	p            *persistent.Options
	ruleName     string
	rulePatterns []string
	threshold    int
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.ruleName,
		"rule-name",
		"",
		"name of rule",
	)
	cmd.MarkFlagRequired("rule-name") //nolint:errcheck

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
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	repo, err := gittuf.LoadRepository()
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	return repo.AddGlobalRule(cmd.Context(), signer, o.ruleName, o.rulePatterns, o.threshold, true)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "add-global-rule",
		Short:             fmt.Sprintf("Add a new global rule to root of trust (developer mode only, set %s=1)", dev.DevModeKey),
		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
