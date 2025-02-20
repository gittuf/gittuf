// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package attest

import (
	"github.com/gittuf/gittuf/internal/cmd/attest/authorize"
	"github.com/gittuf/gittuf/internal/cmd/attest/github"
	"github.com/gittuf/gittuf/internal/cmd/attest/persistent"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	o := &persistent.Options{}
	cmd := &cobra.Command{
		Use:               "attest",
		Short:             "Tools for attesting to code contributions",
		DisableAutoGenTag: true,
	}
	o.AddPersistentFlags(cmd)

	cmd.AddCommand(authorize.New(o))
	cmd.AddCommand(github.New(o))

	return cmd
}
