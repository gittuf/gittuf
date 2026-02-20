// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package version //nolint:revive

import (
	"fmt"

	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/version"
	"github.com/spf13/cobra"
)

type options struct{}

func (o *options) AddFlags(_ *cobra.Command) {}

func (o *options) Run(_ *cobra.Command, _ []string) error {
	v := version.GetVersion()
	if v[0] == 'v' {
		v = v[1:]
	}
	fmt.Printf("gittuf version %s\n", v)

	if dev.InDevMode() {
		fmt.Printf("gittuf is operating in developer mode. Override by setting %s=0.\n", dev.DevModeKey)
	}

	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "version",
		Short:             "Version of gittuf",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
