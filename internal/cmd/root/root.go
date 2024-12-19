// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package root

import (
	"log/slog"
	"os"

	"github.com/gittuf/gittuf/internal/cmd/addhooks"
	"github.com/gittuf/gittuf/internal/cmd/clone"
	"github.com/gittuf/gittuf/internal/cmd/dev"
	"github.com/gittuf/gittuf/internal/cmd/policy"
	"github.com/gittuf/gittuf/internal/cmd/policy/hooks"
	"github.com/gittuf/gittuf/internal/cmd/profile"
	"github.com/gittuf/gittuf/internal/cmd/rsl"
	"github.com/gittuf/gittuf/internal/cmd/trust"
	"github.com/gittuf/gittuf/internal/cmd/verifyref"
	"github.com/gittuf/gittuf/internal/cmd/version"
	"github.com/spf13/cobra"
)

type options struct {
	verbose           bool
	profile           bool
	cpuProfileFile    string
	memoryProfileFile string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().BoolVar(
		&o.verbose,
		"verbose",
		false,
		"enable verbose logging",
	)

	cmd.PersistentFlags().BoolVar(
		&o.profile,
		"profile",
		false,
		"enable CPU and memory profiling",
	)

	cmd.PersistentFlags().StringVar(
		&o.cpuProfileFile,
		"profile-CPU-file",
		"cpu.prof",
		"file to store CPU profile",
	)

	cmd.PersistentFlags().StringVar(
		&o.memoryProfileFile,
		"profile-memory-file",
		"memory.prof",
		"file to store memory profile",
	)
}

func (o *options) PreRunE(_ *cobra.Command, _ []string) error {
	// Setup logging
	level := slog.LevelInfo

	if o.verbose {
		level = slog.LevelDebug
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})))

	// Start profiling if flag is set
	if o.profile {
		return profile.StartProfiling(o.cpuProfileFile, o.memoryProfileFile)
	}

	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "gittuf",
		Short:             "A security layer for Git repositories, powered by TUF",
		SilenceUsage:      true,
		DisableAutoGenTag: true,
		PersistentPreRunE: o.PreRunE,
	}

	o.AddFlags(cmd)

	cmd.AddCommand(addhooks.New())
	cmd.AddCommand(clone.New())
	cmd.AddCommand(dev.New())
	cmd.AddCommand(trust.New())
	cmd.AddCommand(policy.New())
	cmd.AddCommand(hooks.New())
	cmd.AddCommand(rsl.New())
	cmd.AddCommand(verifyref.New())
	cmd.AddCommand(version.New())

	return cmd
}
