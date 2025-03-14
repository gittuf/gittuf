// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addhook

import (
	"errors"
	"fmt"
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
	filePath     string
	hookName     string
	env          string
	modules      []string
	principalIDs []string

	isPreCommit bool
	isPrePush   bool
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(
		&o.filePath,
		"file-path",
		"f",
		"",
		"path of the script to be run as a hook",
	)
	cmd.MarkFlagRequired("file-path") //nolint:errcheck

	cmd.Flags().BoolVarP(
		&o.isPreCommit,
		"is-pre-commit",
		"",
		false,
		"add the hook to the pre-commit stage",
	)
	cmd.Flags().BoolVarP(
		&o.isPrePush,
		"is-pre-push",
		"",
		false,
		"add the hook to the pre-push stage",
	)
	cmd.MarkFlagsOneRequired("is-pre-commit", "is-pre-push")

	cmd.Flags().StringVarP(
		&o.hookName,
		"hook-name",
		"n",
		"",
		"Name of the hook",
	)
	cmd.MarkFlagRequired("hook-name") //nolint:errcheck

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
		nil,
		"modules which the Lua hook must run",
	)

	cmd.Flags().StringArrayVar(
		&o.principalIDs,
		"principal-ID",
		nil,
		"principal IDs which must run this hook",
	)
	cmd.MarkFlagRequired("principal-ID") //nolint:errcheck
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

	stages := []tuf.HookStage{}
	if o.isPreCommit {
		stages = append(stages, tuf.HookStagePreCommit)
	}
	if o.isPrePush {
		stages = append(stages, tuf.HookStagePrePush)
	}

	repo, err := gittuf.LoadRepository()
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	hookBytes, err := os.ReadFile(o.filePath)
	if err != nil {
		return err
	}

	opts := []trustpolicyopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}

	return repo.AddHook(cmd.Context(), signer, stages, o.hookName, hookBytes, environment, o.modules, o.principalIDs, true, opts...)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "add-hook",
		Short:             fmt.Sprintf("Add a script to be run as a gittuf hook, specify when and where to run it (developer mode only, set %s=1)", dev.DevModeKey),
		Long:              fmt.Sprintf("Add a script to be run as a gittuf hook, specify when and which environment to run it in. The only currently supported environment is 'lua' (developer mode only, set %s=1)", dev.DevModeKey),
		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
