// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addhook

import (
	"errors"
	"os"
	"strings"

	"github.com/gittuf/gittuf/experimental/gittuf"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/spf13/cobra"
)

var ErrLuaNoModules = errors.New("must specify modules using --modules flag for Lua sandbox environment")

type options struct {
	p            *persistent.Options
	filepath     string
	stage        string
	hookName     string
	env          string
	modules      []string
	principalIDs []string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(
		&o.filepath,
		"file",
		"f",
		"",
		"filepath of the script to be run as a hook",
	)
	cmd.MarkFlagRequired("file") //nolint:errcheck

	cmd.Flags().StringVarP(
		&o.stage,
		"stage",
		"s",
		"",
		"stage at which the hook must be run",
	)
	cmd.MarkFlagRequired("stage") //nolint:errcheck

	cmd.Flags().StringVarP(
		&o.hookName,
		"hookname",
		"n",
		"",
		"Name of the hook",
	)

	cmd.Flags().StringVarP(
		&o.env,
		"env",
		"e",
		"lua",
		"environment which the hook must run in",
	)

	cmd.Flags().StringArrayVar(
		&o.modules,
		"modules",
		[]string{},
		"modules which the Lua hook must run",
	)
	cmd.Flags().StringArrayVar(
		&o.principalIDs,
		"principalIDs",
		[]string{},
		"principal IDs which must run this hook",
	)
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	var environment tuf.HookEnvironment
	switch strings.ToLower(o.env) {
	case tuf.HookEnvironmentLuaString:
		environment = tuf.HookEnvironmentLua
	default:
		return tuf.ErrInvalidHookEnvironment
	}

	if (strings.ToLower(o.env) == "lua") && (o.modules == nil) {
		return ErrLuaNoModules
	}

	var stage tuf.HookStage
	switch strings.ToLower(o.stage) {
	case "pre-commit":
		stage = tuf.HookStagePreCommit
	case "pre-push":
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

	hookBytes, err := os.ReadFile(o.filepath)
	if err != nil {
		return err
	}

	opts := []trustpolicyopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}

	return repo.AddHook(cmd.Context(), signer, stage, o.hookName, hookBytes, environment, o.modules, o.principalIDs, true, opts...)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "add-hook",
		Short:             "Add a script to be run as a gittuf hook, specify when and where to run it.",
		Long:              "Add a script to be run as a gittuf hook, specify when and which environment to run it in. The only currently supported environment is 'lua'.",
		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
