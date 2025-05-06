// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addglobalrule

import (
	"fmt"

	"github.com/gittuf/gittuf/experimental/gittuf"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/spf13/cobra"
)

type options struct {
	p        *persistent.Options
	ruleName string
	ruleType string

	rulePatterns []string

	threshold int
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.ruleName,
		"rule-name",
		"",
		"name of rule",
	)
	cmd.MarkFlagRequired("rule-name") //nolint:errcheck

	cmd.Flags().StringVar(
		&o.ruleType,
		"type",
		"",
		fmt.Sprintf("type of rule (%s|%s)", tuf.GlobalRuleThresholdType, tuf.GlobalRuleBlockForcePushesType),
	)
	cmd.MarkFlagRequired("type") //nolint:errcheck

	cmd.Flags().StringArrayVar(
		&o.rulePatterns,
		"rule-pattern",
		[]string{},
		"patterns used to identify namespaces rule applies to",
	)

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

	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	opts := []trustpolicyopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}

	switch o.ruleType {
	case tuf.GlobalRuleThresholdType:
		if len(o.rulePatterns) == 0 {
			return fmt.Errorf("required flag --rule-pattern not set for global rule type '%s'", tuf.GlobalRuleThresholdType)
		}

		return repo.AddGlobalRuleThreshold(cmd.Context(), signer, o.ruleName, o.rulePatterns, o.threshold, true, opts...)

	case tuf.GlobalRuleBlockForcePushesType:
		if len(o.rulePatterns) == 0 {
			return fmt.Errorf("required flag --rule-pattern not set for global rule type '%s'", tuf.GlobalRuleBlockForcePushesType)
		}

		return repo.AddGlobalRuleBlockForcePushes(cmd.Context(), signer, o.ruleName, o.rulePatterns, true, opts...)

	default:
		return tuf.ErrUnknownGlobalRuleType
	}
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "add-global-rule",
		Short:             fmt.Sprintf("Add a new global rule to root of trust (developer mode only, set %s=1)", dev.DevModeKey),
		Long:              "This command allows a user to add a new global rule to the root of trust. The user must specify the name, type, and rule pattern for the rule.",
		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
