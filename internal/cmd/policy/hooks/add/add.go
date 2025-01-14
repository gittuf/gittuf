// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package add

import (
	"errors"
	"strings"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/spf13/cobra"
)

var ErrLuaNoModules = errors.New("must specify modules using --modules flag for Lua sandbox environment")

type options struct {
	p            *persistent.Options
	policyName   string
	filepath     string
	stage        string
	hookName     string
	env          string
	modules      []string
	principalIDs []string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.policyName,
		"policy-name",
		policy.TargetsRoleName,
		"name of policy file to add hook to",
	)

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
	if (strings.ToLower(o.env) == "lua") && (o.modules == nil) {
		return ErrLuaNoModules
	}

	repo, err := gittuf.LoadRepository()
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	return repo.AddHook(cmd.Context(), signer, o.policyName, o.hookName, o.filepath, o.stage, o.env, o.modules, o.principalIDs, true)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "add",
		Short:             "add a script to be run as a hook, specify when and where to run it.",
		Long:              "add a script to be run as a hook, specify when and which environment to run it in. Environment can be among lua, gvisor, docker and local.", // placeholder. modify once environments are finalized.
		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
