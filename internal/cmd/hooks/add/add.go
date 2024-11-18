// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package add

import (
	"errors"
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/experimental/gittuf/hooks"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/spf13/cobra"
	"strings"
)

// todo: this function must take 2 arguments: path/to/hooks/file and stage
// the file specified by the file must be copied properly and securely into directory
// the stage specified by the data in the -s flag must be used for generating the metadata
// the metadata can be as simple as
// stage: path/to/hook
// the metadata file will have to be updated everytime gittuf hooks add is called
// the directory structure will be:
// hooks/
//		hooksMetadata.json
//		hook1.hook
//		hook2.hook

// QUESTIONS: where will we be copying the scripts to?

type options struct {
	p        *persistent.Options
	filepath string
	stage    string
	hookname string
	env      string
	modules  []string
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
		&o.hookname,
		"hookname",
		"n",
		"",
		"Name of the hook",
	)

	cmd.Flags().StringVarP(
		&o.env,
		"env",
		"e",
		"",
		"Environment which the hook must run in",
	)
	cmd.MarkFlagRequired("env") //nolint:errcheck

	cmd.Flags().StringSliceVarP(
		&o.modules,
		"modules",
		"m",
		nil,
		"Modules which the Lua hook must run. Usage: -m module1,module2,modulen",
	)

}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	if (strings.ToLower(o.env) == "lua") && (o.modules == nil) {
		return errors.New("must specify modules using --modules flag for Lua sandbox environment")
	}

	repo, err := gittuf.LoadRepository()
	if err != nil {
		return err
	}

	// a hooks.HookIdentifiers object is being used to pass in the value for repo.AddHooks because it's too many
	// arguments to be passed into the function otherwise.
	identifiers := hooks.HookIdentifiers{
		Filepath:    o.filepath,
		Stage:       o.stage,
		Hookname:    o.hookname,
		Environment: o.env,
		Modules:     o.modules,
	}

	return repo.AddHooks(cmd.Context(), identifiers)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "add",
		Short:             "add a script to be run as a hook, specify when and where to run it.",
		Long:              "add a script to be run as a hook, specify when and which environment to run it in. Environment can be among lua, gvisor, docker and local.", // placeholder. modify once environments are finalized.
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
