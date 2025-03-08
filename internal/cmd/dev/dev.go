// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package dev

import (
	"fmt"

	"github.com/gittuf/gittuf/internal/cmd/dev/addgithubapproval"
	"github.com/gittuf/gittuf/internal/cmd/dev/attestgithub"
	"github.com/gittuf/gittuf/internal/cmd/dev/dismissgithubapproval"
	"github.com/gittuf/gittuf/internal/cmd/dev/populatecache"
	"github.com/gittuf/gittuf/internal/cmd/dev/rslrecordat"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "dev",
		Short:   "Developer mode commands",
		Long:    fmt.Sprintf("These commands are meant to be used to aid gittuf development, and are not expected to be used during standard workflows. If used, they can undermine repository security. To proceed, set %s=1.", dev.DevModeKey),
		PreRunE: checkInDevMode,
	}

	cmd.AddCommand(attestgithub.New())
	cmd.AddCommand(addgithubapproval.New())
	cmd.AddCommand(dismissgithubapproval.New())
	cmd.AddCommand(populatecache.New())
	cmd.AddCommand(rslrecordat.New())

	return cmd
}

func checkInDevMode(_ *cobra.Command, _ []string) error {
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}
	return nil
}
