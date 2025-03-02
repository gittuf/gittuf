// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package remove

import (
	"strings"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/spf13/cobra"
)

type options struct {
	p        *persistent.Options
	hookName string
	stage    string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.stage,
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
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	var stage tuf.HookStage
	switch strings.ToLower(o.stage) {
	case tuf.HookStagePreCommitString:
		stage = tuf.HookStagePreCommit
	case tuf.HookStagePrePushString:
		stage = tuf.HookStagePrePush
	default:
		return tuf.ErrInvalidHookStage
	}

	repo, err := gittuf.LoadRepository()
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	return repo.RemoveHook(cmd.Context(), signer, stage, o.hookName, true)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "remove",
		Short:             "Remove a gittuf hook specified in the policy",
		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
