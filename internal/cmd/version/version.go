// SPDX-License-Identifier: Apache-2.0

package version

import (
	"fmt"

	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/version"
	"github.com/spf13/cobra"
)

type options struct{}

func (o *options) AddFlags(cmd *cobra.Command) {}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	v := version.GetVersion()
	if v[0] == 'v' {
		v = v[1:]
	}
	fmt.Printf("gittuf version %s\n", v)

	if common.EvalMode() {
		fmt.Printf("gittuf is operating in eval mode. Override by setting %s=0.\n", common.EvalModeKey)
	}

	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Version of gittuf",
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
