// SPDX-License-Identifier: Apache-2.0

package root

import (
	"log/slog"
	"os"

	"github.com/gittuf/gittuf/internal/cmd/addhooks"
	"github.com/gittuf/gittuf/internal/cmd/clone"
	"github.com/gittuf/gittuf/internal/cmd/dev"
	"github.com/gittuf/gittuf/internal/cmd/policy"
	"github.com/gittuf/gittuf/internal/cmd/rsl"
	"github.com/gittuf/gittuf/internal/cmd/trust"
	"github.com/gittuf/gittuf/internal/cmd/verifycommit"
	"github.com/gittuf/gittuf/internal/cmd/verifyref"
	"github.com/gittuf/gittuf/internal/cmd/verifytag"
	"github.com/gittuf/gittuf/internal/cmd/version"
	"github.com/spf13/cobra"
)

type options struct {
	verbose bool
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().BoolVar(&o.verbose, "verbose", false, "enable verbose logging")
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "gittuf",
		Short: "A security layer for Git repositories, powered by TUF",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			level := slog.LevelInfo

			if o.verbose {
				level = slog.LevelDebug
			}

			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
		},
		SilenceUsage:      true,
		DisableAutoGenTag: true,
	}

	o.AddFlags(cmd)

	cmd.AddCommand(addhooks.New())
	cmd.AddCommand(clone.New())
	cmd.AddCommand(dev.New())
	cmd.AddCommand(trust.New())
	cmd.AddCommand(policy.New())
	cmd.AddCommand(rsl.New())
	cmd.AddCommand(verifycommit.New())
	cmd.AddCommand(verifyref.New())
	cmd.AddCommand(verifytag.New())
	cmd.AddCommand(version.New())

	return cmd
}
