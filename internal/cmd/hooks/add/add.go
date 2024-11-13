package add

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
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
		"Modules which the Lua hook must run",
	)
	if strings.ToLower(o.env) == "lua" {
		cmd.MarkFlagRequired("modules") //nolint:errcheck
	}

}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := gittuf.LoadRepository()
	if err != nil {
		return err
	}

	return repo.AddHooks(o.filepath, o.stage, o.hookname, o.env, o.modules)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "add",
		Short:             "add a script to be run as a hook and mention when to run it.",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
