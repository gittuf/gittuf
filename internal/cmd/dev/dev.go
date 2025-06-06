// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package dev

import (
	"fmt"

	"github.com/gittuf/gittuf/internal/cmd/dev/addgithubapproval"
	"github.com/gittuf/gittuf/internal/cmd/dev/attestgithub"
	"github.com/gittuf/gittuf/internal/cmd/dev/dismissgithubapproval"
	"github.com/gittuf/gittuf/internal/cmd/dev/rslrecordat"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "dev",
		Short:   "Developer mode commands",
		Long:    fmt.Sprintf(`The 'dev' command group provides advanced utilities for use during gittuf development and debugging. These commands are intended for internal or development use and are not designed to be run in production or standard repository workflows. Improper use may compromise repository security guarantees. To enable these commands, the environment variable %s must be set to 1.`, dev.DevModeKey),
		PreRunE: checkInDevMode,
	}

	cmd.AddCommand(attestgithub.New())
	cmd.AddCommand(addgithubapproval.New())
	cmd.AddCommand(dismissgithubapproval.New())
	cmd.AddCommand(rslrecordat.New())

	return cmd
}

func checkInDevMode(_ *cobra.Command, _ []string) error {
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}
	return nil
}
