// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package removehook

import (
	"fmt"

	"github.com/gittuf/gittuf/experimental/gittuf"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/spf13/cobra"
)

type options struct {
	p        *persistent.Options
	hookName string

	isPreCommit bool
	isPrePush   bool
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(
		&o.isPreCommit,
		"is-pre-commit",
		"",
		false,
		"remove the hook from the pre-commit stage",
	)
	cmd.Flags().BoolVarP(
		&o.isPrePush,
		"is-pre-push",
		"",
		false,
		"remove the hook from the pre-push stage",
	)
	cmd.MarkFlagsOneRequired("is-pre-commit", "is-pre-push")

	cmd.Flags().StringVar(
		&o.hookName,
		"hook-name",
		"",
		"name of hook",
	)
	cmd.MarkFlagRequired("hook-name") //nolint:errcheck
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	stages := []tuf.HookStage{}
	if o.isPreCommit {
		stages = append(stages, tuf.HookStagePreCommit)
	}
	if o.isPrePush {
		stages = append(stages, tuf.HookStagePrePush)
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

	return repo.RemoveHook(cmd.Context(), signer, stages, o.hookName, true, opts...)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "remove-hook",
		Short:             fmt.Sprintf("Remove a gittuf hook specified in the policy (developer mode only, set %s=1)", dev.DevModeKey),
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
